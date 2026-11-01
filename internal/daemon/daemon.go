package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/OrenVill/Argus/internal/config"
	"github.com/OrenVill/Argus/internal/control"
	"github.com/OrenVill/Argus/internal/ingest"
	"github.com/OrenVill/Argus/internal/pipeline"
	iredis "github.com/OrenVill/Argus/internal/redis"
	"github.com/OrenVill/Argus/internal/webhook"
)

// Run starts ingest, workers, control API, and webhook pool until ctx is done.
func Run(ctx context.Context, cfg *config.Config) error {
	if err := cfg.ValidateForRun(); err != nil {
		return err
	}

	rdb, err := iredis.NewClient(cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	store := iredis.NewStore(rdb.RDB, cfg.Redis.KeyPrefix, cfg.Redis.TTLDays)
	whTimeout, err := time.ParseDuration(cfg.Webhook.Timeout)
	if err != nil {
		return err
	}
	dispatcher := webhook.NewDispatcher(webhook.Config{
		URL:     cfg.Webhook.URL,
		Timeout: whTimeout,
		Workers: 2,
		Queue:   256,
	})
	defer dispatcher.Close()

	proc, err := pipeline.New(cfg, store, dispatcher)
	if err != nil {
		return err
	}

	defaultGrace, err := time.ParseDuration(cfg.Detection.GraceWindowDuration)
	if err != nil {
		return err
	}
	ctrl := control.NewServer(cfg.Control, cfg.ToMeta(), store, defaultGrace)

	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ctrl.Start(runCtx); err != nil {
			slog.Error("control server", "err", err)
			cancel(err)
		}
	}()

	lines := make(chan string, cfg.Daemon.BufferCapacity)
	ingestMetrics := &ingest.Metrics{
		OnReceived: func() { LinesReceived.Add(1) },
		OnDrop: func() {
			LinesDropped.Add(1)
			LogDropRateLimited()
		},
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(lines)
		ingest.FanIn(runCtx, ingest.Options{
			Stdin: cfg.Inputs.Stdin,
			Files: cfg.Inputs.Files,
		}, lines, ingestMetrics)
	}()

	for i := 0; i < cfg.Daemon.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workerLoop(runCtx, lines, proc)
		}()
	}

	<-runCtx.Done()
	wg.Wait()
	if err := context.Cause(runCtx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func workerLoop(ctx context.Context, lines <-chan string, proc *pipeline.Processor) {
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-lines:
			if !ok {
				return
			}
			if err := proc.ProcessLine(ctx, line); err != nil {
				slog.Warn("process line", "err", err)
				continue
			}
			LinesProcessed.Add(1)
		}
	}
}

// SetGraceCLI sets deploy grace from the CLI (§6).
func SetGraceCLI(ctx context.Context, cfg *config.Config, service string, duration time.Duration) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	rdb, err := iredis.NewClient(cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	store := iredis.NewStore(rdb.RDB, cfg.Redis.KeyPrefix, cfg.Redis.TTLDays)
	m := cfg.ToMeta()
	if service == "" {
		service = m.Service
	}
	deadline := time.Now().UTC().Add(duration)
	return store.SetGraceDeadline(ctx, m, service, deadline, duration+time.Hour)
}

// SetTemplateStatusCLI applies §5.6 from CLI.
func SetTemplateStatusCLI(ctx context.Context, cfg *config.Config, hash, status string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	rdb, err := iredis.NewClient(cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	store := iredis.NewStore(rdb.RDB, cfg.Redis.KeyPrefix, cfg.Redis.TTLDays)
	return store.SetTemplateStatus(ctx, cfg.ToMeta(), hash, status)
}
