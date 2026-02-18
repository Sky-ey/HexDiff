package patch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	hexdiff "github.com/Sky-ey/HexDiff/pkg/diff"
)

func TestDirPatchHeaderMarshalUnmarshal(t *testing.T) {
	original := &DirPatchHeader{
		Magic:         DirPatchMagic,
		Version:       DirPatchVersion,
		Timestamp:     1234567890,
		OldDirNameLen: 3,
		NewDirNameLen: 3,
		FileCount:     5,
		MetadataLen:   10,
	}

	data := original.Marshal()
	if len(data) != DirPatchHeaderSize {
		t.Errorf("Marshal() returned %d bytes, want %d", len(data), DirPatchHeaderSize)
	}

	parsed := &DirPatchHeader{}
	err := parsed.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if parsed.Magic != original.Magic {
		t.Errorf("Magic = %x, want %x", parsed.Magic, original.Magic)
	}
	if parsed.Version != original.Version {
		t.Errorf("Version = %d, want %d", parsed.Version, original.Version)
	}
	if parsed.Timestamp != original.Timestamp {
		t.Errorf("Timestamp = %d, want %d", parsed.Timestamp, original.Timestamp)
	}
	if parsed.OldDirNameLen != original.OldDirNameLen {
		t.Errorf("OldDirNameLen = %d, want %d", parsed.OldDirNameLen, original.OldDirNameLen)
	}
	if parsed.FileCount != original.FileCount {
		t.Errorf("FileCount = %d, want %d", parsed.FileCount, original.FileCount)
	}
}

