package sync

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SyncMeta holds versioned metadata for the sync system.
type SyncMeta struct {
	Version    int                   `json:"version"`
	DeviceID   string                `json:"deviceId"`
	LastSyncAt time.Time             `json:"lastSyncAt"`
	Objects    map[string]ObjectMeta `json:"objects"`
}

// ObjectMeta tracks per-object sync state.
type ObjectMeta struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
	Size      int64     `json:"size"`
	Checksum  string    `json:"checksum"` // SHA256 of content
}

// SyncEngine manages bidirectional file sync between local and cloud directories.
type SyncEngine struct {
	LocalDir string
	CloudDir string
}

// SyncStatus represents the current state of the sync system.
type SyncStatus struct {
	Enabled        bool     `json:"enabled"`
	SyncDir        string   `json:"syncDir"`
	LastSync       string   `json:"lastSync"`
	PendingChanges int      `json:"pendingChanges"`
	Conflicts      []string `json:"conflicts"`
	// Metadata fields
	MetaVersion int    `json:"metaVersion"`
	DeviceID    string `json:"deviceId"`
	ObjectCount int    `json:"objectCount"`
}

// RepairResult reports the outcome of a sync repair operation.
type RepairResult struct {
	Reindexed         bool `json:"reindexed"`
	ObjectCount       int  `json:"objectCount"`
	ConflictsResolved int  `json:"conflictsResolved"`
}

const syncMetaFile = "sync_meta.json"

// NewSyncEngine creates a SyncEngine with the given local and cloud directories.
func NewSyncEngine(localDir, cloudDir string) *SyncEngine {
	return &SyncEngine{
		LocalDir: localDir,
		CloudDir: cloudDir,
	}
}

// Status checks the sync state: whether cloud dir exists, pending changes, and conflicts.
func (s *SyncEngine) Status() (*SyncStatus, error) {
	status := &SyncStatus{
		SyncDir:   s.CloudDir,
		Conflicts: []string{},
	}

	// Check if cloud directory exists
	info, err := os.Stat(s.CloudDir)
	if err != nil {
		if os.IsNotExist(err) {
			status.Enabled = false
			return status, nil
		}
		return nil, fmt.Errorf("stat cloud dir: %w", err)
	}
	if !info.IsDir() {
		status.Enabled = false
		return status, nil
	}

	status.Enabled = true

	// Load sync metadata
	meta, _ := loadSyncMeta(s.CloudDir)
	if meta != nil {
		if !meta.LastSyncAt.IsZero() {
			status.LastSync = meta.LastSyncAt.UTC().Format(time.RFC3339)
		}
		status.MetaVersion = meta.Version
		status.DeviceID = meta.DeviceID
		status.ObjectCount = len(meta.Objects)
	} else {
		// Fall back to legacy .ctm_sync_meta.json if new meta doesn't exist
		legacyPath := filepath.Join(s.CloudDir, ".ctm_sync_meta.json")
		if data, err := os.ReadFile(legacyPath); err == nil {
			var legacy struct {
				LastSync string `json:"lastSync"`
			}
			if json.Unmarshal(data, &legacy) == nil {
				status.LastSync = legacy.LastSync
			}
		}
	}

	// Count pending changes using metadata-aware comparison
	pending, conflicts, err := s.compareFiles()
	if err != nil {
		return nil, fmt.Errorf("compare files: %w", err)
	}
	status.PendingChanges = pending
	status.Conflicts = conflicts

	return status, nil
}

// Repair reindexes sync state and resolves conflicts.
func (s *SyncEngine) Repair() (*RepairResult, error) {
	result := &RepairResult{
		Reindexed: true,
	}

	// Ensure cloud dir exists
	if err := os.MkdirAll(s.CloudDir, 0700); err != nil {
		return nil, fmt.Errorf("ensure cloud dir: %w", err)
	}

	// Count all objects in local dir
	localFiles, err := listAllJSONFiles(s.LocalDir)
	if err != nil {
		return nil, fmt.Errorf("list local files: %w", err)
	}
	result.ObjectCount = len(localFiles)

	// Resolve conflicts: for any conflict, local wins (last-write-wins)
	_, conflicts, err := s.compareFiles()
	if err != nil {
		return nil, fmt.Errorf("compare files: %w", err)
	}

	for _, cf := range conflicts {
		localPath := filepath.Join(s.LocalDir, cf)
		cloudPath := filepath.Join(s.CloudDir, cf)
		if err := copyFile(localPath, cloudPath); err == nil {
			result.ConflictsResolved++
		}
	}

	// Rebuild metadata from scratch
	if err := s.rebuildMeta(); err != nil {
		return nil, fmt.Errorf("rebuild meta: %w", err)
	}

	return result, nil
}

