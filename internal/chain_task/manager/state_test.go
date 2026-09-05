package manager

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewStateManagerUsesGlobalVideoDirectory(t *testing.T) {
	root := t.TempDir()
	created := time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC)
	state := NewStateManager(1, "video_123", root, created)

	wantDir := filepath.Join(root, "2026-01-03", "video_123")
	if state.CurrentDir != wantDir {
		t.Fatalf("CurrentDir = %q, want %q", state.CurrentDir, wantDir)
	}
	if state.InputVideoPath != filepath.Join(wantDir, "video_123.mp4") {
		t.Fatalf("unexpected input path: %q", state.InputVideoPath)
	}
}

func TestStateManagerCache(t *testing.T) {
	state := NewStateManager(1, "video_123", t.TempDir(), time.Now())
	state.SetCache("key", "value")

	value, ok := state.GetCache("key")
	if !ok || value != "value" {
		t.Fatalf("cache = (%v, %v), want (value, true)", value, ok)
	}
}