func TestDirPatchHeaderValidate(t *testing.T) {
	tests := []struct {
		name    string
		header  *DirPatchHeader
		wantErr bool
	}{
		{
			name: "valid header",
			header: &DirPatchHeader{
				Magic:   DirPatchMagic,
				Version: DirPatchVersion,
			},
			wantErr: false,
		},
		{
			name: "invalid magic",
			header: &DirPatchHeader{
				Magic:   0xDEADBEEF,
				Version: DirPatchVersion,
			},
			wantErr: true,
		},
		{
			name: "invalid version",
			header: &DirPatchHeader{
				Magic:   DirPatchMagic,
				Version: 99,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.header.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DirPatchHeader.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDirPatchEntryMarshalUnmarshal(t *testing.T) {
	original := &DirPatchEntry{
		PathLen:       10,
		Status:        1,
		Mode:          0644,
		MTime:         1234567890,
		Size:          1000,
		DataLen:       500,
		IsFullContent: 1,
	}

	data := original.Marshal()
	if len(data) != 64 {
		t.Errorf("Marshal() returned %d bytes, want 64", len(data))
	}

	parsed := &DirPatchEntry{}
	err := parsed.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if parsed.PathLen != original.PathLen {
		t.Errorf("PathLen = %d, want %d", parsed.PathLen, original.PathLen)
	}
	if parsed.Status != original.Status {
		t.Errorf("Status = %d, want %d", parsed.Status, original.Status)
	}
	if parsed.Mode != original.Mode {
		t.Errorf("Mode = %d, want %d", parsed.Mode, original.Mode)
	}
	if parsed.MTime != original.MTime {
		t.Errorf("MTime = %d, want %d", parsed.MTime, original.MTime)
	}
	if parsed.Size != original.Size {
		t.Errorf("Size = %d, want %d", parsed.Size, original.Size)
	}
	if parsed.DataLen != original.DataLen {
		t.Errorf("DataLen = %d, want %d", parsed.DataLen, original.DataLen)
	}
	if parsed.IsFullContent != original.IsFullContent {
		t.Errorf("IsFullContent = %d, want %d", parsed.IsFullContent, original.IsFullContent)
	}
}

func TestNewDirPatchSerializer(t *testing.T) {
	serializer := NewDirPatchSerializer(CompressionNone)
	if serializer == nil {
		t.Fatal("NewDirPatchSerializer() returned nil")
	}
}

func TestDirPatchSerializerRoundTrip(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()
	patchFile := filepath.Join(t.TempDir(), "test.patch")

	// 创建测试文件
	os.WriteFile(filepath.Join(oldDir, "file1.txt"), []byte("old content"), 0644)
	os.WriteFile(filepath.Join(oldDir, "file2.txt"), []byte("unchanged"), 0644)

	os.WriteFile(filepath.Join(newDir, "file1.txt"), []byte("new content"), 0644)
	os.WriteFile(filepath.Join(newDir, "file2.txt"), []byte("unchanged"), 0644)
	os.WriteFile(filepath.Join(newDir, "file3.txt"), []byte("new file"), 0644)

	// 创建目录差异结果
	dirDiffResult := hexdiff.NewDirDiffResult(oldDir, newDir)

	// 添加新增文件
	dirDiffResult.AddFileDiff(&hexdiff.FileDiff{
		RelativePath: "file3.txt",
		Status:       hexdiff.StatusAdded,
		NewEntry: &hexdiff.FileEntry{
			Path:         "file3.txt",
			RelativePath: "file3.txt",
			AbsPath:      filepath.Join(newDir, "file3.txt"),
			Size:         9,
			Mode:         0644,
			MTime:        time.Now(),
		},
		PatchData: []byte("new file"),
	})

	// 添加修改文件
	dirDiffResult.AddFileDiff(&hexdiff.FileDiff{
		RelativePath: "file1.txt",
		Status:       hexdiff.StatusModified,
		OldEntry: &hexdiff.FileEntry{
			Path:         "file1.txt",
			RelativePath: "file1.txt",
			AbsPath:      filepath.Join(oldDir, "file1.txt"),
			Size:         11,
			Mode:         0644,
			MTime:        time.Now(),
		},
		NewEntry: &hexdiff.FileEntry{
			Path:         "file1.txt",
			RelativePath: "file1.txt",
			AbsPath:      filepath.Join(newDir, "file1.txt"),
			Size:         10,
			Mode:         0644,
			MTime:        time.Now(),
		},
	})

	// 添加未改变的文件
	dirDiffResult.AddFileDiff(&hexdiff.FileDiff{
		RelativePath: "file2.txt",
		Status:       hexdiff.StatusUnchanged,
		NewEntry: &hexdiff.FileEntry{
			Path:         "file2.txt",
			RelativePath: "file2.txt",
			AbsPath:      filepath.Join(newDir, "file2.txt"),
			Size:         9,
			Mode:         0644,
			MTime:        time.Now(),
		},
	})

	serializer := NewDirPatchSerializer(CompressionNone)
	err := serializer.SerializeDirPatch(dirDiffResult, "old", "new", patchFile)
	if err != nil {
		t.Fatalf("SerializeDirPatch() error = %v", err)
	}

	// 验证文件已创建
	info, err := os.Stat(patchFile)
	if err != nil {
		t.Fatalf("Failed to stat patch file: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Patch file is empty")
	}

	// 反序列化
	dirPatch, err := serializer.DeserializeDirPatch(patchFile)
	if err != nil {
		t.Fatalf("DeserializeDirPatch() error = %v", err)
	}

	// 验证反序列化结果
	if dirPatch.OldDir != "old" {
		t.Errorf("OldDir = %v, want old", dirPatch.OldDir)
	}
	if dirPatch.NewDir != "new" {
		t.Errorf("NewDir = %v, want new", dirPatch.NewDir)
	}
	if dirPatch.Version != DirPatchVersion {
		t.Errorf("Version = %d, want %d", dirPatch.Version, DirPatchVersion)
	}

	// 验证文件数量
	fileCount := 0
	for _, f := range dirPatch.Files {
		if f.Status != hexdiff.StatusUnchanged {
			fileCount++
		}
	}
	if fileCount != 2 {
		t.Errorf("Expected 2 changed files, got %d", fileCount)
	}
}

func TestIsDirPatch(t *testing.T) {
	patchFile := filepath.Join(t.TempDir(), "test.patch")

	// 创建测试目录差异
	result := hexdiff.NewDirDiffResult("old", "new")
	result.AddFileDiff(&hexdiff.FileDiff{
		RelativePath: "test.txt",
		Status:       hexdiff.StatusAdded,
		NewEntry: &hexdiff.FileEntry{
			Path:         "test.txt",
			RelativePath: "test.txt",
			Size:         10,
			Mode:         0644,
			MTime:        time.Now(),
		},
		PatchData: []byte("test data"),
	})

	serializer := NewDirPatchSerializer(CompressionNone)
	err := serializer.SerializeDirPatch(result, "old", "new", patchFile)
	if err != nil {
		t.Fatalf("SerializeDirPatch() error = %v", err)
	}

	isDir, err := IsDirPatch(patchFile)
	if err != nil {
		t.Fatalf("IsDirPatch() error = %v", err)
	}
	if !isDir {
		t.Error("Expected patch file to be recognized as directory patch")
	}

	// 测试非目录补丁文件
	regularPatchFile := filepath.Join(t.TempDir(), "regular.patch")
	os.WriteFile(regularPatchFile, []byte("not a valid patch"), 0644)

	isDir, err = IsDirPatch(regularPatchFile)
	if err == nil && isDir {
		t.Error("Expected false for invalid patch file")
	}
}

func TestGetDirPatchInfo(t *testing.T) {
	patchFile := filepath.Join(t.TempDir(), "test.patch")

	result := hexdiff.NewDirDiffResult("old", "new")
	result.AddFileDiff(&hexdiff.FileDiff{
		RelativePath: "test.txt",
		Status:       hexdiff.StatusAdded,
		NewEntry: &hexdiff.FileEntry{
			Path:         "test.txt",
			RelativePath: "test.txt",
			Size:         10,
			Mode:         0644,
			MTime:        time.Now(),
		},
		PatchData: []byte("test data"),
	})

	serializer := NewDirPatchSerializer(CompressionNone)
	err := serializer.SerializeDirPatch(result, "olddir", "newdir", patchFile)
	if err != nil {
		t.Fatalf("SerializeDirPatch() error = %v", err)
	}

	header, err := GetDirPatchInfo(patchFile)
	if err != nil {
		t.Fatalf("GetDirPatchInfo() error = %v", err)
	}

	if header.Magic != DirPatchMagic {
		t.Errorf("Magic = %x, want %x", header.Magic, DirPatchMagic)
	}
	if header.Version != DirPatchVersion {
		t.Errorf("Version = %d, want %d", header.Version, DirPatchVersion)
	}
	if header.OldDirNameLen != 6 { // "olddir"
		t.Errorf("OldDirNameLen = %d, want 6", header.OldDirNameLen)
	}
	if header.NewDirNameLen != 6 { // "newdir"
		t.Errorf("NewDirNameLen = %d, want 6", header.NewDirNameLen)
	}
}
