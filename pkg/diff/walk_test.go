package diff

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建测试文件
	files := map[string]string{
		"file1.txt":        "content1",
		"file2.txt":        "content2",
		"subdir/file3.txt": "content3",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	config := &DirDiffConfig{
		Recursive: true,
	}

	entries, err := WalkDirectory(tmpDir, config)
	if err != nil {
		t.Fatalf("WalkDirectory() error = %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	if _, ok := entries["file1.txt"]; !ok {
		t.Error("Expected file1.txt in entries")
	}
	if _, ok := entries["subdir/file3.txt"]; !ok {
		t.Error("Expected subdir/file3.txt in entries")
	}
}

func TestWalkDirectoryNonRecursive(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir/file2.txt"), []byte("content2"), 0644)

	config := &DirDiffConfig{
		Recursive: false,
	}

	entries, err := WalkDirectory(tmpDir, config)
	if err != nil {
		t.Fatalf("WalkDirectory() error = %v", err)
	}

	// 只应该包含根目录文件
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	if _, ok := entries["file1.txt"]; !ok {
		t.Error("Expected file1.txt in entries")
	}
}

func TestWalkDirectoryIgnoreHidden(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建测试文件，包括隐藏文件
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, ".hidden_dir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".hidden_dir/file2.txt"), []byte("content2"), 0644)

	config := &DirDiffConfig{
		Recursive:    true,
		IgnoreHidden: true,
	}

	entries, err := WalkDirectory(tmpDir, config)
	if err != nil {
		t.Fatalf("WalkDirectory() error = %v", err)
	}

	// 不应该包含隐藏文件
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	if _, ok := entries[".hidden"]; ok {
		t.Error("Expected .hidden to be ignored")
	}
}

func TestWalkDirectoryIgnorePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建测试文件
	files := []string{
		"file1.txt",
		"readme.txt",
	}

	for _, file := range files {
		os.WriteFile(filepath.Join(tmpDir, file), []byte("content"), 0644)
	}

	config := &DirDiffConfig{
		Recursive:      true,
		IgnorePatterns: []string{"readme"},
	}

	entries, err := WalkDirectory(tmpDir, config)
	if err != nil {
		t.Fatalf("WalkDirectory() error = %v", err)
	}

	// 只应该包含file1.txt
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	if _, ok := entries["file1.txt"]; !ok {
		t.Error("Expected file1.txt in entries")
	}
}

