package ingest

import (
	"context"
	"sync"
)

// Metrics hooks for backpressure (wired by daemon).
type Metrics struct {
	OnReceived func()
	OnDrop     func()
}

// Options describes log sources to fan in.
type Options struct {
	Stdin bool
	Files []string
}

type sendLine func(ctx context.Context, out chan<- string, line string)

// FanIn starts ingest goroutines and sends lines to out until ctx is cancelled.
func FanIn(ctx context.Context, opts Options, out chan<- string, m *Metrics) {
	send := makeSend(m)
	var wg sync.WaitGroup
	if opts.Stdin {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runStdin(ctx, out, send)
		}()
	}
	if len(opts.Files) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runFileTails(ctx, opts.Files, out, send)
		}()
	}
	wg.Wait()
}

func makeSend(m *Metrics) sendLine {
	return func(ctx context.Context, out chan<- string, line string) {
		if m != nil && m.OnReceived != nil {
			m.OnReceived()
		}
		select {
		case out <- line:
		case <-ctx.Done():
		default:
			if m != nil && m.OnDrop != nil {
				m.OnDrop()
			}
		}
	}
}
