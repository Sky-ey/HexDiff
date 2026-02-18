package diff

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDirEngine(t *testing.T) {
	tests := []struct {
		name      string
		config    *DiffConfig
		dirConfig *DirDiffConfig
		wantErr   bool
	}{
		{
			name:      "nil configs",
			config:    nil,
			dirConfig: nil,
			wantErr:   false,
		},
		{
			name:   "valid config",
			config: DefaultDiffConfig(),
			dirConfig: &DirDiffConfig{
				Recursive:   true,
				WorkerCount: 2,
			},
			wantErr: false,
		},
		{
			name:   "valid dir config",
			config: DefaultDiffConfig(),
			dirConfig: &DirDiffConfig{
				Recursive:   true,
				WorkerCount: 2,
			},
			wantErr: false,
		},
		{
			name: "invalid diff config - block size too small",
			config: &DiffConfig{
				BlockSize:  32,
				WindowSize: 64,
			},
			dirConfig: nil,
			wantErr:   true,
		},
		{
			name:   "invalid dir config - worker count too high",
			config: nil,
			dirConfig: &DirDiffConfig{
				WorkerCount: 33,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewDirEngine(tt.config, tt.dirConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDirEngine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && engine == nil {
				t.Error("NewDirEngine() returned nil engine")
			}
		})
	}
}

func TestDirEngineGenerateDirDiff(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()

	// 创建测试文件 - 使用不同大小的内容确保被检测为修改
	os.WriteFile(filepath.Join(oldDir, "file1.txt"), []byte("old content"), 0644)
	os.WriteFile(filepath.Join(oldDir, "file2.txt"), []byte("same content here"), 0644)
	os.WriteFile(filepath.Join(newDir, "file1.txt"), []byte("new content"), 0644)
	os.WriteFile(filepath.Join(newDir, "file2.txt"), []byte("same content here"), 0644)
	os.WriteFile(filepath.Join(newDir, "file3.txt"), []byte("new file"), 0644)

	engine, err := NewDirEngine(nil, nil)
	if err != nil {
		t.Fatalf("NewDirEngine() error = %v", err)
	}

	result, err := engine.GenerateDirDiff(oldDir, newDir, nil)
	if err != nil {
		t.Fatalf("GenerateDirDiff() error = %v", err)
	}

	if result == nil {
		t.Fatal("GenerateDirDiff() returned nil result")
	}

	// 验证有文件添加和修改
	if len(result.AddedFiles)+len(result.ModifiedFiles) < 2 {
		t.Errorf("Expected at least 2 changed files, got %d", len(result.AddedFiles)+len(result.ModifiedFiles))
	}
}

func TestDirEngineGenerateDirDiffWithSubdirs(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()

	// 创建嵌套目录结构
	os.MkdirAll(filepath.Join(oldDir, "sub1", "sub2"), 0755)
	os.MkdirAll(filepath.Join(newDir, "sub1", "sub2"), 0755)

	os.WriteFile(filepath.Join(oldDir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(oldDir, "sub1", "file1.txt"), []byte("old"), 0644)
	os.WriteFile(filepath.Join(oldDir, "sub1", "sub2", "file2.txt"), []byte("old"), 0644)

	os.WriteFile(filepath.Join(newDir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(newDir, "sub1", "file1.txt"), []byte("new"), 0644)
	os.WriteFile(filepath.Join(newDir, "sub1", "sub2", "file2.txt"), []byte("newer"), 0644)
	os.WriteFile(filepath.Join(newDir, "sub1", "sub2", "file3.txt"), []byte("new"), 0644)

	engine, err := NewDirEngine(nil, &DirDiffConfig{
		Recursive:   true,
		WorkerCount: 4,
	})
	if err != nil {
		t.Fatalf("NewDirEngine() error = %v", err)
	}

	result, err := engine.GenerateDirDiff(oldDir, newDir, nil)
	if err != nil {
		t.Fatalf("GenerateDirDiff() error = %v", err)
	}

	// 应该有修改的文件和新增的文件
	totalChanged := len(result.AddedFiles) + len(result.ModifiedFiles)
	if totalChanged == 0 {
		t.Error("Expected at least one changed file")
	}
}

func TestDirEngineGenerateDirDiffEmptyDirs(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()

	engine, err := NewDirEngine(nil, nil)
	if err != nil {
		t.Fatalf("NewDirEngine() error = %v", err)
	}

	result, err := engine.GenerateDirDiff(oldDir, newDir, nil)
	if err != nil {
		t.Fatalf("GenerateDirDiff() error = %v", err)
	}

	if len(result.AddedFiles) != 0 || len(result.DeletedFiles) != 0 || len(result.ModifiedFiles) != 0 {
		t.Error("Expected no changes for empty directories")
	}
}

func TestDirEngineGenerateDirDiffInvalidDir(t *testing.T) {
	engine, err := NewDirEngine(nil, nil)
	if err != nil {
		t.Fatalf("NewDirEngine() error = %v", err)
	}

	_, err = engine.GenerateDirDiff("/nonexistent_dir", "/tmp", nil)
	if err == nil {
		t.Error("Expected error for nonexistent old directory")
	}

	_, err = engine.GenerateDirDiff("/tmp", "/nonexistent_dir", nil)
	if err == nil {
		t.Error("Expected error for nonexistent new directory")
	}
}

func TestDirEngineGetConfig(t *testing.T) {
	diffConfig := &DiffConfig{
		BlockSize:  8192,
		WindowSize: 128,
		MaxMemory:  100 * 1024 * 1024,
	}

	engine, err := NewDirEngine(diffConfig, nil)
	if err != nil {
		t.Fatalf("NewDirEngine() error = %v", err)
	}

	config := engine.GetConfig()
	if config.BlockSize != 8192 {
		t.Errorf("BlockSize = %d, want 8192", config.BlockSize)
	}
}

func TestDirEngineGetDirConfig(t *testing.T) {
	dirConfig := &DirDiffConfig{
		Recursive:    false,
		WorkerCount:  8,
		IgnoreHidden: true,
	}

	engine, err := NewDirEngine(nil, dirConfig)
	if err != nil {
		t.Fatalf("NewDirEngine() error = %v", err)
	}

	config := engine.GetDirConfig()
	if config.Recursive {
		t.Error("Recursive should be false")
	}
	if config.WorkerCount != 8 {
		t.Errorf("WorkerCount = %d, want 8", config.WorkerCount)
	}
	if !config.IgnoreHidden {
		t.Error("IgnoreHidden should be true")
	}
}