func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		path     string
		patterns []string
		expected bool
	}{
		{"file.txt", []string{".git"}, false},
		{".git", []string{".git"}, true},
		{"sub/file.txt", []string{"sub"}, true},
		{"other/file.txt", []string{"sub"}, false},
		{"readme.txt", []string{"readme"}, true},
		{"data.csv", []string{"data"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := shouldIgnore(tt.path, tt.patterns)
			if result != tt.expected {
				t.Errorf("shouldIgnore(%v, %v) = %v, want %v", tt.path, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestCompareDirectories(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()

	// 创建旧目录文件
	os.WriteFile(filepath.Join(oldDir, "unchanged.txt"), []byte("same content"), 0644)
	os.WriteFile(filepath.Join(oldDir, "modified.txt"), []byte("old content"), 0644)
	os.WriteFile(filepath.Join(oldDir, "deleted.txt"), []byte("to be deleted"), 0644)

	// 创建新目录文件
	os.WriteFile(filepath.Join(newDir, "unchanged.txt"), []byte("same content"), 0644)
	os.WriteFile(filepath.Join(newDir, "modified.txt"), []byte("new content"), 0644)
	os.WriteFile(filepath.Join(newDir, "added.txt"), []byte("new file"), 0644)

	config := &DirDiffConfig{
		Recursive: true,
	}

	result, err := CompareDirectories(oldDir, newDir, config)
	if err != nil {
		t.Fatalf("CompareDirectories() error = %v", err)
	}

	// 验证添加的文件
	found := false
	for _, f := range result.AddedFiles {
		if f.RelativePath == "added.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected added.txt in AddedFiles")
	}

	// 验证删除的文件
	found = false
	for _, f := range result.DeletedFiles {
		if f.RelativePath == "deleted.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected deleted.txt in DeletedFiles")
	}

	// 验证修改的文件 - 文件大小不同会被标记为修改
	found = false
	for _, f := range result.ModifiedFiles {
		if f.RelativePath == "modified.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected modified.txt in ModifiedFiles")
	}
}

func TestCompareDirectoriesEmpty(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()

	config := &DirDiffConfig{
		Recursive: true,
	}

	result, err := CompareDirectories(oldDir, newDir, config)
	if err != nil {
		t.Fatalf("CompareDirectories() error = %v", err)
	}

	if len(result.AddedFiles) != 0 {
		t.Errorf("AddedFiles count = %d, want 0", len(result.AddedFiles))
	}
	if len(result.DeletedFiles) != 0 {
		t.Errorf("DeletedFiles count = %d, want 0", len(result.DeletedFiles))
	}
	if len(result.ModifiedFiles) != 0 {
		t.Errorf("ModifiedFiles count = %d, want 0", len(result.ModifiedFiles))
	}
	if len(result.UnchangedFiles) != 0 {
		t.Errorf("UnchangedFiles count = %d, want 0", len(result.UnchangedFiles))
	}
}

func TestCompareDirectoriesNested(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()

	// 创建嵌套目录结构
	os.MkdirAll(filepath.Join(oldDir, "sub1", "sub2"), 0755)
	os.MkdirAll(filepath.Join(newDir, "sub1", "sub2"), 0755)

	os.WriteFile(filepath.Join(oldDir, "sub1", "file1.txt"), []byte("old"), 0644)
	os.WriteFile(filepath.Join(oldDir, "sub1", "sub2", "file2.txt"), []byte("old"), 0644)

	os.WriteFile(filepath.Join(newDir, "sub1", "file1.txt"), []byte("new"), 0644)
	os.WriteFile(filepath.Join(newDir, "sub1", "sub2", "file2.txt"), []byte("new"), 0644)
	os.WriteFile(filepath.Join(newDir, "sub1", "sub2", "file3.txt"), []byte("new"), 0644)

	config := &DirDiffConfig{
		Recursive: true,
	}

	result, err := CompareDirectories(oldDir, newDir, config)
	if err != nil {
		t.Fatalf("CompareDirectories() error = %v", err)
	}

	if len(result.ModifiedFiles)+len(result.AddedFiles) != 3 {
		t.Errorf("Expected 3 changed files, got %d", len(result.ModifiedFiles)+len(result.AddedFiles))
	}
}

func TestProcessDirDiff(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()

	// 创建测试文件
	os.WriteFile(filepath.Join(oldDir, "file1.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(newDir, "file1.txt"), []byte("hello go"), 0644)
	os.WriteFile(filepath.Join(newDir, "file2.txt"), []byte("new file"), 0644)

	config := &DirDiffConfig{
		Recursive:   true,
		WorkerCount: 2,
		BlockSize:   4096,
	}

	result, err := CompareDirectories(oldDir, newDir, config)
	if err != nil {
		t.Fatalf("CompareDirectories() error = %v", err)
	}

	diffEngine, err := NewEngine(DefaultDiffConfig())
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	err = ProcessDirDiff(result, diffEngine, config, nil)
	if err != nil {
		t.Fatalf("ProcessDirDiff() error = %v", err)
	}

	// 验证ModifiedFiles有Delta
	if len(result.ModifiedFiles) > 0 {
		if result.ModifiedFiles[0].Delta == nil {
			t.Error("Modified file should have Delta")
		}
	}

	// 验证AddedFiles有PatchData
	if len(result.AddedFiles) > 0 {
		if result.AddedFiles[0].PatchData == nil {
			t.Error("Added file should have PatchData")
		}
		if string(result.AddedFiles[0].PatchData) != "new file" {
			t.Error("Added file PatchData should match content")
		}
	}
}
