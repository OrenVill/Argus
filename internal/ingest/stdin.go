package ingest

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
)

const stdinBufSize = 256 * 1024

// runStdin reads line-delimited input from os.Stdin until ctx is cancelled.
func runStdin(ctx context.Context, out chan<- string, send sendLine) {
	sc := bufio.NewScanner(os.Stdin)
	buf := make([]byte, 0, stdinBufSize)
	sc.Buffer(buf, stdinBufSize)
	for {
		if ctx.Err() != nil {
			return
		}
		if !sc.Scan() {
			if err := sc.Err(); err != nil && err != io.EOF {
				slog.Warn("stdin read", "err", err)
			}
			return
		}
		send(ctx, out, sc.Text())
	}
}
