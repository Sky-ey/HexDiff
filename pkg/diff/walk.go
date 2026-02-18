package diff

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// WalkDirectory 遍历目录获取文件列表
func WalkDirectory(dirPath string, config *DirDiffConfig) (map[string]*FileEntry, error) {
	entries := make(map[string]*FileEntry)

	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, NewDiffError("abs path", dirPath, err)
	}

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(absDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		if config.IgnoreHidden && strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if shouldIgnore(relPath, config.IgnorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !config.Recursive && info.IsDir() && relPath != "." {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		if config.FollowSymlinks {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		entry := &FileEntry{
			Path:         relPath,
			RelativePath: filepath.ToSlash(relPath),
			AbsPath:      path,
			Size:         info.Size(),
			Mode:         info.Mode(),
			MTime:        info.ModTime(),
			IsDir:        info.IsDir(),
		}

		entries[filepath.ToSlash(relPath)] = entry
		return nil
	})

	if err != nil {
		return nil, NewDiffError("walk directory", dirPath, err)
	}

	return entries, nil
}

// shouldIgnore 检查路径是否应该被忽略
func shouldIgnore(path string, patterns []string) bool {
	base := filepath.Base(path)

	for _, pattern := range patterns {
		pattern = strings.TrimPrefix(pattern, "*")

		if strings.HasPrefix(pattern, "*.") {
			ext := strings.TrimPrefix(pattern, "*.")
			if strings.HasSuffix(base, "."+ext) || base == "."+ext {
				return true
			}
		}

		if base == pattern || strings.HasPrefix(path, pattern+"/") || path == pattern {
			return true
		}
	}

	return false
}

// CompareDirectories 比较两个目录返回差异结果
func CompareDirectories(oldDir, newDir string, config *DirDiffConfig) (*DirDiffResult, error) {
	if config == nil {
		config = DefaultDirDiffConfig()
	}

	oldDir = filepath.Clean(oldDir)
	newDir = filepath.Clean(newDir)

	oldEntries, err := WalkDirectory(oldDir, config)
	if err != nil {
		return nil, err
	}

	newEntries, err := WalkDirectory(newDir, config)
	if err != nil {
		return nil, err
	}

	result := NewDirDiffResult(oldDir, newDir)

	allPaths := make(map[string]bool)
	for path := range oldEntries {
		allPaths[path] = true
	}
	for path := range newEntries {
		allPaths[path] = true
	}

	for path := range allPaths {
		oldEntry, oldExists := oldEntries[path]
		newEntry, newExists := newEntries[path]

		var fileDiff *FileDiff

		if !oldExists && newExists {
			fileDiff = &FileDiff{
				RelativePath: path,
				Status:       StatusAdded,
				NewEntry:     newEntry,
			}
		} else if oldExists && !newExists {
			fileDiff = &FileDiff{
				RelativePath: path,
				Status:       StatusDeleted,
				OldEntry:     oldEntry,
			}
		} else if oldExists && newExists {
			if oldEntry.Size == newEntry.Size && oldEntry.MTime.Equal(newEntry.MTime) {
				continue
			}

			if oldEntry.Size != newEntry.Size {
				hashOld, err := computeFileHash(oldEntry.AbsPath)
				if err != nil {
					continue
				}
				hashNew, err := computeFileHash(newEntry.AbsPath)
				if err != nil {
					continue
				}

				if bytes.Equal(hashOld, hashNew) {
					continue
				}
			}

			fileDiff = &FileDiff{
				RelativePath: path,
				Status:       StatusModified,
				OldEntry:     oldEntry,
				NewEntry:     newEntry,
			}
		}

		if fileDiff != nil {
			result.AddFileDiff(fileDiff)
		}
	}

	return result, nil
}

// computeFileHash 计算文件SHA-256校验和
func computeFileHash(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return nil, err
	}

	return hasher.Sum(nil), nil
}

// ProcessDirDiff 处理目录差异，为修改的文件生成补丁
func ProcessDirDiff(result *DirDiffResult, diffEngine *Engine, config *DirDiffConfig, progress ProgressReporter) error {
	var wg sync.WaitGroup
	fileChan := make(chan *FileDiff, config.WorkerCount*2)
	errChan := make(chan error, 1)
	doneChan := make(chan struct{})

	go func() {
		for diff := range fileChan {
			if diff.Status == StatusModified {
				delta, err := diffEngine.GenerateDelta(diff.OldEntry.AbsPath, diff.NewEntry.AbsPath)
				if err != nil {
					errChan <- fmt.Errorf("generate delta for %s: %w", diff.RelativePath, err)
					continue
				}
				diff.Delta = delta
			} else if diff.Status == StatusAdded {
				data, err := os.ReadFile(diff.NewEntry.AbsPath)
				if err != nil {
					errChan <- fmt.Errorf("read new file %s: %w", diff.RelativePath, err)
					continue
				}
				diff.PatchData = data
			}
			wg.Done()
		}
		close(doneChan)
	}()

	for _, diff := range result.ModifiedFiles {
		wg.Add(1)
		fileChan <- diff
	}

	close(fileChan)
	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
	}

	if progress != nil {
		progress.SetProgress(100)
	}

	return nil
}

// ProgressReporter 进度报告接口
type ProgressReporter interface {
	SetProgress(percent int)
	IncProgress(delta int)
	Message(msg string)
}
