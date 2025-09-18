package integrity

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// EnhancedApplier 增强的补丁应用器（集成完整性校验）
type EnhancedApplier struct {
	checker             *IntegrityChecker
	recoveryManager     *RecoveryManager
	realtimeVerifier    *RealtimeVerifier
	progressiveVerifier *ProgressiveVerifier
	config              *EnhancedApplierConfig
	stats               *ApplicationStats
}

// EnhancedApplierConfig 增强应用器配置
type EnhancedApplierConfig struct {
	BufferSize        int           // 缓冲区大小
	TempDir           string        // 临时目录
	BackupEnabled     bool          // 是否启用备份
	VerifyTarget      bool          // 是否验证目标文件
	EnableIntegrity   bool          // 是否启用完整性检查
	EnableRealtime    bool          // 是否启用实时验证
	EnableRecovery    bool          // 是否启用恢复功能
	EnableProgressive bool          // 是否启用渐进式验证
	BlockSize         int           // 完整性检查块大小
	MaxRetries        int           // 最大重试次数
	RetryDelay        time.Duration // 重试延迟
}

// ApplicationStats 应用统计信息
type ApplicationStats struct {
	StartTime         time.Time     // 开始时间
	EndTime           time.Time     // 结束时间
	Duration          time.Duration // 总耗时
	TotalOperations   int           // 总操作数
	SuccessOperations int           // 成功操作数
	FailedOperations  int           // 失败操作数
	BytesProcessed    int64         // 处理字节数
	VerificationTime  time.Duration // 验证耗时
	BackupTime        time.Duration // 备份耗时
	RecoveryAttempts  int           // 恢复尝试次数
	ErrorsEncountered []error       // 遇到的错误
}

// DefaultEnhancedApplierConfig 默认增强应用器配置
func DefaultEnhancedApplierConfig() *EnhancedApplierConfig {
	return &EnhancedApplierConfig{
		BufferSize:        64 * 1024, // 64KB
		TempDir:           os.TempDir(),
		BackupEnabled:     true,
		VerifyTarget:      true,
		EnableIntegrity:   true,
		EnableRealtime:    true,
		EnableRecovery:    true,
		EnableProgressive: true,
		BlockSize:         64 * 1024, // 64KB
		MaxRetries:        3,
		RetryDelay:        time.Second,
	}
}

// NewEnhancedApplier 创建新的增强补丁应用器
func NewEnhancedApplier(config *EnhancedApplierConfig) *EnhancedApplier {
	if config == nil {
		config = DefaultEnhancedApplierConfig()
	}

	applier := &EnhancedApplier{
		config: config,
		stats: &ApplicationStats{
			ErrorsEncountered: make([]error, 0),
		},
	}

	// 初始化完整性检查器
	if config.EnableIntegrity {
		checkerConfig := &CheckerConfig{
			BlockSize:    config.BlockSize,
			EnableSHA256: true,
			EnableCRC32:  true,
			ErrorCallback: func(err error) {
				applier.handleError(err)
			},
		}
		applier.checker = NewIntegrityChecker(checkerConfig)
	}

	// 初始化恢复管理器
	if config.EnableRecovery && applier.checker != nil {
		recoveryConfig := &RecoveryConfig{
			BackupDir:  filepath.Join(config.TempDir, ".hexdiff_backups"),
			MaxBackups: 5,
			ErrorHandler: func(err error) {
				applier.handleError(err)
			},
		}
		applier.recoveryManager = NewRecoveryManager(applier.checker, recoveryConfig)
	}

	// 初始化实时验证器
	if config.EnableRealtime && applier.checker != nil {
		applier.realtimeVerifier = NewRealtimeVerifier(applier.checker, config.BlockSize)
		applier.realtimeVerifier.SetErrorCallback(func(err error) {
			applier.handleError(err)
		})
	}

	return applier
}

