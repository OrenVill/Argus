package pipeline

import (
	"context"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	"github.com/OrenVill/Argus/internal/config"
	iredis "github.com/OrenVill/Argus/internal/redis"
)

func BenchmarkProcessLineMaskDrainHash(b *testing.B) {
	cfg := &config.Config{
		Meta: config.Meta{Env: "bench", Service: "svc"},
		Redis: config.Redis{
			Address:   "127.0.0.1:6379",
			KeyPrefix: "argus:bench:",
			TTLDays:   1,
		},
		Normalization: config.Normalization{
			DrainDepth:     4,
			SimThreshold:   0.5,
			SampleLinesMax: 5,
		},
		Detection: config.Detection{
			RegressionThreshold: "24h",
			SurgeMultipliers:    []int{10, 100, 1000},
			GraceWindowDuration: "30m",
		},
		LogLevel: config.LogLevel{
			Regex: `(?i)\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|PANIC)\b`,
		},
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		b.Skip("redis not available")
	}
	store := iredis.NewStore(rdb, cfg.Redis.KeyPrefix, cfg.Redis.TTLDays)
	proc, err := New(cfg, store, nil)
	if err != nil {
		b.Fatal(err)
	}
	line := `2026-11-01T12:00:00Z ERROR payment failed user=abc123 amount=42.5`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = proc.ProcessLine(ctx, line)
	}
}
