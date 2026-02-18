package diff

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStatusString(t *testing.T) {
	tests := []struct {
		status   FileStatus
		expected string
	}{
		{StatusUnchanged, "unchanged"},
		{StatusAdded, "added"},
		{StatusDeleted, "deleted"},
		{StatusModified, "modified"},
		{FileStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("FileStatus.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewDirDiffResult(t *testing.T) {
	result := NewDirDiffResult("/old", "/new")

	if result.OldDir != "/old" {
		t.Errorf("OldDir = %v, want /old", result.OldDir)
	}
	if result.NewDir != "/new" {
		t.Errorf("NewDir = %v, want /new", result.NewDir)
	}
	if result.Files == nil {
		t.Error("Files should not be nil")
	}
	if result.AddedFiles == nil {
		t.Error("AddedFiles should not be nil")
	}
	if result.DeletedFiles == nil {
		t.Error("DeletedFiles should not be nil")
	}
	if result.ModifiedFiles == nil {
		t.Error("ModifiedFiles should not be nil")
	}
	if result.UnchangedFiles == nil {
		t.Error("UnchangedFiles should not be nil")
	}
}

func TestDirDiffResultAddFileDiff(t *testing.T) {
	result := NewDirDiffResult("/old", "/new")

	added := &FileDiff{
		RelativePath: "new.txt",
		Status:       StatusAdded,
		NewEntry:     &FileEntry{Path: "new.txt"},
	}
	result.AddFileDiff(added)

	if len(result.AddedFiles) != 1 {
		t.Errorf("AddedFiles length = %d, want 1", len(result.AddedFiles))
	}
	if result.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", result.TotalFiles)
	}
	if result.ChangedFiles != 1 {
		t.Errorf("ChangedFiles = %d, want 1", result.ChangedFiles)
	}

	deleted := &FileDiff{
		RelativePath: "old.txt",
		Status:       StatusDeleted,
		OldEntry:     &FileEntry{Path: "old.txt"},
	}
	result.AddFileDiff(deleted)

	if len(result.DeletedFiles) != 1 {
		t.Errorf("DeletedFiles length = %d, want 1", len(result.DeletedFiles))
	}

	unchanged := &FileDiff{
		RelativePath: "same.txt",
		Status:       StatusUnchanged,
	}
	result.AddFileDiff(unchanged)

	if len(result.UnchangedFiles) != 1 {
		t.Errorf("UnchangedFiles length = %d, want 1", len(result.UnchangedFiles))
	}
	if result.ChangedFiles != 2 {
		t.Errorf("ChangedFiles = %d, want 2", result.ChangedFiles)
	}
}

func TestNewDirPatch(t *testing.T) {
	patch := NewDirPatch("old", "new")

	if patch.Version != 1 {
		t.Errorf("Version = %d, want 1", patch.Version)
	}
	if patch.OldDir != "old" {
		t.Errorf("OldDir = %v, want old", patch.OldDir)
	}
	if patch.NewDir != "new" {
		t.Errorf("NewDir = %v, want new", patch.NewDir)
	}
	if patch.Files == nil {
		t.Error("Files should not be nil")
	}
	if patch.Metadata == nil {
		t.Error("Metadata should not be nil")
	}
}

func TestDirPatchAddFile(t *testing.T) {
	patch := NewDirPatch("old", "new")

	file := &DirPatchFile{
		RelativePath: "test.txt",
		Status:       StatusAdded,
		Mode:         0644,
		MTime:        time.Now().Unix(),
		Size:         100,
	}
	patch.AddFile(file)

	if patch.GetFileCount() != 1 {
		t.Errorf("GetFileCount() = %d, want 1", patch.GetFileCount())
	}
}

func TestDirPatchFileGetMTime(t *testing.T) {
	nowUnix := time.Now().Unix()
	file := &DirPatchFile{
		MTime: nowUnix,
	}

	result := file.GetMTime()

	if result.Unix() != nowUnix {
		t.Errorf("GetMTime() = %v, want %v", result.Unix(), nowUnix)
	}
}

func TestDefaultDirDiffConfig(t *testing.T) {
	config := DefaultDirDiffConfig()

	if !config.Recursive {
		t.Error("Recursive should be true by default")
	}
	if config.FollowSymlinks {
		t.Error("FollowSymlinks should be false by default")
	}
	if config.IgnoreHidden {
		t.Error("IgnoreHidden should be false by default")
	}
	if config.WorkerCount != 4 {
		t.Errorf("WorkerCount = %d, want 4", config.WorkerCount)
	}
	if config.BlockSize != DefaultBlockSize {
		t.Errorf("BlockSize = %d, want %d", config.BlockSize, DefaultBlockSize)
	}
}

func TestDirDiffConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *DirDiffConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &DirDiffConfig{
				WorkerCount: 4,
				BlockSize:   4096,
			},
			wantErr: false,
		},
		{
			name: "invalid worker count - too low",
			config: &DirDiffConfig{
				WorkerCount: 0,
				BlockSize:   4096,
			},
			wantErr: true,
		},
		{
			name: "invalid worker count - too high",
			config: &DirDiffConfig{
				WorkerCount: 33,
				BlockSize:   4096,
			},
			wantErr: true,
		},
		{
			name: "invalid block size - too low",
			config: &DirDiffConfig{
				WorkerCount: 4,
				BlockSize:   32,
			},
			wantErr: true,
		},
		{
			name: "invalid block size - too high",
			config: &DirDiffConfig{
				WorkerCount: 4,
				BlockSize:   65537,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DirDiffConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFileEntry(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	entry := &FileEntry{
		Path:         "test.txt",
		RelativePath: "test.txt",
		AbsPath:      testFile,
		Size:         info.Size(),
		Mode:         info.Mode(),
		MTime:        info.ModTime(),
		IsDir:        info.IsDir(),
	}

	if entry.Path != "test.txt" {
		t.Errorf("Path = %v, want test.txt", entry.Path)
	}
	if entry.AbsPath != testFile {
		t.Errorf("AbsPath = %v, want %v", entry.AbsPath, testFile)
	}
	if entry.IsDir {
		t.Error("IsDir should be false for a file")
	}
}