// ApplyPatchWithIntegrity 应用补丁并进行完整性验证
func (ea *EnhancedApplier) ApplyPatchWithIntegrity(sourceFilePath, patchFilePath, targetFilePath string, patchData interface{}) (*EnhancedApplyResult, error) {
	ea.stats.StartTime = time.Now()
	defer func() {
		ea.stats.EndTime = time.Now()
		ea.stats.Duration = ea.stats.EndTime.Sub(ea.stats.StartTime)
	}()

	// 第一步：预验证和备份
	if err := ea.preValidationAndBackup(sourceFilePath, targetFilePath); err != nil {
		return nil, fmt.Errorf("预验证和备份失败: %w", err)
	}

	// 第二步：生成源文件完整性校验和
	if ea.config.EnableIntegrity {
		verifyStart := time.Now()
		if err := ea.checker.GenerateFileChecksums(sourceFilePath); err != nil {
			return nil, fmt.Errorf("生成源文件校验和失败: %w", err)
		}
		ea.stats.VerificationTime += time.Since(verifyStart)
	}

	// 第三步：应用补丁操作
	result, err := ea.applyPatchOperations(sourceFilePath, patchFilePath, targetFilePath, patchData)
	if err != nil {
		// 尝试恢复
		if ea.config.EnableRecovery {
			ea.stats.RecoveryAttempts++
			if recoverErr := ea.attemptRecovery(targetFilePath); recoverErr != nil {
				ea.handleError(recoverErr)
			}
		}
		return nil, fmt.Errorf("应用补丁操作失败: %w", err)
	}

	// 第四步：后验证
	if ea.config.VerifyTarget {
		verifyStart := time.Now()
		if err := ea.postVerification(targetFilePath); err != nil {
			return nil, fmt.Errorf("后验证失败: %w", err)
		}
		ea.stats.VerificationTime += time.Since(verifyStart)
	}

	// 第五步：清理和统计
	ea.finalizeApplication(result)

	return result, nil
}

// preValidationAndBackup 预验证和备份
func (ea *EnhancedApplier) preValidationAndBackup(sourceFilePath, targetFilePath string) error {
	// 验证源文件存在
	if _, err := os.Stat(sourceFilePath); os.IsNotExist(err) {
		return fmt.Errorf("源文件不存在: %s", sourceFilePath)
	}

	// 创建备份
	if ea.config.BackupEnabled && ea.recoveryManager != nil {
		backupStart := time.Now()

		// 如果目标文件存在，创建备份
		if _, err := os.Stat(targetFilePath); err == nil {
			if _, err := ea.recoveryManager.CreateBackup(targetFilePath); err != nil {
				return fmt.Errorf("创建备份失败: %w", err)
			}
		}

		ea.stats.BackupTime += time.Since(backupStart)
	}

	return nil
}

// applyPatchOperations 应用补丁操作
func (ea *EnhancedApplier) applyPatchOperations(sourceFilePath, patchFilePath, targetFilePath string, patchData interface{}) (*EnhancedApplyResult, error) {
	result := &EnhancedApplyResult{
		SourceFilePath:    sourceFilePath,
		PatchFilePath:     patchFilePath,
		TargetFilePath:    targetFilePath,
		StartTime:         time.Now(),
		OperationsApplied: 0,
		BytesProcessed:    0,
		Success:           false,
	}

	// 打开源文件
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return result, fmt.Errorf("打开源文件失败: %w", err)
	}
	defer sourceFile.Close()

	// 创建目标文件
	targetFile, err := os.Create(targetFilePath)
	if err != nil {
		return result, fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer targetFile.Close()

	// 如果启用实时验证，包装目标文件写入器
	var writer io.Writer = targetFile
	if ea.config.EnableRealtime && ea.realtimeVerifier != nil {
		writer = io.MultiWriter(targetFile, ea.realtimeVerifier)
	}

	// 模拟补丁操作应用（这里需要根据实际的补丁格式来实现）
	// 为了演示，我们简单地复制源文件到目标文件
	buffer := make([]byte, ea.config.BufferSize)
	for {
		n, err := sourceFile.Read(buffer)
		if err != nil && err != io.EOF {
			return result, fmt.Errorf("读取源文件失败: %w", err)
		}

		if n == 0 {
			break
		}

		if _, err := writer.Write(buffer[:n]); err != nil {
			return result, fmt.Errorf("写入目标文件失败: %w", err)
		}

		result.BytesProcessed += int64(n)
		ea.stats.BytesProcessed += int64(n)

		if err == io.EOF {
			break
		}
	}

	// 刷新实时验证器
	if ea.config.EnableRealtime && ea.realtimeVerifier != nil {
		if err := ea.realtimeVerifier.Flush(); err != nil {
			return result, fmt.Errorf("实时验证失败: %w", err)
		}
	}

	result.Success = true
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	ea.stats.SuccessOperations++

	return result, nil
}

