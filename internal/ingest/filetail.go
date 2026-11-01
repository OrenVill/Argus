package ingest

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nxadm/tail"
)

type fileTailer struct {
	patterns     []string
	send         sendLine
	active       sync.Map // path -> struct{}
	startupPaths sync.Map // paths present at initial glob (tail from EOF)
}

func runFileTails(ctx context.Context, patterns []string, out chan<- string, send sendLine) {
	ft := &fileTailer{patterns: patterns, send: send}
	if paths, err := expandPatterns(patterns); err != nil {
		slog.Warn("expand file patterns at startup", "err", err)
	} else {
		for _, path := range paths {
			ft.startupPaths.Store(path, struct{}{})
		}
	}
	ft.refresh(ctx, out)
	go ft.watchDirs(ctx, out)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ft.refresh(ctx, out)
		}
	}
}

func (ft *fileTailer) refresh(ctx context.Context, out chan<- string) {
	paths, err := expandPatterns(ft.patterns)
	if err != nil {
		slog.Warn("expand file patterns", "err", err)
		return
	}
	for _, path := range paths {
		if _, loaded := ft.active.LoadOrStore(path, struct{}{}); loaded {
			continue
		}
		_, seekEOF := ft.startupPaths.Load(path)
		go ft.tailOne(ctx, path, out, seekEOF)
	}
}

func (ft *fileTailer) tailOne(ctx context.Context, path string, out chan<- string, seekEOF bool) {
	defer ft.active.Delete(path)

	for ctx.Err() == nil {
		if err := tailFile(ctx, path, out, ft.send, seekEOF); err != nil {
			slog.Debug("file tail ended", "path", path, "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
	}
}

func tailFile(ctx context.Context, path string, out chan<- string, send sendLine, seekEOF bool) error {
	cfg := tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: false,
		Poll:      true,
	}
	if seekEOF {
		cfg.Location = &tail.SeekInfo{Offset: 0, Whence: 2}
	}
	t, err := tail.TailFile(path, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = t.Stop() }()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line, ok := <-t.Lines:
			if !ok {
				return nil
			}
			if line.Err != nil {
				return line.Err
			}
			if line.Text == "" {
				continue
			}
			send(ctx, out, line.Text)
		}
	}
}

func (ft *fileTailer) watchDirs(ctx context.Context, out chan<- string) {
	dirs := uniqueParentDirs(ft.patterns)
	if len(dirs) == 0 {
		return
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("fsnotify", "err", err)
		return
	}
	defer func() { _ = w.Close() }()
	for _, d := range dirs {
		if err := w.Add(d); err != nil {
			slog.Warn("fsnotify add", "dir", d, "err", err)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Remove) != 0 {
				ft.refresh(ctx, out)
			}
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			slog.Warn("fsnotify error", "err", err)
		}
	}
}

func expandPatterns(patterns []string) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string
	for _, p := range patterns {
		matches, err := filepath.Glob(p)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || info.IsDir() {
				continue
			}
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			out = append(out, m)
		}
	}
	return out, nil
}

func uniqueParentDirs(patterns []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, p := range patterns {
		dir := filepath.Dir(p)
		if dir == "" || dir == "." {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		out = append(out, dir)
	}
	return out
}
