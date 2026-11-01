package metrics

import (
	"expvar"
	"log/slog"
	"sync"
	"time"
)

var (
	LinesReceived  = expvar.NewInt("argus_lines_received")
	LinesDropped   = expvar.NewInt("argus_lines_dropped")
	LinesProcessed = expvar.NewInt("argus_lines_processed")
	WebhooksQueued = expvar.NewInt("argus_webhooks_queued")
	WebhooksSent   = expvar.NewInt("argus_webhooks_sent")
	WebhooksFailed = expvar.NewInt("argus_webhooks_failed")
)

var dropLogMu sync.Mutex
var lastDropLog time.Time

// LogDropRateLimited emits at most one warning per second when ingest drops lines.
func LogDropRateLimited() {
	dropLogMu.Lock()
	defer dropLogMu.Unlock()
	now := time.Now()
	if now.Sub(lastDropLog) < time.Second {
		return
	}
	lastDropLog = now
	slog.Warn("ingest buffer full; dropping log lines")
}