// SyncToCloud copies all local JSON files to the cloud directory.
func (s *SyncEngine) SyncToCloud() error {
	if err := os.MkdirAll(s.CloudDir, 0700); err != nil {
		return fmt.Errorf("ensure cloud dir: %w", err)
	}

	files, err := listAllJSONFiles(s.LocalDir)
	if err != nil {
		return fmt.Errorf("list local files: %w", err)
	}

	// Load or create metadata
	meta, _ := loadSyncMeta(s.CloudDir)
	if meta == nil {
		deviceID, err := getOrCreateDeviceID(s.CloudDir)
		if err != nil {
			return fmt.Errorf("get device id: %w", err)
		}
		meta = &SyncMeta{
			DeviceID: deviceID,
			Objects:  make(map[string]ObjectMeta),
		}
	}

	for _, f := range files {
		src := filepath.Join(s.LocalDir, f)
		dst := filepath.Join(s.CloudDir, f)

		// Ensure subdirectories exist in cloud
		dstDir := filepath.Dir(dst)
		if err := os.MkdirAll(dstDir, 0700); err != nil {
			return fmt.Errorf("ensure dir %s: %w", dstDir, err)
		}

		// Read source content for checksum
		content, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}

		// Check if content actually changed
		checksum := checksumBytes(content)
		if existing, ok := meta.Objects[f]; ok && existing.Checksum == checksum {
			continue // No actual change, skip
		}

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", f, err)
		}

		// Update object metadata
		objMeta := meta.Objects[f]
		objMeta.Version++
		objMeta.UpdatedAt = time.Now().UTC()
		objMeta.Size = int64(len(content))
		objMeta.Checksum = checksum
		meta.Objects[f] = objMeta
	}

	// Remove metadata for objects that no longer exist locally
	fileSet := make(map[string]struct{}, len(files))
	for _, f := range files {
		fileSet[f] = struct{}{}
	}
	for key := range meta.Objects {
		if _, exists := fileSet[key]; !exists {
			delete(meta.Objects, key)
		}
	}

	meta.Version++
	meta.LastSyncAt = time.Now().UTC()

	if err := saveSyncMeta(s.CloudDir, meta); err != nil {
		return fmt.Errorf("save sync meta: %w", err)
	}

	return nil
}

// SyncFromCloud copies all cloud JSON files to the local directory.
func (s *SyncEngine) SyncFromCloud() error {
	files, err := listAllJSONFiles(s.CloudDir)
	if err != nil {
		return fmt.Errorf("list cloud files: %w", err)
	}

	for _, f := range files {
		src := filepath.Join(s.CloudDir, f)
		dst := filepath.Join(s.LocalDir, f)

		// Ensure subdirectories exist locally
		dstDir := filepath.Dir(dst)
		if err := os.MkdirAll(dstDir, 0700); err != nil {
			return fmt.Errorf("ensure dir %s: %w", dstDir, err)
		}

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", f, err)
		}
	}

	// Update sync meta with last sync time
	meta, _ := loadSyncMeta(s.CloudDir)
	if meta != nil {
		meta.LastSyncAt = time.Now().UTC()
		meta.Version++
		saveSyncMeta(s.CloudDir, meta)
	}

	return nil
}

// compareFiles compares local and cloud directories, returning counts of
// pending changes and file names with conflicts. Uses metadata checksums
// when available, falling back to modtime comparison.
func (s *SyncEngine) compareFiles() (pending int, conflicts []string, err error) {
	localFiles, err := listAllJSONFiles(s.LocalDir)
	if err != nil {
		return 0, nil, err
	}

	// Load metadata for checksum-based comparison
	meta, _ := loadSyncMeta(s.CloudDir)

	cloudFiles := make(map[string]os.FileInfo)
	cloudFileList, _ := listAllJSONFiles(s.CloudDir)
	for _, f := range cloudFileList {
		info, err := os.Stat(filepath.Join(s.CloudDir, f))
		if err == nil {
			cloudFiles[f] = info
		}
	}

	for _, f := range localFiles {
		localInfo, err := os.Stat(filepath.Join(s.LocalDir, f))
		if err != nil {
			continue
		}

		_, existsInCloud := cloudFiles[f]
		if !existsInCloud {
			pending++
			continue
		}

		// Prefer checksum comparison when metadata is available
		if meta != nil {
			if objMeta, ok := meta.Objects[f]; ok {
				localContent, err := os.ReadFile(filepath.Join(s.LocalDir, f))
				if err != nil {
					continue
				}
				localChecksum := checksumBytes(localContent)
				if localChecksum != objMeta.Checksum {
					// Local content differs from what was last synced
					// Check if cloud also changed
					cloudContent, err := os.ReadFile(filepath.Join(s.CloudDir, f))
					if err != nil {
						continue
					}
					cloudChecksum := checksumBytes(cloudContent)
					if cloudChecksum != objMeta.Checksum {
						// Both changed — conflict
						conflicts = append(conflicts, f)
					} else {
						// Only local changed
						pending++
					}
				} else {
					// Local matches synced version, check if cloud changed
					cloudContent, err := os.ReadFile(filepath.Join(s.CloudDir, f))
					if err != nil {
						continue
					}
					cloudChecksum := checksumBytes(cloudContent)
					if cloudChecksum != objMeta.Checksum {
						// Only cloud changed
						conflicts = append(conflicts, f)
					}
					// Both match — no change
				}
				continue
			}
		}

		// Fallback: modtime comparison
		localMod := localInfo.ModTime()
		cloudInfo := cloudFiles[f]
		cloudMod := cloudInfo.ModTime()

		diff := localMod.Sub(cloudMod)
		if diff > time.Second {
			pending++
		} else if diff < -time.Second {
			conflicts = append(conflicts, f)
		}
	}

	// Check for files only in cloud (not in local)
	for _, f := range cloudFileList {
		localPath := filepath.Join(s.LocalDir, f)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			pending++
		}
	}

	return pending, conflicts, nil
}

