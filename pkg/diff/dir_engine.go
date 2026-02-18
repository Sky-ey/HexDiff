package diff

import (
	"os"
	"path/filepath"
)

type DirEngine struct {
	config    *DiffConfig
	dirConfig *DirDiffConfig
}

func NewDirEngine(config *DiffConfig, dirConfig *DirDiffConfig) (*DirEngine, error) {
	if config == nil {
		config = DefaultDiffConfig()
	}
	if dirConfig == nil {
		dirConfig = DefaultDirDiffConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}
	if err := dirConfig.Validate(); err != nil {
		return nil, err
	}

	return &DirEngine{
		config:    config,
		dirConfig: dirConfig,
	}, nil
}

func (e *DirEngine) GenerateDirDiff(oldDir, newDir string, progress ProgressReporter) (*DirDiffResult, error) {
	oldDir = filepath.Clean(oldDir)
	newDir = filepath.Clean(newDir)

	if _, err := os.Stat(oldDir); err != nil {
		return nil, NewDiffError("stat old directory", oldDir, err)
	}
	if _, err := os.Stat(newDir); err != nil {
		return nil, NewDiffError("stat new directory", newDir, err)
	}

	if progress != nil {
		progress.Message("正在扫描目录...")
	}

	result, err := CompareDirectories(oldDir, newDir, e.dirConfig)
	if err != nil {
		return nil, err
	}

	if progress != nil {
		progress.Message("正在生成补丁...")
	}

	diffEngine, err := NewEngine(e.config)
	if err != nil {
		return nil, err
	}

	err = ProcessDirDiff(result, diffEngine, e.dirConfig, progress)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (e *DirEngine) GetConfig() *DiffConfig {
	return e.config
}

func (e *DirEngine) GetDirConfig() *DirDiffConfig {
	return e.dirConfig
}