// postVerification 后验证
func (ea *EnhancedApplier) postVerification(targetFilePath string) error {
	if ea.checker == nil {
		return nil
	}

	// 生成目标文件校验和
	if err := ea.checker.GenerateFileChecksums(targetFilePath); err != nil {
		return fmt.Errorf("生成目标文件校验和失败: %w", err)
	}

	// 验证目标文件完整性
	result, err := ea.checker.VerifyFile(targetFilePath)
	if err != nil {
		return fmt.Errorf("验证目标文件失败: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("目标文件完整性验证失败: %d个块验证失败", result.FailedBlocks)
	}

	return nil
}

// attemptRecovery 尝试恢复
func (ea *EnhancedApplier) attemptRecovery(targetFilePath string) error {
	if ea.recoveryManager == nil {
		return fmt.Errorf("恢复管理器未初始化")
	}

	// 尝试自动恢复
	return ea.recoveryManager.AutoRecover(targetFilePath)
}

// finalizeApplication 完成应用
func (ea *EnhancedApplier) finalizeApplication(result *EnhancedApplyResult) {
	ea.stats.TotalOperations++

	if result.Success {
		ea.stats.SuccessOperations++
	} else {
		ea.stats.FailedOperations++
	}
}

// handleError 处理错误
func (ea *EnhancedApplier) handleError(err error) {
	ea.stats.ErrorsEncountered = append(ea.stats.ErrorsEncountered, err)
}

// GetStats 获取应用统计信息
func (ea *EnhancedApplier) GetStats() *ApplicationStats {
	return &ApplicationStats{
		StartTime:         ea.stats.StartTime,
		EndTime:           ea.stats.EndTime,
		Duration:          ea.stats.Duration,
		TotalOperations:   ea.stats.TotalOperations,
		SuccessOperations: ea.stats.SuccessOperations,
		FailedOperations:  ea.stats.FailedOperations,
		BytesProcessed:    ea.stats.BytesProcessed,
		VerificationTime:  ea.stats.VerificationTime,
		BackupTime:        ea.stats.BackupTime,
		RecoveryAttempts:  ea.stats.RecoveryAttempts,
		ErrorsEncountered: append([]error(nil), ea.stats.ErrorsEncountered...),
	}
}

// GetIntegrityChecker 获取完整性检查器
func (ea *EnhancedApplier) GetIntegrityChecker() *IntegrityChecker {
	return ea.checker
}

// GetRecoveryManager 获取恢复管理器
func (ea *EnhancedApplier) GetRecoveryManager() *RecoveryManager {
	return ea.recoveryManager
}

// GetRealtimeVerifier 获取实时验证器
func (ea *EnhancedApplier) GetRealtimeVerifier() *RealtimeVerifier {
	return ea.realtimeVerifier
}

// EnhancedApplyResult 增强应用结果
type EnhancedApplyResult struct {
	SourceFilePath    string             // 源文件路径
	PatchFilePath     string             // 补丁文件路径
	TargetFilePath    string             // 目标文件路径
	Success           bool               // 是否成功
	OperationsApplied int                // 已应用的操作数
	BytesProcessed    int64              // 处理的字节数
	StartTime         time.Time          // 开始时间
	EndTime           time.Time          // 结束时间
	Duration          time.Duration      // 耗时
	VerificationStats *VerificationStats // 验证统计
	BackupCreated     bool               // 是否创建了备份
	RecoveryUsed      bool               // 是否使用了恢复
}

// String 返回结果的字符串表示
func (ear *EnhancedApplyResult) String() string {
	status := "失败"
	if ear.Success {
		status = "成功"
	}

	return fmt.Sprintf(`增强补丁应用结果:
  状态: %s
  源文件: %s
  补丁文件: %s
  目标文件: %s
  应用操作数: %d
  处理字节数: %d
  耗时: %v
  备份创建: %t
  恢复使用: %t`,
		status,
		filepath.Base(ear.SourceFilePath),
		filepath.Base(ear.PatchFilePath),
		filepath.Base(ear.TargetFilePath),
		ear.OperationsApplied,
		ear.BytesProcessed,
		ear.Duration,
		ear.BackupCreated,
		ear.RecoveryUsed)
}

// String 返回应用统计信息的字符串表示
func (as *ApplicationStats) String() string {
	successRate := float64(as.SuccessOperations) / float64(as.TotalOperations) * 100
	if as.TotalOperations == 0 {
		successRate = 0
	}

	return fmt.Sprintf(`应用统计信息:
  总操作数: %d
  成功操作: %d
  失败操作: %d
  成功率: %.2f%%
  处理字节数: %d
  总耗时: %v
  验证耗时: %v
  备份耗时: %v
  恢复尝试: %d
  错误数量: %d`,
		as.TotalOperations,
		as.SuccessOperations,
		as.FailedOperations,
		successRate,
		as.BytesProcessed,
		as.Duration,
		as.VerificationTime,
		as.BackupTime,
		as.RecoveryAttempts,
		len(as.ErrorsEncountered))
}
