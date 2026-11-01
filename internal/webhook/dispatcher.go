package webhook

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/OrenVill/Argus/internal/metrics"
)

// Job is one async webhook delivery.
type Job struct {
	Payload        Payload
	TemplateHash   string
	IdempotencyKey string
}

// Config holds delivery settings.
type Config struct {
	URL     string
	Timeout time.Duration
	Workers int
	Queue   int
}

// Dispatcher posts anomaly webhooks without blocking ingest.
type Dispatcher struct {
	cfg    Config
	client *http.Client
	jobs   chan Job
	wg     sync.WaitGroup
}

// NewDispatcher starts worker goroutines.
func NewDispatcher(cfg Config) *Dispatcher {
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	if cfg.Queue <= 0 {
		cfg.Queue = 256
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	d := &Dispatcher{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		jobs: make(chan Job, cfg.Queue),
	}
	for i := 0; i < cfg.Workers; i++ {
		d.wg.Add(1)
		go d.worker()
	}
	return d
}

// Enqueue schedules delivery; drops when queue is full.
func (d *Dispatcher) Enqueue(j Job) {
	select {
	case d.jobs <- j:
		metrics.WebhooksQueued.Add(1)
	default:
		metrics.WebhooksFailed.Add(1)
		slog.Warn("webhook queue full; dropping delivery", "key", j.IdempotencyKey)
	}
}

// Close drains workers after context cancellation.
func (d *Dispatcher) Close() {
	close(d.jobs)
	d.wg.Wait()
}

func (d *Dispatcher) worker() {
	defer d.wg.Done()
	for j := range d.jobs {
		d.deliver(j)
	}
}

var retryDelays = []time.Duration{time.Second, 4 * time.Second, 16 * time.Second}

func (d *Dispatcher) deliver(j Job) {
	body, err := j.Payload.MarshalJSON()
	if err != nil {
		metrics.WebhooksFailed.Add(1)
		slog.Error("webhook marshal", "err", err)
		return
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelays[attempt-1])
		}
		if err := d.post(body, j.IdempotencyKey); err != nil {
			lastErr = err
			slog.Warn("webhook attempt failed", "attempt", attempt+1, "err", err)
			continue
		}
		metrics.WebhooksSent.Add(1)
		return
	}
	metrics.WebhooksFailed.Add(1)
	slog.Error("webhook delivery exhausted retries", "key", j.IdempotencyKey, "err", lastErr)
}

func (d *Dispatcher) post(body []byte, idempotencyKey string) error {
	req, err := http.NewRequest(http.MethodPost, d.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Argus-Idempotency-Key", idempotencyKey)
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
