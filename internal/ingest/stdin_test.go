package ingest

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestReadStdinLineTruncatesOversized(t *testing.T) {
	long := strings.Repeat("a", stdinBufSize+100)
	r := bufio.NewReader(bytes.NewReader([]byte(long + "\n")))
	got, err := readStdinLine(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != stdinBufSize {
		t.Fatalf("len=%d want %d", len(got), stdinBufSize)
	}
}

func TestReadStdinLineContinuesAfterLongLine(t *testing.T) {
	long := strings.Repeat("b", stdinBufSize+50)
	r := bufio.NewReader(bytes.NewReader([]byte(long + "\nsecond\n")))
	if _, err := readStdinLine(r); err != nil {
		t.Fatal(err)
	}
	got, err := readStdinLine(r)
	if err != nil {
		t.Fatal(err)
	}
	if got != "second" {
		t.Fatalf("got %q", got)
	}
}

func TestRunStdinRespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runStdin(ctx, out, func(ctx context.Context, out chan<- string, line string) {
			select {
			case out <- line:
			case <-ctx.Done():
			}
		})
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runStdin blocked after context cancel")
	}
}
