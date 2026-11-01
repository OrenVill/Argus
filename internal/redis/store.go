package redis

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/OrenVill/Argus/internal/meta"
)

//go:embed scripts/process_line.lua
var processLineLua string

// Store performs atomic template state updates in Redis.
type Store struct {
	rdb       *goredis.Client
	keyPrefix string
	ttlDays   int
	script    *goredis.Script
}

// NewStore builds a Store from an existing Redis client and layout config.
func NewStore(rdb *goredis.Client, keyPrefix string, ttlDays int) *Store {
	return &Store{
		rdb:       rdb,
		keyPrefix: keyPrefix,
		ttlDays:   ttlDays,
		script:    goredis.NewScript(processLineLua),
	}
}

// ProcessInput is the per-line detection payload passed to Lua.
type ProcessInput struct {
	TemplateHash, Template, Level, NowRFC3339, MaskedSample string
	GraceDeadlineRFC3339                                    string
	RegressionThreshold                                     time.Duration
	SurgeMultipliers                                        []int
	TTLSeconds, SampleMax                                   int
}

// LineResult is the decoded Lua script outcome for webhook dispatch.
type LineResult struct {
	FireWebhook     bool
	AnomalyType     string
	Count           int
	ReportedCount   int
	SurgeMultiplier int
	SampleLines     []string
	FirstSeen       string
	LastSeen        string
}

type luaResult struct {
	FireWebhook     bool     `json:"fire_webhook"`
	AnomalyType     string   `json:"anomaly_type"`
	Count           int      `json:"count"`
	ReportedCount   int      `json:"reported_count"`
	SurgeMultiplier int      `json:"surge_multiplier"`
	SampleLines     []string `json:"sample_lines"`
	FirstSeen       string   `json:"first_seen"`
	LastSeen        string   `json:"last_seen"`
	Error           string   `json:"error"`
}

// ProcessLine runs the §5 state machine atomically for one template hash.
func (s *Store) ProcessLine(ctx context.Context, m meta.Meta, in ProcessInput) (LineResult, error) {
	key := TemplateKeyMeta(s.keyPrefix, m, in.TemplateHash)
	ttl := in.TTLSeconds
	if ttl <= 0 {
		ttl = s.ttlDays * 86400
	}
	sampleMax := in.SampleMax
	if sampleMax <= 0 {
		sampleMax = 5
	}
	surgeJSON, err := json.Marshal(in.SurgeMultipliers)
	if err != nil {
		return LineResult{}, fmt.Errorf("marshal surge multipliers: %w", err)
	}
	regressionSec := int(in.RegressionThreshold.Seconds())
	if regressionSec <= 0 && in.RegressionThreshold > 0 {
		regressionSec = 1
	}

	raw, err := s.script.Run(ctx, s.rdb, []string{key},
		in.Template,
		in.Level,
		in.NowRFC3339,
		in.MaskedSample,
		in.GraceDeadlineRFC3339,
		regressionSec,
		string(surgeJSON),
		ttl,
		sampleMax,
	).Text()
	if err != nil {
		return LineResult{}, fmt.Errorf("process_line lua: %w", err)
	}

	var decoded luaResult
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return LineResult{}, fmt.Errorf("decode lua result: %w", err)
	}
	if decoded.Error != "" {
		return LineResult{}, fmt.Errorf("lua: %s", decoded.Error)
	}
	return LineResult{
		FireWebhook:     decoded.FireWebhook,
		AnomalyType:     decoded.AnomalyType,
		Count:           decoded.Count,
		ReportedCount:   decoded.ReportedCount,
		SurgeMultiplier: decoded.SurgeMultiplier,
		SampleLines:     decoded.SampleLines,
		FirstSeen:       decoded.FirstSeen,
		LastSeen:        decoded.LastSeen,
	}, nil
}

// GetGraceDeadline reads the active grace deadline for a service, if any.
func (s *Store) GetGraceDeadline(ctx context.Context, m meta.Meta, service string) (time.Time, bool, error) {
	gkey := GraceKeyMeta(s.keyPrefix, m, service)
	val, err := s.rdb.Get(ctx, gkey).Result()
	if err == goredis.Nil {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse grace deadline: %w", err)
	}
	return t, true, nil
}

// SetGraceDeadline stores the deploy grace deadline (§6) with TTL = duration + 1h buffer.
func (s *Store) SetGraceDeadline(ctx context.Context, m meta.Meta, service string, deadline time.Time, ttl time.Duration) error {
	gkey := GraceKeyMeta(s.keyPrefix, m, service)
	deadline = deadline.UTC()
	if err := s.rdb.Set(ctx, gkey, deadline.Format(time.RFC3339), ttl).Err(); err != nil {
		return fmt.Errorf("set grace deadline: %w", err)
	}
	return nil
}

// SetTemplateStatus applies §5.6 admin status (known|ignored|new).
func (s *Store) SetTemplateStatus(ctx context.Context, m meta.Meta, hash, status string) error {
	key := TemplateKeyMeta(s.keyPrefix, m, hash)
	switch status {
	case "new":
		if err := s.rdb.Del(ctx, key).Err(); err != nil {
			return fmt.Errorf("delete template: %w", err)
		}
		return nil
	case "known", "ignored":
		exists, err := s.rdb.Exists(ctx, key).Result()
		if err != nil {
			return err
		}
		if exists == 0 {
			return fmt.Errorf("template hash %q not found", hash)
		}
		if err := s.rdb.HSet(ctx, key, "status", status).Err(); err != nil {
			return fmt.Errorf("set template status: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("invalid template status %q", status)
	}
}
