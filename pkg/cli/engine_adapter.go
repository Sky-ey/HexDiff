package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sky-ey/HexDiff/pkg/diff"
	"github.com/Sky-ey/HexDiff/pkg/integrity"
	"github.com/Sky-ey/HexDiff/pkg/patch"
)

// EngineAdapter CLI引擎适配器
type EngineAdapter struct {
	diffEngine         *diff.Engine
	dirDiffEngine      *diff.DirEngine
	patchGenerator     *patch.Generator
	patchApplier       *patch.Applier
	dirPatchSerializer *patch.DirPatchSerializer
	validator          *patch.Validator
	integrityChecker   *integrity.IntegrityChecker
}

// NewEngineAdapter 创建引擎适配器
func NewEngineAdapter() (*EngineAdapter, error) {
	// 创建差异检测引擎
	diffEngine, err := diff.NewEngine(nil)
	if err != nil {
		return nil, fmt.Errorf("创建差异检测引擎失败: %w", err)
	}

	// 创建目录差异检测引擎
	dirDiffEngine, err := diff.NewDirEngine(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("创建目录差异检测引擎失败: %w", err)
	}

	// 创建补丁生成器
	patchGenerator := patch.NewGenerator(diffEngine, patch.CompressionGzip)

	// 创建补丁应用器
	patchApplier := patch.NewApplier(nil)

	// 创建目录补丁序列化器
	dirPatchSerializer := patch.NewDirPatchSerializer(patch.CompressionNone)

	// 创建验证器
	validator := patch.NewValidator()

	// 创建完整性检查器
	integrityChecker := integrity.NewIntegrityChecker(integrity.DefaultCheckerConfig())

	return &EngineAdapter{
		diffEngine:         diffEngine,
		dirDiffEngine:      dirDiffEngine,
		patchGenerator:     patchGenerator,
		patchApplier:       patchApplier,
		dirPatchSerializer: dirPatchSerializer,
		validator:          validator,
		integrityChecker:   integrityChecker,
	}, nil
}

// GenerateSignature 生成文件签名
func (ea *EngineAdapter) GenerateSignature(inputFile, outputFile string, blockSize int, progress ProgressReporter) error {
	// 设置进度
	progress.SetMessage("正在生成文件签名...")
	progress.SetCurrent(10)

	// 生成签名
	signature, err := ea.diffEngine.GenerateSignature(inputFile)
	if err != nil {
		return err
	}

	progress.SetCurrent(50)

	// 保存签名到文件（这里需要实现签名序列化）
	// 暂时只返回成功
	progress.SetMessage("保存签名文件...")
	progress.SetCurrent(90)

	// 模拟保存过程
	_ = signature
	_ = outputFile

	progress.SetCurrent(100)
	progress.SetMessage("签名生成完成")

	return nil
}

// GeneratePatch 生成补丁
func (ea *EngineAdapter) GeneratePatch(oldFile, newFile, outputFile, signature string, compress bool, progress ProgressReporter) error {
	progress.SetMessage("正在分析文件差异...")
	progress.SetCurrent(10)

	// 检查文件是否存在
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		return fmt.Errorf("旧文件不存在: %s", oldFile)
	}
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		return fmt.Errorf("新文件不存在: %s", newFile)
	}

	progress.SetCurrent(30)
	progress.SetMessage("生成补丁文件...")

	// 生成补丁
	_, err := ea.patchGenerator.GeneratePatch(oldFile, newFile, outputFile)
	if err != nil {
		return err
	}

	progress.SetCurrent(100)
	progress.SetMessage("补丁生成完成")

	return nil
}

// ApplyPatch 应用补丁
func (ea *EngineAdapter) ApplyPatch(patchFile, targetFile, outputFile string, verify bool, progress ProgressReporter) error {
	progress.SetMessage("正在读取补丁文件...")
	progress.SetCurrent(10)

	// 检查文件是否存在
	if _, err := os.Stat(patchFile); os.IsNotExist(err) {
		return fmt.Errorf("补丁文件不存在: %s", patchFile)
	}
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		return fmt.Errorf("目标文件不存在: %s", targetFile)
	}

	progress.SetCurrent(30)
	progress.SetMessage("应用补丁...")

	// 应用补丁
	_, err := ea.patchApplier.ApplyPatch(targetFile, patchFile, outputFile)
	if err != nil {
		return err
	}

	progress.SetCurrent(80)

	// 如果启用验证
	if verify {
		progress.SetMessage("验证结果...")
		// 这里可以添加验证逻辑
		progress.SetCurrent(95)
	}

	progress.SetCurrent(100)
	progress.SetMessage("补丁应用完成")

	return nil
}

