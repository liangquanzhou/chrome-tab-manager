package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{"work", "my-session", "test_123", "A", "a1b2c3", "工作", "我的收藏", "日本語テスト"}
	for _, name := range valid {
		if err := validateName(name); err != nil {
			t.Errorf("validateName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []struct {
		name string
		desc string
	}{
		{"", "empty"},
		{"../evil", "path traversal"},
		{"has space", "space"},
		{"has/slash", "slash"},
		{"has.dot", "dot"},
		{string(make([]byte, 129)), "too long"},
	}
	for _, tt := range invalid {
		t.Run(tt.desc, func(t *testing.T) {
			if err := validateName(tt.name); err == nil {
				t.Errorf("validateName(%q) = nil, want error for %s", tt.name, tt.desc)
			}
		})
	}
}

func TestAtomicWriteJSON(t *testing.T) {
	dir := t.TempDir()
	data := map[string]string{"key": "value"}

	if err := atomicWriteJSON(dir, "test.json", data); err != nil {
		t.Fatalf("atomicWriteJSON: %v", err)
	}

	// Verify file exists and is valid JSON
	var result map[string]string
	if err := loadJSON(filepath.Join(dir, "test.json"), &result); err != nil {
		t.Fatalf("loadJSON: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("key = %q, want value", result["key"])
	}
}

func TestAtomicWriteJSONNoTmpLeftover(t *testing.T) {
	dir := t.TempDir()
	data := map[string]string{"hello": "world"}

	if err := atomicWriteJSON(dir, "test.json", data); err != nil {
		t.Fatalf("atomicWriteJSON: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "test.json" {
			t.Errorf("unexpected file left: %s", e.Name())
		}
	}
}

func TestAtomicWriteJSONOverwrite(t *testing.T) {
	dir := t.TempDir()

	if err := atomicWriteJSON(dir, "test.json", map[string]int{"v": 1}); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	if err := atomicWriteJSON(dir, "test.json", map[string]int{"v": 2}); err != nil {
		t.Fatalf("write 2: %v", err)
	}

	var result map[string]int
	loadJSON(filepath.Join(dir, "test.json"), &result)
	if result["v"] != 2 {
		t.Errorf("v = %d, want 2", result["v"])
	}
}

func TestListJSONFiles(t *testing.T) {
	dir := t.TempDir()

	atomicWriteJSON(dir, "a.json", map[string]string{})
	atomicWriteJSON(dir, "b.json", map[string]string{})
	os.WriteFile(filepath.Join(dir, "not-json.txt"), []byte("x"), 0600)

	files, err := listJSONFiles(dir)
	if err != nil {
		t.Fatalf("listJSONFiles: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("got %d files, want 2", len(files))
	}
}

func TestListJSONFilesEmpty(t *testing.T) {
	dir := t.TempDir()
	files, err := listJSONFiles(dir)
	if err != nil {
		t.Fatalf("listJSONFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}

func TestListJSONFilesNonexistent(t *testing.T) {
	files, err := listJSONFiles("/nonexistent/path")
	if err != nil {
		t.Fatalf("listJSONFiles: %v", err)
	}
	if files != nil {
		t.Errorf("got %v, want nil", files)
	}
}

// --- Error injection tests ---

func TestAtomicWriteJSON_ReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod not effective on Windows")
	}

	dir := t.TempDir()
	// Make directory read-only so CreateTemp fails
	if err := os.Chmod(dir, 0444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0700) })

	err := atomicWriteJSON(dir, "test.json", map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error writing to read-only directory")
	}
	if !strings.Contains(err.Error(), "create temp") {
		t.Errorf("error should mention create temp, got: %v", err)
	}
}

func TestAtomicWriteJSON_MarshalError(t *testing.T) {
	dir := t.TempDir()
	// channels cannot be marshaled to JSON
	err := atomicWriteJSON(dir, "test.json", make(chan int))
	if err == nil {
		t.Fatal("expected marshal error for channel type")
	}
	if !strings.Contains(err.Error(), "marshal") {
		t.Errorf("error should mention marshal, got: %v", err)
	}
}

func TestAtomicWriteJSON_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	err := atomicWriteJSON(dir, "../evil.json", map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error for path traversal filename")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}
}

func TestLoadJSON_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`{invalid json`), 0600)

	var result map[string]string
	err := loadJSON(path, &result)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error should mention unmarshal, got: %v", err)
	}
}

func TestLoadJSON_NonexistentFile(t *testing.T) {
	var result map[string]string
	err := loadJSON("/nonexistent/file.json", &result)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Errorf("error should mention read, got: %v", err)
	}
}

func TestValidatePathSafe(t *testing.T) {
	valid := []string{"123", "ss_abc", "abc-def_123"}
	for _, id := range valid {
		if err := validatePathSafe(id); err != nil {
			t.Errorf("validatePathSafe(%q) = %v, want nil", id, err)
		}
	}

	invalid := []struct {
		id   string
		desc string
	}{
		{"", "empty"},
		{"has/slash", "contains slash"},
		{"has\\backslash", "contains backslash"},
		{"..", "dot-dot"},
		{"abc/../def", "dot-dot in middle"},
		{".", "single dot"},
		{string(make([]byte, 257)), "too long"},
	}
	for _, tt := range invalid {
		t.Run(tt.desc, func(t *testing.T) {
			if err := validatePathSafe(tt.id); err == nil {
				t.Errorf("validatePathSafe(%q) = nil, want error for %s", tt.id, tt.desc)
			}
		})
	}
}

func TestAtomicWriteJSONSiblingDir(t *testing.T) {
	// Create dir="/tmp/xxx/abc" then attempt to write "../abcdef/evil.json".
	// A naive prefix check (dir="/tmp/xxx/abc", target="/tmp/xxx/abcdef/evil.json")
	// would pass because "/tmp/xxx/abcdef" starts with "/tmp/xxx/abc".
	// The current implementation appends filepath.Separator before checking,
	// so it correctly detects this as path traversal.
	base := t.TempDir()
	dir := filepath.Join(base, "abc")
	os.MkdirAll(dir, 0700)
	// Create the sibling directory so rename would succeed if not blocked
	sibling := filepath.Join(base, "abcdef")
	os.MkdirAll(sibling, 0700)

	err := atomicWriteJSON(dir, "../abcdef/evil.json", map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error for sibling directory path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}

	// Verify no file was created in the sibling directory
	entries, _ := os.ReadDir(sibling)
	for _, e := range entries {
		if e.Name() == "evil.json" {
			t.Error("evil.json should not have been created in sibling directory")
		}
	}
}

func TestListJSONFiles_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod not effective on Windows")
	}

	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0700)
	os.WriteFile(filepath.Join(subDir, "a.json"), []byte("{}"), 0600)

	// Make parent dir unreadable
	os.Chmod(dir, 0000)
	t.Cleanup(func() { os.Chmod(dir, 0700) })

	_, err := listJSONFiles(dir)
	if err == nil {
		t.Fatal("expected error for unreadable directory")
	}
}
