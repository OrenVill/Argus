package ingest

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
)

const stdinBufSize = 256 * 1024

// runStdin reads line-delimited input from os.Stdin until ctx is cancelled.
func runStdin(ctx context.Context, out chan<- string, send sendLine) {
	reader := bufio.NewReader(os.Stdin)
	for {
		if ctx.Err() != nil {
			return
		}
		lineCh := make(chan string, 1)
		errCh := make(chan error, 1)
		go func() {
			line, err := readStdinLine(reader)
			if err != nil {
				errCh <- err
				return
			}
			lineCh <- line
		}()
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err == io.EOF {
				return
			}
			if err != nil {
				slog.Warn("stdin read", "err", err)
				return
			}
		case line := <-lineCh:
			send(ctx, out, line)
		}
	}
}

func readStdinLine(r *bufio.Reader) (string, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		line = append(line, part...)
		if err == nil {
			break
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err == io.EOF {
			if len(line) == 0 {
				return "", io.EOF
			}
			break
		}
		return "", err
	}
	line = bytes.TrimSuffix(line, []byte("\n"))
	line = bytes.TrimSuffix(line, []byte("\r"))
	if len(line) > stdinBufSize {
		slog.Warn("stdin line exceeds limit; truncating", "limit", stdinBufSize, "bytes", len(line))
		line = line[:stdinBufSize]
	}
	return string(line), nil
}