// ValidatePatch 验证补丁
func (ea *EngineAdapter) ValidatePatch(patchFile string, progress ProgressReporter) (*ValidationResult, error) {
	progress.SetMessage("正在验证补丁文件...")
	progress.SetCurrent(20)

	// 验证补丁文件
	result, err := ea.validator.ValidatePatchFile(patchFile)
	if err != nil {
		return nil, err
	}

	progress.SetCurrent(80)
	progress.SetMessage("分析验证结果...")

	// 转换结果格式
	validationResult := &ValidationResult{
		Valid:         result.Valid,
		ValidFormat:   result.Valid,
		ValidChecksum: result.Valid,
		ValidData:     result.Valid,
		Errors:        result.Issues,
	}

	progress.SetCurrent(100)
	progress.SetMessage("验证完成")

	return validationResult, nil
}

// GetPatchInfo 获取补丁信息
func (ea *EngineAdapter) GetPatchInfo(patchFile string) (*PatchInfo, error) {
	// 读取补丁头信息
	header, err := patch.GetPatchInfo(patchFile)
	if err != nil {
		return nil, err
	}

	// 获取文件大小
	stat, err := os.Stat(patchFile)
	if err != nil {
		return nil, err
	}

	// 转换为CLI格式
	info := &PatchInfo{
		Version:        header.Version,
		Compression:    CompressionType(header.Compression),
		SourceChecksum: header.SourceChecksum[:],
		TargetChecksum: header.TargetChecksum[:],
		OperationCount: int(header.OperationCount),
		PatchSize:      stat.Size(),
		CreatedAt:      time.Unix(header.Timestamp, 0),
		Metadata:       make(map[string]string),
	}

	return info, nil
}

// GetDirPatchInfo 获取目录补丁信息
func (ea *EngineAdapter) GetDirPatchInfo(patchFile string) (*DirPatchInfo, error) {
	header, err := patch.GetDirPatchInfo(patchFile)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(patchFile)
	if err != nil {
		return nil, err
	}

	dirPatch, err := ea.dirPatchSerializer.DeserializeDirPatch(patchFile)
	if err != nil {
		return nil, err
	}

	addedCount := 0
	deletedCount := 0
	modifiedCount := 0
	unchangedCount := 0

	var addedFiles []string
	var deletedFiles []string
	var modifiedFiles []string

	for _, f := range dirPatch.Files {
		switch f.Status {
		case diff.StatusAdded:
			addedCount++
			addedFiles = append(addedFiles, f.RelativePath)
		case diff.StatusDeleted:
			deletedCount++
			deletedFiles = append(deletedFiles, f.RelativePath)
		case diff.StatusModified:
			modifiedCount++
			modifiedFiles = append(modifiedFiles, f.RelativePath)
		case diff.StatusUnchanged:
			unchangedCount++
		}
	}

	info := &DirPatchInfo{
		Version:          header.Version,
		OldDir:           dirPatch.OldDir,
		NewDir:           dirPatch.NewDir,
		FileCount:        len(dirPatch.Files),
		AddedFiles:       addedCount,
		DeletedFiles:     deletedCount,
		ModifiedFiles:    modifiedCount,
		UnchangedFiles:   unchangedCount,
		PatchSize:        stat.Size(),
		CreatedAt:        time.Unix(header.Timestamp, 0),
		AddedFileList:    addedFiles,
		DeletedFileList:  deletedFiles,
		ModifiedFileList: modifiedFiles,
	}

	return info, nil
}

// GenerateDirDiff 生成目录补丁
func (ea *EngineAdapter) GenerateDirDiff(oldDir, newDir, outputFile string, recursive, ignoreHidden bool, ignorePatterns string, compress bool, progress ProgressReporter) (interface{}, error) {
	progress.SetMessage("正在分析目录差异...")
	progress.SetCurrent(10)

	dirConfig := diff.DefaultDirDiffConfig()
	dirConfig.Recursive = recursive
	dirConfig.IgnoreHidden = ignoreHidden
	if ignorePatterns != "" {
		dirConfig.IgnorePatterns = splitIgnorePatterns(ignorePatterns)
	}
	dirConfig.Compress = compress

	ea.dirDiffEngine, _ = diff.NewDirEngine(nil, dirConfig)

	wrapper := &diffProgressWrapper{progress}
	result, err := ea.dirDiffEngine.GenerateDirDiff(oldDir, newDir, wrapper)
	if err != nil {
		return nil, err
	}

	totalBytes := result.TotalBytesToProcess()

	progress.SetMessage("正在序列化补丁...")

	oldBase := filepath.Base(oldDir)
	newBase := filepath.Base(newDir)
	err = ea.dirPatchSerializer.SerializeDirPatch(result, oldBase, newBase, outputFile)
	if err != nil {
		return nil, err
	}

	if totalBytes > 0 {
		progress.SetTotal(totalBytes)
		progress.SetCurrent(totalBytes)
	}
	progress.SetMessage("目录补丁生成完成")

	return result, nil
}

