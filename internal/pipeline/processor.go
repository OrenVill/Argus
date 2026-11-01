package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/OrenVill/Argus/internal/config"
	"github.com/OrenVill/Argus/internal/drain"
	"github.com/OrenVill/Argus/internal/hash"
	"github.com/OrenVill/Argus/internal/level"
	"github.com/OrenVill/Argus/internal/mask"
	"github.com/OrenVill/Argus/internal/meta"
	iredis "github.com/OrenVill/Argus/internal/redis"
	"github.com/OrenVill/Argus/internal/webhook"
)

// Processor runs mask → drain → hash → level → Redis for one log line.
type Processor struct {
	meta       meta.Meta
	masker     *mask.Masker
	forest     *drain.ProcessForest
	level      *level.Parser
	store      *iredis.Store
	webhooks   *webhook.Dispatcher
	regression time.Duration
	surge      []int
	sampleMax  int
	ttlSeconds int
}

// New builds a hot-path processor from loaded config and dependencies.
func New(cfg *config.Config, store *iredis.Store, hooks *webhook.Dispatcher) (*Processor, error) {
	masker, err := mask.New(cfg.Normalization)
	if err != nil {
		return nil, fmt.Errorf("masker: %w", err)
	}
	levelParser, err := level.NewParser(cfg.LogLevel.JSONField, cfg.LogLevel.Regex)
	if err != nil {
		return nil, fmt.Errorf("level parser: %w", err)
	}
	regression, err := time.ParseDuration(cfg.Detection.RegressionThreshold)
	if err != nil {
		return nil, err
	}
	m := cfg.ToMeta()
	forest := drain.NewProcessForest(m, cfg.Normalization.DrainDepth, cfg.Normalization.SimThreshold)
	return &Processor{
		meta:       m,
		masker:     masker,
		forest:     forest,
		level:      levelParser,
		store:      store,
		webhooks:   hooks,
		regression: regression,
		surge:      cfg.Detection.SurgeMultipliers,
		sampleMax:  cfg.Normalization.SampleLinesMax,
		ttlSeconds: cfg.Redis.TTLDays * 86400,
	}, nil
}

// ProcessLine evaluates one raw log line.
func (p *Processor) ProcessLine(ctx context.Context, raw string) error {
	masked := p.masker.Apply(raw)
	cluster := p.forest.Match(masked)
	templateHash := hash.TemplateHash(p.meta, cluster.Template)
	lv := p.level.Parse(raw)

	graceDeadline := ""
	if deadline, ok, err := p.store.GetGraceDeadline(ctx, p.meta, p.meta.Service); err != nil {
		return fmt.Errorf("grace deadline: %w", err)
	} else if ok && time.Now().UTC().Before(deadline.UTC()) {
		graceDeadline = deadline.UTC().Format(time.RFC3339)
	}

	now := time.Now().UTC()
	in := iredis.ProcessInput{
		TemplateHash:         templateHash,
		Template:             cluster.Template,
		Level:                lv.String(),
		NowRFC3339:           now.Format(time.RFC3339),
		MaskedSample:         masked,
		GraceDeadlineRFC3339: graceDeadline,
		RegressionThreshold:  p.regression,
		SurgeMultipliers:     p.surge,
		TTLSeconds:           p.ttlSeconds,
		SampleMax:            p.sampleMax,
	}

	res, err := p.store.ProcessLine(ctx, p.meta, in)
	if err != nil {
		return err
	}
	if res.FireWebhook && p.webhooks != nil {
		payload := webhook.NewPayload(p.meta, templateHash, cluster.Template, lv.String(), res)
		key := webhook.IdempotencyKey(p.meta, templateHash, res.AnomalyType, res.Count)
		p.webhooks.Enqueue(webhook.Job{
			Payload:        payload,
			TemplateHash:   templateHash,
			IdempotencyKey: key,
		})
	}
	return nil
}
