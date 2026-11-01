//go:build integration

package golden_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/OrenVill/Argus/internal/config"
	"github.com/OrenVill/Argus/internal/pipeline"
	iredis "github.com/OrenVill/Argus/internal/redis"
	"github.com/OrenVill/Argus/internal/webhook"
)

func TestRuntimeSmokePipelineWebhookRedis(t *testing.T) {
	ctx := context.Background()
	rdb := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:6379"})
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis: %v", err)
	}
	prefix := "argus:smoke-runtime:"
	_ = rdb.FlushDB(ctx)

	var mu sync.Mutex
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var obj map[string]any
		_ = json.Unmarshal(b, &obj)
		mu.Lock()
		bodies = append(bodies, obj)
		mu.Unlock()
		if r.Header.Get("X-Argus-Idempotency-Key") == "" {
			t.Error("missing idempotency header")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Meta: config.Meta{Env: "smoke", Service: "runtime"},
		Redis: config.Redis{
			Address:   "127.0.0.1:6379",
			KeyPrefix: prefix,
			TTLDays:   1,
		},
		Normalization: config.Normalization{
			DrainDepth:     4,
			SimThreshold: 0.5,
			SampleLinesMax: 3,
		},
		Detection: config.Detection{
			GraceWindowDuration: "30m",
			RegressionThreshold: "24h",
			SurgeMultipliers:    []int{10, 100, 1000},
		},
		LogLevel: config.LogLevel{
			Regex: `(?i)\b(DEBUG|INFO|WARN|ERROR|FATAL)\b`,
		},
		Webhook: config.Webhook{URL: srv.URL, Timeout: "2s"},
	}
	store := iredis.NewStore(rdb, prefix, 1)
	dispatcher := webhook.NewDispatcher(webhook.Config{URL: srv.URL, Timeout: 2 * time.Second, Workers: 1, Queue: 8})
	defer dispatcher.Close()

	proc, err := pipeline.New(cfg, store, dispatcher)
	if err != nil {
		t.Fatal(err)
	}
	line := "ERROR runtime smoke failed host=10.0.0.1"
	if err := proc.ProcessLine(ctx, line); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	n := len(bodies)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 webhook, got %d", n)
	}
	keys, _ := rdb.Keys(ctx, prefix+"*").Result()
	if len(keys) < 1 {
		t.Fatalf("expected redis template key, keys=%v", keys)
	}
}