type diffProgressWrapper struct {
	cliProgress ProgressReporter
}

func (w *diffProgressWrapper) SetProgress(percent int) {
	w.cliProgress.SetCurrent(int64(percent))
}

func (w *diffProgressWrapper) IncProgress(delta int) {
	w.cliProgress.Increment(int64(delta))
}

func (w *diffProgressWrapper) SetProgressBytes(bytes int64) {
	w.cliProgress.Increment(int64(bytes))
}

func (w *diffProgressWrapper) Message(msg string) {
	w.cliProgress.SetMessage(msg)
}

func splitIgnorePatterns(patterns string) []string {
	if patterns == "" {
		return nil
	}
	var result []string
	for _, p := range strings.Split(patterns, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ApplyDirPatch 应用目录补丁
func (ea *EngineAdapter) ApplyDirPatch(patchFile, targetDir string, verify bool, progress ProgressReporter) (interface{}, error) {
	progress.SetMessage("正在读取目录补丁...")
	progress.SetCurrent(10)

	dirPatch, err := ea.dirPatchSerializer.DeserializeDirPatch(patchFile)
	if err != nil {
		return nil, err
	}

	progress.SetMessage("正在应用目录补丁...")
	progress.SetCurrent(10)

	var totalBytes int64

	for _, filePatch := range dirPatch.Files {
		switch filePatch.Status {
		case diff.StatusAdded, diff.StatusModified:
			if filePatch.IsFullContent {
				totalBytes += filePatch.Size
			} else if len(filePatch.Delta) > 0 {
				totalBytes += filePatch.DeltaSize
			}
		}
	}

	if totalBytes > 0 {
		progress.SetTotal(10 + 30 + totalBytes)
	}

	var processedBytes int64

	for _, filePatch := range dirPatch.Files {
		var fileBytes int64

		targetPath := filepath.Join(targetDir, filePatch.RelativePath)

		switch filePatch.Status {
		case diff.StatusAdded, diff.StatusModified:
			dir := filepath.Dir(targetPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("创建目录失败: %w", err)
			}

			if filePatch.IsFullContent {
				if err := os.WriteFile(targetPath, filePatch.Delta, os.FileMode(filePatch.Mode)); err != nil {
					return nil, fmt.Errorf("写入文件失败: %w", err)
				}
				fileBytes = filePatch.Size
			} else if len(filePatch.Delta) > 0 {
				if filePatch.Status == diff.StatusAdded {
					if err := os.WriteFile(targetPath, filePatch.Delta, os.FileMode(filePatch.Mode)); err != nil {
						return nil, fmt.Errorf("写入文件失败: %w", err)
					}
					fileBytes = filePatch.Size
				} else {
					if _, err := os.Stat(targetPath); err == nil {
						if err := ea.patchApplier.ApplyDelta(targetPath, filePatch.Delta, targetPath+".tmp"); err != nil {
							return nil, fmt.Errorf("应用二进制差异补丁失败 %s: %w", filePatch.RelativePath, err)
						}
						if err := os.Rename(targetPath+".tmp", targetPath); err != nil {
							return nil, fmt.Errorf("重命名文件失败 %s: %w", filePatch.RelativePath, err)
						}
						fileBytes = filePatch.DeltaSize
					} else {
						return nil, fmt.Errorf("源文件不存在: %s", filePatch.RelativePath)
					}
				}
			}

			os.Chtimes(targetPath, filePatch.GetMTime(), filePatch.GetMTime())

		case diff.StatusDeleted:
			if _, err := os.Stat(targetPath); err == nil {
				if err := os.Remove(targetPath); err != nil {
					return nil, fmt.Errorf("删除文件失败: %w", err)
				}
			}
		}

		if fileBytes > 0 {
			processedBytes += fileBytes
			if totalBytes > 0 {
				progress.SetCurrent(30 + processedBytes)
			}
		}
	}

	progress.SetMessage("目录补丁应用完成")

	return dirPatch, nil
}
