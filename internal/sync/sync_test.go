package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStatusCloudNotExist(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := filepath.Join(t.TempDir(), "nonexistent")

	engine := NewSyncEngine(localDir, cloudDir)
	status, err := engine.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Enabled {
		t.Error("should be disabled when cloud dir doesn't exist")
	}
	if status.SyncDir != cloudDir {
		t.Errorf("SyncDir = %q, want %q", status.SyncDir, cloudDir)
	}
}

func TestStatusEnabled(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	engine := NewSyncEngine(localDir, cloudDir)
	status, err := engine.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Enabled {
		t.Error("should be enabled when cloud dir exists")
	}
}

func TestStatusPendingChanges(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	// Write a file only in local
	os.WriteFile(filepath.Join(localDir, "test.json"), []byte(`{"key":"value"}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	status, err := engine.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.PendingChanges == 0 {
		t.Error("should have pending changes when local has files not in cloud")
	}
}

func TestRepairCreatesCloudDir(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := filepath.Join(t.TempDir(), "cloud")

	os.WriteFile(filepath.Join(localDir, "a.json"), []byte(`{}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	result, err := engine.Repair()
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if !result.Reindexed {
		t.Error("should report reindexed")
	}
	if result.ObjectCount != 1 {
		t.Errorf("ObjectCount = %d, want 1", result.ObjectCount)
	}

	// Cloud dir should exist
	if _, err := os.Stat(cloudDir); os.IsNotExist(err) {
		t.Error("cloud dir should have been created")
	}
}

func TestSyncToCloud(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := filepath.Join(t.TempDir(), "cloud")

	os.WriteFile(filepath.Join(localDir, "session1.json"), []byte(`{"name":"s1"}`), 0600)
	os.WriteFile(filepath.Join(localDir, "session2.json"), []byte(`{"name":"s2"}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	if err := engine.SyncToCloud(); err != nil {
		t.Fatalf("SyncToCloud: %v", err)
	}

	// Verify files exist in cloud
	for _, name := range []string{"session1.json", "session2.json"} {
		if _, err := os.Stat(filepath.Join(cloudDir, name)); os.IsNotExist(err) {
			t.Errorf("file %s should exist in cloud", name)
		}
	}

	// Verify sync metadata was written
	metaPath := filepath.Join(cloudDir, syncMetaFile)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read sync meta: %v", err)
	}
	var meta SyncMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal sync meta: %v", err)
	}
	if meta.Version != 1 {
		t.Errorf("Version = %d, want 1", meta.Version)
	}
	if meta.DeviceID == "" {
		t.Error("DeviceID should be set")
	}
	if meta.LastSyncAt.IsZero() {
		t.Error("LastSyncAt should be set")
	}
	if len(meta.Objects) != 2 {
		t.Errorf("Objects count = %d, want 2", len(meta.Objects))
	}
	for _, name := range []string{"session1.json", "session2.json"} {
		obj, ok := meta.Objects[name]
		if !ok {
			t.Errorf("missing object metadata for %s", name)
			continue
		}
		if obj.Version != 1 {
			t.Errorf("%s version = %d, want 1", name, obj.Version)
		}
		if obj.Checksum == "" {
			t.Errorf("%s checksum should not be empty", name)
		}
		if obj.Size == 0 {
			t.Errorf("%s size should not be 0", name)
		}
	}
}

func TestSyncFromCloud(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	os.WriteFile(filepath.Join(cloudDir, "workspace1.json"), []byte(`{"name":"ws1"}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	if err := engine.SyncFromCloud(); err != nil {
		t.Fatalf("SyncFromCloud: %v", err)
	}

	if _, err := os.Stat(filepath.Join(localDir, "workspace1.json")); os.IsNotExist(err) {
		t.Error("file should be copied to local")
	}
}

func TestRepairEmptyDirs(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := filepath.Join(t.TempDir(), "cloud")

	engine := NewSyncEngine(localDir, cloudDir)
	result, err := engine.Repair()
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if result.ObjectCount != 0 {
		t.Errorf("ObjectCount = %d, want 0", result.ObjectCount)
	}
	if result.ConflictsResolved != 0 {
		t.Errorf("ConflictsResolved = %d, want 0", result.ConflictsResolved)
	}
}

// --- Error injection tests ---

func TestCompareFiles_LocalNewerThanCloud(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	// Create file in cloud first
	cloudFile := filepath.Join(cloudDir, "session.json")
	os.WriteFile(cloudFile, []byte(`{"v":1}`), 0600)
	// Set cloud file to be older
	oldTime := time.Now().Add(-10 * time.Minute)
	os.Chtimes(cloudFile, oldTime, oldTime)

	// Create same file in local (newer)
	localFile := filepath.Join(localDir, "session.json")
	os.WriteFile(localFile, []byte(`{"v":2}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	pending, conflicts, err := engine.compareFiles()
	if err != nil {
		t.Fatalf("compareFiles: %v", err)
	}
	if pending != 1 {
		t.Errorf("pending = %d, want 1 (local is newer)", pending)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %v, want none", conflicts)
	}
}

func TestCompareFiles_CloudNewerThanLocal(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	// Create file in local first (older)
	localFile := filepath.Join(localDir, "session.json")
	os.WriteFile(localFile, []byte(`{"v":1}`), 0600)
	oldTime := time.Now().Add(-10 * time.Minute)
	os.Chtimes(localFile, oldTime, oldTime)

	// Create same file in cloud (newer)
	cloudFile := filepath.Join(cloudDir, "session.json")
	os.WriteFile(cloudFile, []byte(`{"v":2}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	pending, conflicts, err := engine.compareFiles()
	if err != nil {
		t.Fatalf("compareFiles: %v", err)
	}
	if pending != 0 {
		t.Errorf("pending = %d, want 0", pending)
	}
	if len(conflicts) != 1 {
		t.Errorf("conflicts count = %d, want 1 (cloud is newer)", len(conflicts))
	}
}

func TestCompareFiles_CloudOnlyFile(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	// File only exists in cloud
	os.WriteFile(filepath.Join(cloudDir, "cloud-only.json"), []byte(`{}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	pending, _, err := engine.compareFiles()
	if err != nil {
		t.Fatalf("compareFiles: %v", err)
	}
	if pending != 1 {
		t.Errorf("pending = %d, want 1 (file only in cloud counts as pending)", pending)
	}
}

func TestCopyFile_SrcNotExist(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dst.json")
	err := copyFile("/nonexistent/src.json", dst)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "open src") {
		t.Errorf("error should mention open src, got: %v", err)
	}
}

func TestCopyFile_DstReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod not effective on Windows")
	}

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "src.json")
	os.WriteFile(srcFile, []byte(`{"k":"v"}`), 0600)

	dstDir := t.TempDir()
	os.Chmod(dstDir, 0444)
	t.Cleanup(func() { os.Chmod(dstDir, 0700) })

	err := copyFile(srcFile, filepath.Join(dstDir, "dst.json"))
	if err == nil {
		t.Fatal("expected error creating file in read-only directory")
	}
	if !strings.Contains(err.Error(), "create dst") {
		t.Errorf("error should mention create dst, got: %v", err)
	}
}

func TestCopyFile_Success(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "src.json")
	os.WriteFile(srcFile, []byte(`{"hello":"world"}`), 0600)

	dstFile := filepath.Join(dstDir, "dst.json")
	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != `{"hello":"world"}` {
		t.Errorf("dst content = %q, want original content", string(data))
	}
}

func TestListAllJSONFiles_Nonexistent(t *testing.T) {
	files, err := listAllJSONFiles("/nonexistent/dir")
	if err != nil {
		t.Fatalf("listAllJSONFiles nonexistent: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestListAllJSONFiles_SkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "visible.json"), []byte("{}"), 0600)
	os.WriteFile(filepath.Join(dir, ".hidden.json"), []byte("{}"), 0600)
	os.WriteFile(filepath.Join(dir, "not-json.txt"), []byte("x"), 0600)

	files, err := listAllJSONFiles(dir)
	if err != nil {
		t.Fatalf("listAllJSONFiles: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file (only visible.json), got %d: %v", len(files), files)
	}
	if len(files) == 1 && files[0] != "visible.json" {
		t.Errorf("expected visible.json, got %q", files[0])
	}
}

func TestListAllJSONFiles_Recursive(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0700)
	os.WriteFile(filepath.Join(dir, "root.json"), []byte("{}"), 0600)
	os.WriteFile(filepath.Join(subDir, "nested.json"), []byte("{}"), 0600)

	files, err := listAllJSONFiles(dir)
	if err != nil {
		t.Fatalf("listAllJSONFiles: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestListAllJSONFiles_SkipsSyncMeta(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0600)
	os.WriteFile(filepath.Join(dir, syncMetaFile), []byte("{}"), 0600)

	files, err := listAllJSONFiles(dir)
	if err != nil {
		t.Fatalf("listAllJSONFiles: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file (sync_meta.json should be excluded), got %d: %v", len(files), files)
	}
}

func TestStatusWithSyncMeta(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	// Write new-format sync metadata
	meta := &SyncMeta{
		Version:    3,
		DeviceID:   "test-device-id",
		LastSyncAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Objects:    map[string]ObjectMeta{},
	}
	saveSyncMeta(cloudDir, meta)

	engine := NewSyncEngine(localDir, cloudDir)
	status, err := engine.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.LastSync != "2026-01-01T00:00:00Z" {
		t.Errorf("LastSync = %q, want 2026-01-01T00:00:00Z", status.LastSync)
	}
	if status.MetaVersion != 3 {
		t.Errorf("MetaVersion = %d, want 3", status.MetaVersion)
	}
	if status.DeviceID != "test-device-id" {
		t.Errorf("DeviceID = %q, want test-device-id", status.DeviceID)
	}
}

func TestStatusWithLegacyMeta(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	// Write legacy format metadata
	legacyMeta := map[string]string{"lastSync": "2026-01-01T00:00:00Z"}
	data, _ := json.MarshalIndent(legacyMeta, "", "  ")
	os.WriteFile(filepath.Join(cloudDir, ".ctm_sync_meta.json"), data, 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	status, err := engine.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.LastSync != "2026-01-01T00:00:00Z" {
		t.Errorf("LastSync = %q, want 2026-01-01T00:00:00Z", status.LastSync)
	}
	// No new metadata, so version and device should be zero/empty
	if status.MetaVersion != 0 {
		t.Errorf("MetaVersion = %d, want 0 (legacy)", status.MetaVersion)
	}
	if status.DeviceID != "" {
		t.Errorf("DeviceID = %q, want empty (legacy)", status.DeviceID)
	}
}

func TestStatusCloudIsFile(t *testing.T) {
	localDir := t.TempDir()
	// Create a file instead of directory for cloud path
	cloudPath := filepath.Join(t.TempDir(), "cloud")
	os.WriteFile(cloudPath, []byte("not a dir"), 0600)

	engine := NewSyncEngine(localDir, cloudPath)
	status, err := engine.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Enabled {
		t.Error("should be disabled when cloud path is a file, not a directory")
	}
}

func TestRepairWithConflicts(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	// Create file in local (older)
	localFile := filepath.Join(localDir, "conflict.json")
	os.WriteFile(localFile, []byte(`{"v":"local"}`), 0600)
	oldTime := time.Now().Add(-10 * time.Minute)
	os.Chtimes(localFile, oldTime, oldTime)

	// Create same file in cloud (newer) -- this creates a conflict
	cloudFile := filepath.Join(cloudDir, "conflict.json")
	os.WriteFile(cloudFile, []byte(`{"v":"cloud"}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	result, err := engine.Repair()
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if result.ConflictsResolved != 1 {
		t.Errorf("ConflictsResolved = %d, want 1", result.ConflictsResolved)
	}

	// After repair, cloud should have local content (local wins)
	data, _ := os.ReadFile(cloudFile)
	if string(data) != `{"v":"local"}` {
		t.Errorf("cloud file content = %q, want local content", string(data))
	}
}

// --- Metadata versioning tests ---

func TestGetOrCreateDeviceID(t *testing.T) {
	dir := t.TempDir()

	// First call should generate an ID
	id1, err := getOrCreateDeviceID(dir)
	if err != nil {
		t.Fatalf("getOrCreateDeviceID: %v", err)
	}
	if id1 == "" {
		t.Fatal("device ID should not be empty")
	}

	// Should be a valid UUID format (8-4-4-4-12)
	parts := strings.Split(id1, "-")
	if len(parts) != 5 {
		t.Errorf("device ID should be UUID format, got %q", id1)
	}

	// Second call should return the same ID
	id2, err := getOrCreateDeviceID(dir)
	if err != nil {
		t.Fatalf("getOrCreateDeviceID second call: %v", err)
	}
	if id1 != id2 {
		t.Errorf("device ID changed: %q -> %q", id1, id2)
	}
}

func TestGetOrCreateDeviceID_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	id, err := getOrCreateDeviceID(dir)
	if err != nil {
		t.Fatalf("getOrCreateDeviceID: %v", err)
	}
	if id == "" {
		t.Fatal("device ID should not be empty")
	}
}

func TestGenerateUUID(t *testing.T) {
	id, err := generateUUID()
	if err != nil {
		t.Fatalf("generateUUID: %v", err)
	}

	// Validate format
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("UUID should have 5 parts, got %d: %q", len(parts), id)
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 ||
		len(parts[3]) != 4 || len(parts[4]) != 12 {
		t.Errorf("UUID part lengths incorrect: %q", id)
	}

	// Version should be 4
	if parts[2][0] != '4' {
		t.Errorf("UUID version should be 4, got %c", parts[2][0])
	}

	// Two UUIDs should be different
	id2, _ := generateUUID()
	if id == id2 {
		t.Error("two generated UUIDs should be different")
	}
}

func TestChecksumBytes(t *testing.T) {
	c1 := checksumBytes([]byte("hello"))
	c2 := checksumBytes([]byte("hello"))
	c3 := checksumBytes([]byte("world"))

	if c1 != c2 {
		t.Error("same content should produce same checksum")
	}
	if c1 == c3 {
		t.Error("different content should produce different checksum")
	}
	if len(c1) != 64 {
		t.Errorf("SHA256 hex should be 64 chars, got %d", len(c1))
	}
}

func TestChecksumFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	content := []byte(`{"key":"value"}`)
	os.WriteFile(path, content, 0600)

	checksum, err := checksumFile(path)
	if err != nil {
		t.Fatalf("checksumFile: %v", err)
	}

	expected := checksumBytes(content)
	if checksum != expected {
		t.Errorf("checksumFile = %q, want %q", checksum, expected)
	}
}

func TestChecksumFile_NotExist(t *testing.T) {
	_, err := checksumFile("/nonexistent/file.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestSyncMetaSaveLoad(t *testing.T) {
	dir := t.TempDir()
	meta := &SyncMeta{
		Version:    5,
		DeviceID:   "test-device",
		LastSyncAt: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC),
		Objects: map[string]ObjectMeta{
			"session.json": {
				Version:   3,
				UpdatedAt: time.Date(2026, 3, 8, 11, 0, 0, 0, time.UTC),
				Size:      42,
				Checksum:  "abc123",
			},
		},
	}

	if err := saveSyncMeta(dir, meta); err != nil {
		t.Fatalf("saveSyncMeta: %v", err)
	}

	loaded, err := loadSyncMeta(dir)
	if err != nil {
		t.Fatalf("loadSyncMeta: %v", err)
	}

	if loaded.Version != 5 {
		t.Errorf("Version = %d, want 5", loaded.Version)
	}
	if loaded.DeviceID != "test-device" {
		t.Errorf("DeviceID = %q, want test-device", loaded.DeviceID)
	}
	if loaded.LastSyncAt.IsZero() {
		t.Error("LastSyncAt should not be zero")
	}
	if len(loaded.Objects) != 1 {
		t.Errorf("Objects count = %d, want 1", len(loaded.Objects))
	}

	obj := loaded.Objects["session.json"]
	if obj.Version != 3 {
		t.Errorf("Object version = %d, want 3", obj.Version)
	}
	if obj.Size != 42 {
		t.Errorf("Object size = %d, want 42", obj.Size)
	}
	if obj.Checksum != "abc123" {
		t.Errorf("Object checksum = %q, want abc123", obj.Checksum)
	}
}

func TestLoadSyncMeta_NotExist(t *testing.T) {
	meta, err := loadSyncMeta("/nonexistent/dir")
	if err == nil {
		t.Fatal("expected error for nonexistent dir")
	}
	if meta != nil {
		t.Error("meta should be nil on error")
	}
}

func TestSyncToCloud_VersionIncrement(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := filepath.Join(t.TempDir(), "cloud")

	os.WriteFile(filepath.Join(localDir, "s.json"), []byte(`{"v":1}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)

	// First sync
	if err := engine.SyncToCloud(); err != nil {
		t.Fatalf("SyncToCloud #1: %v", err)
	}

	meta1, err := loadSyncMeta(cloudDir)
	if err != nil {
		t.Fatalf("loadSyncMeta #1: %v", err)
	}
	if meta1.Version != 1 {
		t.Errorf("meta version after first sync = %d, want 1", meta1.Version)
	}
	if meta1.Objects["s.json"].Version != 1 {
		t.Errorf("object version after first sync = %d, want 1", meta1.Objects["s.json"].Version)
	}

	// Modify file and sync again
	os.WriteFile(filepath.Join(localDir, "s.json"), []byte(`{"v":2}`), 0600)

	if err := engine.SyncToCloud(); err != nil {
		t.Fatalf("SyncToCloud #2: %v", err)
	}

	meta2, err := loadSyncMeta(cloudDir)
	if err != nil {
		t.Fatalf("loadSyncMeta #2: %v", err)
	}
	if meta2.Version != 2 {
		t.Errorf("meta version after second sync = %d, want 2", meta2.Version)
	}
	if meta2.Objects["s.json"].Version != 2 {
		t.Errorf("object version after second sync = %d, want 2", meta2.Objects["s.json"].Version)
	}

	// Sync without changes (same content) -- version should still increment for meta
	// but object version should not change
	if err := engine.SyncToCloud(); err != nil {
		t.Fatalf("SyncToCloud #3: %v", err)
	}

	meta3, err := loadSyncMeta(cloudDir)
	if err != nil {
		t.Fatalf("loadSyncMeta #3: %v", err)
	}
	if meta3.Version != 3 {
		t.Errorf("meta version after no-change sync = %d, want 3", meta3.Version)
	}
	if meta3.Objects["s.json"].Version != 2 {
		t.Errorf("object version should stay at 2 (no change), got %d", meta3.Objects["s.json"].Version)
	}
}

func TestSyncToCloud_ChecksumSkipsUnchanged(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := filepath.Join(t.TempDir(), "cloud")

	content := []byte(`{"data":"unchanged"}`)
	os.WriteFile(filepath.Join(localDir, "s.json"), content, 0600)

	engine := NewSyncEngine(localDir, cloudDir)

	// First sync
	if err := engine.SyncToCloud(); err != nil {
		t.Fatalf("SyncToCloud: %v", err)
	}

	// Record the cloud file modtime
	info1, _ := os.Stat(filepath.Join(cloudDir, "s.json"))
	mod1 := info1.ModTime()

	// Wait briefly to ensure modtime would differ if rewritten
	time.Sleep(10 * time.Millisecond)

	// Sync again with same content -- should skip copy
	if err := engine.SyncToCloud(); err != nil {
		t.Fatalf("SyncToCloud: %v", err)
	}

	info2, _ := os.Stat(filepath.Join(cloudDir, "s.json"))
	mod2 := info2.ModTime()

	if !mod1.Equal(mod2) {
		t.Error("cloud file should not be rewritten when content hasn't changed")
	}
}

func TestSyncToCloud_RemovesDeletedObjects(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := filepath.Join(t.TempDir(), "cloud")

	os.WriteFile(filepath.Join(localDir, "keep.json"), []byte(`{}`), 0600)
	os.WriteFile(filepath.Join(localDir, "remove.json"), []byte(`{}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	engine.SyncToCloud()

	meta1, _ := loadSyncMeta(cloudDir)
	if len(meta1.Objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(meta1.Objects))
	}

	// Delete one file from local
	os.Remove(filepath.Join(localDir, "remove.json"))

	engine.SyncToCloud()

	meta2, _ := loadSyncMeta(cloudDir)
	if len(meta2.Objects) != 1 {
		t.Errorf("expected 1 object after deletion, got %d", len(meta2.Objects))
	}
	if _, ok := meta2.Objects["remove.json"]; ok {
		t.Error("removed file should not be in metadata")
	}
}

func TestSyncToCloud_DeviceIDPersists(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := filepath.Join(t.TempDir(), "cloud")

	os.WriteFile(filepath.Join(localDir, "s.json"), []byte(`{}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	engine.SyncToCloud()

	meta1, _ := loadSyncMeta(cloudDir)
	deviceID := meta1.DeviceID

	// Modify and sync again
	os.WriteFile(filepath.Join(localDir, "s.json"), []byte(`{"v":2}`), 0600)
	engine.SyncToCloud()

	meta2, _ := loadSyncMeta(cloudDir)
	if meta2.DeviceID != deviceID {
		t.Errorf("device ID changed: %q -> %q", deviceID, meta2.DeviceID)
	}
}

func TestRepairRebuildsMetadata(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	// Create files in cloud (simulating existing sync state)
	os.WriteFile(filepath.Join(cloudDir, "a.json"), []byte(`{"name":"a"}`), 0600)
	os.WriteFile(filepath.Join(cloudDir, "b.json"), []byte(`{"name":"b"}`), 0600)

	// Also in local
	os.WriteFile(filepath.Join(localDir, "a.json"), []byte(`{"name":"a"}`), 0600)
	os.WriteFile(filepath.Join(localDir, "b.json"), []byte(`{"name":"b"}`), 0600)

	engine := NewSyncEngine(localDir, cloudDir)
	result, err := engine.Repair()
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if result.ObjectCount != 2 {
		t.Errorf("ObjectCount = %d, want 2", result.ObjectCount)
	}

	// Verify metadata was rebuilt
	meta, err := loadSyncMeta(cloudDir)
	if err != nil {
		t.Fatalf("loadSyncMeta after repair: %v", err)
	}
	if meta.Version != 1 {
		t.Errorf("Version = %d, want 1", meta.Version)
	}
	if meta.DeviceID == "" {
		t.Error("DeviceID should be set after repair")
	}
	if len(meta.Objects) != 2 {
		t.Errorf("Objects count = %d, want 2", len(meta.Objects))
	}
	for _, name := range []string{"a.json", "b.json"} {
		obj, ok := meta.Objects[name]
		if !ok {
			t.Errorf("missing object metadata for %s", name)
			continue
		}
		if obj.Checksum == "" {
			t.Errorf("%s should have checksum", name)
		}
		if obj.Size == 0 {
			t.Errorf("%s should have size", name)
		}
	}
}

func TestCompareFiles_WithMetadata(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	// Create local and cloud with same modtime but different content
	localFile := filepath.Join(localDir, "s.json")
	cloudFile := filepath.Join(cloudDir, "s.json")

	localContent := []byte(`{"v":"local-changed"}`)
	cloudContent := []byte(`{"v":"original"}`)

	os.WriteFile(localFile, localContent, 0600)
	os.WriteFile(cloudFile, cloudContent, 0600)

	// Set same modtime to avoid modtime-based detection
	now := time.Now()
	os.Chtimes(localFile, now, now)
	os.Chtimes(cloudFile, now, now)

	// Create metadata that records the original checksum
	meta := &SyncMeta{
		Version:    1,
		DeviceID:   "test",
		LastSyncAt: now.Add(-time.Hour),
		Objects: map[string]ObjectMeta{
			"s.json": {
				Version:   1,
				UpdatedAt: now.Add(-time.Hour),
				Size:      int64(len(cloudContent)),
				Checksum:  checksumBytes(cloudContent),
			},
		},
	}
	saveSyncMeta(cloudDir, meta)

	engine := NewSyncEngine(localDir, cloudDir)
	pending, conflicts, err := engine.compareFiles()
	if err != nil {
		t.Fatalf("compareFiles: %v", err)
	}
	// Local changed, cloud unchanged -> pending
	if pending != 1 {
		t.Errorf("pending = %d, want 1 (local content changed from sync)", pending)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0", len(conflicts))
	}
}

func TestCompareFiles_BothChangedWithMetadata(t *testing.T) {
	localDir := t.TempDir()
	cloudDir := t.TempDir()

	localFile := filepath.Join(localDir, "s.json")
	cloudFile := filepath.Join(cloudDir, "s.json")

	// Both changed from what was synced
	os.WriteFile(localFile, []byte(`{"v":"local-new"}`), 0600)
	os.WriteFile(cloudFile, []byte(`{"v":"cloud-new"}`), 0600)

	// Metadata records the original synced content
	originalContent := []byte(`{"v":"original"}`)
	meta := &SyncMeta{
		Version:  1,
		DeviceID: "test",
		Objects: map[string]ObjectMeta{
			"s.json": {
				Version:  1,
				Checksum: checksumBytes(originalContent),
			},
		},
	}
	saveSyncMeta(cloudDir, meta)

	engine := NewSyncEngine(localDir, cloudDir)
	pending, conflicts, err := engine.compareFiles()
	if err != nil {
		t.Fatalf("compareFiles: %v", err)
	}
	if pending != 0 {
		t.Errorf("pending = %d, want 0", pending)
	}
	if len(conflicts) != 1 {
		t.Errorf("conflicts = %d, want 1 (both changed = conflict)", len(conflicts))
	}
}
