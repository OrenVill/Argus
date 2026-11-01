package daemon

import "github.com/OrenVill/Argus/internal/metrics"

// Re-export hot-path counters for daemon wiring.
var (
	LinesReceived      = metrics.LinesReceived
	LinesDropped       = metrics.LinesDropped
	LinesProcessed     = metrics.LinesProcessed
	LogDropRateLimited = metrics.LogDropRateLimited
)
