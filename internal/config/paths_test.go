package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfigDirDefault(t *testing.T) {
	t.Setenv("CTM_CONFIG_DIR", "")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "ctm")
	if got := ConfigDir(); got != want {
		t.Errorf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestConfigDirEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTM_CONFIG_DIR", tmp)
	if got := ConfigDir(); got != tmp {
		t.Errorf("ConfigDir() = %q, want %q", got, tmp)
	}
}

func TestSocketPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTM_CONFIG_DIR", tmp)
	want := filepath.Join(tmp, "daemon.sock")
	if got := SocketPath(); got != want {
		t.Errorf("SocketPath() = %q, want %q", got, want)
	}
}

func TestSubdirectoryPaths(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTM_CONFIG_DIR", tmp)

	tests := []struct {
		name string
		fn   func() string
		sub  string
	}{
		{"SessionsDir", SessionsDir, "sessions"},
		{"CollectionsDir", CollectionsDir, "collections"},
		{"BookmarksDir", BookmarksDir, "bookmarks"},
		{"OverlaysDir", OverlaysDir, "overlays"},
		{"WorkspacesDir", WorkspacesDir, "workspaces"},
		{"SavedSearchesDir", SavedSearchesDir, "searches"},
		{"ExtensionDir", ExtensionDir, "extension"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := filepath.Join(tmp, tt.sub)
			if got := tt.fn(); got != want {
				t.Errorf("%s() = %q, want %q", tt.name, got, want)
			}
		})
	}
}

func TestLogPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTM_CONFIG_DIR", tmp)
	want := filepath.Join(tmp, "daemon.log")
	if got := LogPath(); got != want {
		t.Errorf("LogPath() = %q, want %q", got, want)
	}
}

func TestLockPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTM_CONFIG_DIR", tmp)
	want := filepath.Join(tmp, "daemon.lock")
	if got := LockPath(); got != want {
		t.Errorf("LockPath() = %q, want %q", got, want)
	}
}

func TestSyncDir(t *testing.T) {
	got := SyncDir()
	if !strings.Contains(got, "Mobile Documents") && !strings.Contains(got, "ctm-sync") {
		t.Errorf("SyncDir() = %q, expected to contain 'Mobile Documents' or 'ctm-sync'", got)
	}
}

func TestEnsureDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTM_CONFIG_DIR", tmp)

	if err := EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error: %v", err)
	}

	dirs := []string{
		"sessions", "collections", "bookmarks", "overlays",
		"workspaces", "searches", "extension",
	}
	for _, d := range dirs {
		path := filepath.Join(tmp, d)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("directory %s not created: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
		if perm := info.Mode().Perm(); perm != 0700 {
			t.Errorf("%s has permissions %o, want 0700", d, perm)
		}
	}
}

// --- Error injection tests ---

func TestConfigDir_CTMConfigDirEnv(t *testing.T) {
	custom := "/tmp/ctm-test-custom-config"
	t.Setenv("CTM_CONFIG_DIR", custom)

	got := ConfigDir()
	if got != custom {
		t.Errorf("ConfigDir() = %q, want %q", got, custom)
	}

	// All path functions should use the custom base
	if !strings.HasPrefix(SocketPath(), custom) {
		t.Errorf("SocketPath() = %q, should be under %q", SocketPath(), custom)
	}
	if !strings.HasPrefix(SessionsDir(), custom) {
		t.Errorf("SessionsDir() = %q, should be under %q", SessionsDir(), custom)
	}
}

func TestSyncDir_DefaultPath(t *testing.T) {
	got := SyncDir()
	// On a typical macOS/Linux system, it should contain "Mobile Documents" or "ctm-sync"
	if got == "" {
		t.Error("SyncDir() should not be empty")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		expected := filepath.Join(home, "Library", "Mobile Documents", "com~ctm")
		if got != expected {
			// Might be the temp fallback on non-macOS
			if !strings.Contains(got, "ctm-sync") {
				t.Errorf("SyncDir() = %q, expected %q or a ctm-sync path", got, expected)
			}
		}
	}
}

func TestEnsureDirs_ReadOnlyParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod not effective on Windows")
	}

	// Create a base dir and make it read-only
	baseDir := t.TempDir()
	readOnlyDir := filepath.Join(baseDir, "readonly")
	os.MkdirAll(readOnlyDir, 0700)
	os.Chmod(readOnlyDir, 0444)
	t.Cleanup(func() { os.Chmod(readOnlyDir, 0700) })

	t.Setenv("CTM_CONFIG_DIR", filepath.Join(readOnlyDir, "ctm"))

	err := EnsureDirs()
	if err == nil {
		t.Fatal("expected error creating dirs under read-only parent")
	}
	if !strings.Contains(err.Error(), "ensure dir") {
		t.Errorf("error should mention ensure dir, got: %v", err)
	}
}

func TestEnsureDirs_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CTM_CONFIG_DIR", tmp)

	// Call twice — should succeed both times
	if err := EnsureDirs(); err != nil {
		t.Fatalf("first EnsureDirs() error: %v", err)
	}
	if err := EnsureDirs(); err != nil {
		t.Fatalf("second EnsureDirs() error: %v", err)
	}
}
