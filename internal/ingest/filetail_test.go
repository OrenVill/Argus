package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartupPathsOnlySeekEOFForInitialFiles(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "boot.log")
	if err := os.WriteFile(existing, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ft := &fileTailer{patterns: []string{filepath.Join(dir, "*.log")}}
	paths, err := expandPatterns(ft.patterns)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		ft.startupPaths.Store(p, struct{}{})
	}
	if _, ok := ft.startupPaths.Load(existing); !ok {
		t.Fatal("expected existing file in startup set")
	}
	newFile := filepath.Join(dir, "new.log")
	if _, ok := ft.startupPaths.Load(newFile); ok {
		t.Fatal("new file should not be in startup set")
	}
}
