//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/OrenVill/Argus/internal/meta"
	iredis "github.com/OrenVill/Argus/internal/redis"
	"github.com/OrenVill/Argus/test/redisutil"
)

const (
	testPrefix = "argus:"
	testAddr   = "127.0.0.1:6379"
)

func testMeta(suffix string) meta.Meta {
	return meta.Meta{Env: "test", Service: "svc-" + suffix}
}

func newTestStore(t *testing.T, rdb *goredis.Client) *iredis.Store {
	t.Helper()
	return iredis.NewStore(rdb, testPrefix, 30)
}

func connectRedis(t *testing.T) *goredis.Client {
	t.Helper()
	rdb := goredis.NewClient(&goredis.Options{Addr: testAddr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	if err := redisutil.DeleteKeysWithPrefix(ctx, rdb, testPrefix); err != nil {
		t.Fatalf("delete test keys: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func baseInput(now time.Time) iredis.ProcessInput {
	return iredis.ProcessInput{
		Template:            "error connecting to <IP>",
		Level:               "ERROR",
		NowRFC3339:          now.UTC().Format(time.RFC3339),
		MaskedSample:        "error connecting to <IP>",
		RegressionThreshold: 24 * time.Hour,
		SurgeMultipliers:    []int{10, 100, 1000},
		TTLSeconds:          86400,
		SampleMax:           5,
	}
}

func TestProcessLineNewNoGrace(t *testing.T) {
	rdb := connectRedis(t)
	store := newTestStore(t, rdb)
	m := testMeta("new-no-grace")
	ctx := context.Background()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	in := baseInput(now)
	in.TemplateHash = "abc123"
	res, err := store.ProcessLine(ctx, m, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.FireWebhook || res.AnomalyType != "NEW_SIGNATURE" {
		t.Fatalf("expected NEW_SIGNATURE webhook, got %+v", res)
	}
	if res.Count != 1 {
		t.Fatalf("count=%d", res.Count)
	}
}

func TestProcessLineGraceSuppressesInfo(t *testing.T) {
	rdb := connectRedis(t)
	store := newTestStore(t, rdb)
	m := testMeta("grace-info")
	ctx := context.Background()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(30 * time.Minute)
	if err := store.SetGraceDeadline(ctx, m, m.Service, deadline, 31*time.Hour); err != nil {
		t.Fatal(err)
	}

	in := baseInput(now)
	in.TemplateHash = "graceinfo1"
	in.Level = "INFO"
	in.GraceDeadlineRFC3339 = deadline.UTC().Format(time.RFC3339)

	res, err := store.ProcessLine(ctx, m, in)
	if err != nil {
		t.Fatal(err)
	}
	if res.FireWebhook {
		t.Fatalf("expected no webhook during grace for INFO, got %+v", res)
	}
}

func TestProcessLineGraceFiresWarnNew(t *testing.T) {
	rdb := connectRedis(t)
	store := newTestStore(t, rdb)
	m := testMeta("grace-warn")
	ctx := context.Background()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(30 * time.Minute)
	if err := store.SetGraceDeadline(ctx, m, m.Service, deadline, 31*time.Hour); err != nil {
		t.Fatal(err)
	}

	in := baseInput(now)
	in.TemplateHash = "gracewarn1"
	in.Level = "WARN"
	in.GraceDeadlineRFC3339 = deadline.UTC().Format(time.RFC3339)

	res, err := store.ProcessLine(ctx, m, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.FireWebhook || res.AnomalyType != "NEW_SIGNATURE" {
		t.Fatalf("expected NEW_SIGNATURE for WARN during grace, got %+v", res)
	}
}

func TestProcessLineIgnoredIncrementsOnly(t *testing.T) {
	rdb := connectRedis(t)
	store := newTestStore(t, rdb)
	m := testMeta("ignored")
	ctx := context.Background()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	hash := "ignored1"
	in := baseInput(now)
	in.TemplateHash = hash
	if _, err := store.ProcessLine(ctx, m, in); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTemplateStatus(ctx, m, hash, "ignored"); err != nil {
		t.Fatal(err)
	}

	in.NowRFC3339 = now.Add(time.Minute).UTC().Format(time.RFC3339)
	res, err := store.ProcessLine(ctx, m, in)
	if err != nil {
		t.Fatal(err)
	}
	if res.FireWebhook {
		t.Fatalf("ignored template should not webhook: %+v", res)
	}
	if res.Count != 2 {
		t.Fatalf("count=%d want 2", res.Count)
	}
}

func TestProcessLineSurgeTiersLatch(t *testing.T) {
	rdb := connectRedis(t)
	store := newTestStore(t, rdb)
	m := testMeta("surge")
	ctx := context.Background()
	base := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	hash := "surge1"

	in := baseInput(base)
	in.TemplateHash = hash
	in.RegressionThreshold = 365 * 24 * time.Hour // disable regression
	res, err := store.ProcessLine(ctx, m, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.FireWebhook || res.AnomalyType != "NEW_SIGNATURE" {
		t.Fatalf("first line: %+v", res)
	}

	// reported_count=1 after NEW; tier 10 at count>=10
	for i := 2; i < 10; i++ {
		in.NowRFC3339 = base.Add(time.Duration(i) * time.Minute).UTC().Format(time.RFC3339)
		res, err = store.ProcessLine(ctx, m, in)
		if err != nil {
			t.Fatal(err)
		}
		if res.FireWebhook {
			t.Fatalf("line %d unexpected webhook: %+v", i, res)
		}
	}
	in.NowRFC3339 = base.Add(10 * time.Minute).UTC().Format(time.RFC3339)
	res, err = store.ProcessLine(ctx, m, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.FireWebhook || res.AnomalyType != "FREQUENCY_SPIKE" || res.SurgeMultiplier != 10 {
		t.Fatalf("expected tier 10 spike at count 10, got %+v", res)
	}
	if res.ReportedCount != 1 {
		t.Fatalf("reported_count baseline=%d want 1", res.ReportedCount)
	}

	// Same tier should not re-fire until next tier (100 at count>=100)
	in.NowRFC3339 = base.Add(11 * time.Minute).UTC().Format(time.RFC3339)
	res, err = store.ProcessLine(ctx, m, in)
	if err != nil {
		t.Fatal(err)
	}
	if res.FireWebhook {
		t.Fatalf("should not re-fire tier 10: %+v", res)
	}
}

func TestProcessLineRegressionResetsSurgeTier(t *testing.T) {
	rdb := connectRedis(t)
	store := newTestStore(t, rdb)
	m := testMeta("regression")
	ctx := context.Background()
	hash := "reg1"
	key := iredis.TemplateKey(testPrefix, m.Env, m.Service, hash)

	first := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	in := baseInput(first)
	in.TemplateHash = hash
	in.RegressionThreshold = time.Hour
	if _, err := store.ProcessLine(ctx, m, in); err != nil {
		t.Fatal(err)
	}

	// Seed high last_surge_tier and counts without firing webhook path
	if err := rdb.HSet(ctx, key,
		"count", "50",
		"reported_count", "50",
		"last_surge_tier", "10",
		"last_seen", first.UTC().Format(time.RFC3339),
	).Err(); err != nil {
		t.Fatal(err)
	}

	second := first.Add(2 * time.Hour)
	in.NowRFC3339 = second.UTC().Format(time.RFC3339)
	res, err := store.ProcessLine(ctx, m, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.FireWebhook || res.AnomalyType != "REGRESSION" {
		t.Fatalf("expected REGRESSION, got %+v", res)
	}
	tier, err := rdb.HGet(ctx, key, "last_surge_tier").Int()
	if err != nil {
		t.Fatal(err)
	}
	if tier != 0 {
		t.Fatalf("last_surge_tier=%d want 0 after regression", tier)
	}
}

func TestSetTemplateStatusNewDeletesHash(t *testing.T) {
	rdb := connectRedis(t)
	store := newTestStore(t, rdb)
	m := testMeta("status-new")
	ctx := context.Background()
	hash := "delme"
	key := iredis.TemplateKey(testPrefix, m.Env, m.Service, hash)

	in := baseInput(time.Now().UTC())
	in.TemplateHash = hash
	if _, err := store.ProcessLine(ctx, m, in); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTemplateStatus(ctx, m, hash, "new"); err != nil {
		t.Fatal(err)
	}
	n, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("hash should be deleted")
	}

	res, err := store.ProcessLine(ctx, m, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res.FireWebhook || res.AnomalyType != "NEW_SIGNATURE" {
		t.Fatalf("after reset expect NEW_SIGNATURE, got %+v", res)
	}
}

func TestGraceKeyLayout(t *testing.T) {
	rdb := connectRedis(t)
	store := newTestStore(t, rdb)
	m := testMeta("grace-key")
	ctx := context.Background()
	deadline := time.Now().UTC().Truncate(time.Second).Add(15 * time.Minute)
	if err := store.SetGraceDeadline(ctx, m, m.Service, deadline, 2*time.Hour); err != nil {
		t.Fatal(err)
	}
	gkey := iredis.GraceKey(testPrefix, m.Env, m.Service)
	val, err := rdb.Get(ctx, gkey).Result()
	if err != nil {
		t.Fatal(err)
	}
	if val != deadline.Format(time.RFC3339) {
		t.Fatalf("grace value=%q want %q", val, deadline.Format(time.RFC3339))
	}
	got, ok, err := store.GetGraceDeadline(ctx, m, m.Service)
	if err != nil || !ok {
		t.Fatalf("GetGraceDeadline: ok=%v err=%v", ok, err)
	}
	if !got.Equal(deadline) {
		t.Fatalf("deadline mismatch: %v vs %v", got, deadline)
	}
}

func TestTemplateKeyLayout(t *testing.T) {
	got := iredis.TemplateKey("argus:", "prod", "pay", "deadbeef")
	want := fmt.Sprintf("argus:prod:pay:deadbeef")
	if got != want {
		t.Fatalf("template key=%q want %q", got, want)
	}
	g := iredis.GraceKey("argus:", "prod", "pay")
	if g != "argus:prod:grace:pay" {
		t.Fatalf("grace key=%q", g)
	}
}