// rebuildMeta rebuilds sync metadata from scratch by scanning all files.
func (s *SyncEngine) rebuildMeta() error {
	deviceID, err := getOrCreateDeviceID(s.CloudDir)
	if err != nil {
		return fmt.Errorf("get device id: %w", err)
	}

	// Load existing meta to preserve version counter
	existing, _ := loadSyncMeta(s.CloudDir)
	version := 0
	if existing != nil {
		version = existing.Version
	}

	meta := &SyncMeta{
		Version:    version + 1,
		DeviceID:   deviceID,
		LastSyncAt: time.Now().UTC(),
		Objects:    make(map[string]ObjectMeta),
	}

	// Scan cloud directory for all objects
	cloudFiles, err := listAllJSONFiles(s.CloudDir)
	if err != nil {
		return fmt.Errorf("list cloud files: %w", err)
	}

	for _, f := range cloudFiles {
		path := filepath.Join(s.CloudDir, f)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		objVersion := 1
		if existing != nil {
			if existingObj, ok := existing.Objects[f]; ok {
				objVersion = existingObj.Version
			}
		}

		meta.Objects[f] = ObjectMeta{
			Version:   objVersion,
			UpdatedAt: info.ModTime().UTC(),
			Size:      info.Size(),
			Checksum:  checksumBytes(content),
		}
	}

	return saveSyncMeta(s.CloudDir, meta)
}

// getOrCreateDeviceID reads or generates a persistent device identifier.
func getOrCreateDeviceID(syncDir string) (string, error) {
	path := filepath.Join(syncDir, "device_id")
	data, err := os.ReadFile(path)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	id, err := generateUUID()
	if err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(syncDir, 0700); err != nil {
		return "", fmt.Errorf("ensure sync dir: %w", err)
	}

	// Write atomically: tmp -> fsync -> rename
	tmp, err := os.CreateTemp(syncDir, ".tmp-device-id-*")
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.WriteString(id + "\n"); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write temp: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", fmt.Errorf("fsync: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("rename: %w", err)
	}

	tmpPath = "" // prevent cleanup
	return id, nil
}

// generateUUID generates a version 4 UUID using crypto/rand.
func generateUUID() (string, error) {
	var uuid [16]byte
	if _, err := io.ReadFull(rand.Reader, uuid[:]); err != nil {
		return "", err
	}
	// Set version 4
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant RFC 4122
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}

// checksumBytes computes SHA256 hex digest of the given content.
func checksumBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// checksumFile computes SHA256 hex digest of a file's content.
func checksumFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return checksumBytes(data), nil
}

// loadSyncMeta reads sync metadata from the sync directory.
func loadSyncMeta(syncDir string) (*SyncMeta, error) {
	path := filepath.Join(syncDir, syncMetaFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta SyncMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal sync meta: %w", err)
	}

	if meta.Objects == nil {
		meta.Objects = make(map[string]ObjectMeta)
	}

	return &meta, nil
}

// saveSyncMeta writes sync metadata atomically.
func saveSyncMeta(syncDir string, meta *SyncMeta) error {
	if err := os.MkdirAll(syncDir, 0700); err != nil {
		return fmt.Errorf("ensure sync dir: %w", err)
	}

	content, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sync meta: %w", err)
	}

	target := filepath.Join(syncDir, syncMetaFile)

	tmp, err := os.CreateTemp(syncDir, ".tmp-sync-meta-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	defer func() {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("fsync: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	tmpPath = "" // prevent cleanup
	return nil
}

// listAllJSONFiles recursively finds all .json files in the directory,
// returning paths relative to the base directory.
// Excludes hidden files (starting with '.') and sync_meta.json.
func listAllJSONFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".json") &&
			!strings.HasPrefix(info.Name(), ".") &&
			info.Name() != syncMetaFile {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return files, nil
}

// copyFile copies src to dst, creating dst's parent directory if needed.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return out.Sync()
}
