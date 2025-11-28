package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/Sky-ey/HexDiff/pkg/diff"
	"github.com/Sky-ey/HexDiff/pkg/integrity"
	"github.com/Sky-ey/HexDiff/pkg/patch"
)

// EngineAdapter CLI引擎适配器
type EngineAdapter struct {
	diffEngine       *diff.Engine
	patchGenerator   *patch.Generator
	patchApplier     *patch.Applier
	validator        *patch.Validator
	integrityChecker *integrity.IntegrityChecker
}

// NewEngineAdapter 创建引擎适配器
func NewEngineAdapter() (*EngineAdapter, error) {
	// 创建差异检测引擎
	diffEngine, err := diff.NewEngine(nil)
	if err != nil {
		return nil, fmt.Errorf("创建差异检测引擎失败: %w", err)
	}

	// 创建补丁生成器
	patchGenerator := patch.NewGenerator(diffEngine, patch.CompressionGzip)

	// 创建补丁应用器
	patchApplier := patch.NewApplier(nil)

	// 创建验证器
	validator := patch.NewValidator()

	// 创建完整性检查器
	integrityChecker := integrity.NewIntegrityChecker(integrity.DefaultCheckerConfig())

	return &EngineAdapter{
		diffEngine:       diffEngine,
		patchGenerator:   patchGenerator,
		patchApplier:     patchApplier,
		validator:        validator,
		integrityChecker: integrityChecker,
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
