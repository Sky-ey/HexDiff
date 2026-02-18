package patch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Sky-ey/HexDiff/pkg/integrity"
)

// Applier 补丁应用器
type Applier struct {
	config           *ApplierConfig
	integrityChecker *integrity.IntegrityChecker
	recoveryManager  *integrity.RecoveryManager
	realtimeVerifier *integrity.RealtimeVerifier
}

// ApplierConfig 补丁应用器配置
type ApplierConfig struct {
	BufferSize      int    // 缓冲区大小
	TempDir         string // 临时目录
	BackupEnabled   bool   // 是否启用备份
	VerifyTarget    bool   // 是否验证目标文件
	EnableIntegrity bool   // 是否启用完整性检查
	EnableRealtime  bool   // 是否启用实时验证
	EnableRecovery  bool   // 是否启用恢复功能
	BlockSize       int    // 完整性检查块大小
}

// DefaultApplierConfig 默认配置
func DefaultApplierConfig() *ApplierConfig {
	return &ApplierConfig{
		BufferSize:      64 * 1024, // 64KB
		TempDir:         os.TempDir(),
		BackupEnabled:   true,
		VerifyTarget:    true,
		EnableIntegrity: true,
		EnableRealtime:  true,
		EnableRecovery:  true,
		BlockSize:       64 * 1024, // 64KB
	}
}

// NewApplier 创建新的补丁应用器
func NewApplier(config *ApplierConfig) *Applier {
	if config == nil {
		config = DefaultApplierConfig()
	}

	applier := &Applier{
		config: config,
	}

	// 初始化完整性检查器
	if config.EnableIntegrity {
		checkerConfig := &integrity.CheckerConfig{
			BlockSize:    config.BlockSize,
			EnableSHA256: true,
			EnableCRC32:  true,
		}
		applier.integrityChecker = integrity.NewIntegrityChecker(checkerConfig)
	}

	// 初始化恢复管理器
	if config.EnableRecovery && applier.integrityChecker != nil {
		recoveryConfig := &integrity.RecoveryConfig{
			BackupDir:  filepath.Join(config.TempDir, ".hexdiff_backups"),
			MaxBackups: 5,
		}
		applier.recoveryManager = integrity.NewRecoveryManager(applier.integrityChecker, recoveryConfig)
	}

	// 初始化实时验证器
	if config.EnableRealtime && applier.integrityChecker != nil {
		applier.realtimeVerifier = integrity.NewRealtimeVerifier(applier.integrityChecker, config.BlockSize)
	}

	return applier
}

// ApplyPatch 应用补丁到文件
func (a *Applier) ApplyPatch(sourceFilePath, patchFilePath, targetFilePath string) (*ApplyResult, error) {
	// 验证输入文件
	if err := a.validateInputFiles(sourceFilePath, patchFilePath); err != nil {
		return nil, fmt.Errorf("validate input files: %w", err)
	}

	// 读取补丁文件
	serializer := NewSerializer(CompressionNone)
	patchFile, err := serializer.DeserializePatch(patchFilePath)
	if err != nil {
		return nil, fmt.Errorf("deserialize patch: %w", err)
	}

	// 验证源文件校验和
	if err := a.verifySourceFile(sourceFilePath, patchFile.Header.SourceChecksum); err != nil {
		return nil, fmt.Errorf("verify source file: %w", err)
	}

	// 创建临时文件进行原子操作
	tempFile, err := a.createTempFile(targetFilePath)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tempFile) // 清理临时文件

	// 应用补丁操作
	result, err := a.applyOperations(sourceFilePath, patchFile, tempFile)
	if err != nil {
		return nil, fmt.Errorf("apply operations: %w", err)
	}

	// 验证目标文件校验和
	if a.config.VerifyTarget {
		if err := a.verifyTargetFile(tempFile, patchFile.Header.TargetChecksum); err != nil {
			return nil, fmt.Errorf("verify target file: %w", err)
		}
	}

	// 创建备份（如果启用）
	if a.config.BackupEnabled {
		if err := a.createBackup(targetFilePath); err != nil {
			return nil, fmt.Errorf("create backup: %w", err)
		}
	}

	// 原子性替换目标文件
	if err := a.atomicReplace(tempFile, targetFilePath); err != nil {
		return nil, fmt.Errorf("atomic replace: %w", err)
	}

	result.TargetFilePath = targetFilePath
	result.Success = true

	return result, nil
}

// validateInputFiles 验证输入文件
func (a *Applier) validateInputFiles(sourceFilePath, patchFilePath string) error {
	// 检查源文件
	if _, err := os.Stat(sourceFilePath); os.IsNotExist(err) {
		return fmt.Errorf("source file does not exist: %s", sourceFilePath)
	}

	// 检查补丁文件
	if _, err := os.Stat(patchFilePath); os.IsNotExist(err) {
		return fmt.Errorf("patch file does not exist: %s", patchFilePath)
	}

	return nil
}

// verifySourceFile 验证源文件校验和
func (a *Applier) verifySourceFile(filePath string, expectedChecksum [32]byte) error {
	actualChecksum, err := calculateFileChecksum(filePath)
	if err != nil {
		return err
	}

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("source file checksum mismatch: expected %x, got %x",
			expectedChecksum, actualChecksum)
	}

	return nil
}

// verifyTargetFile 验证目标文件校验和
func (a *Applier) verifyTargetFile(filePath string, expectedChecksum [32]byte) error {
	actualChecksum, err := calculateFileChecksum(filePath)
	if err != nil {
		return err
	}

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("target file checksum mismatch: expected %x, got %x",
			expectedChecksum, actualChecksum)
	}

	return nil
}

// createTempFile 创建临时文件
func (a *Applier) createTempFile(targetFilePath string) (string, error) {
	dir := filepath.Dir(targetFilePath)
	base := filepath.Base(targetFilePath)

	tempFile, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return "", err
	}

	tempPath := tempFile.Name()
	tempFile.Close()

	return tempPath, nil
}

// createBackup 创建备份文件
func (a *Applier) createBackup(targetFilePath string) error {
	// 如果目标文件不存在，不需要备份
	if _, err := os.Stat(targetFilePath); os.IsNotExist(err) {
		return nil
	}

	backupPath := targetFilePath + ".backup"
	return copyFile(targetFilePath, backupPath)
}

// atomicReplace 原子性替换文件
func (a *Applier) atomicReplace(tempFilePath, targetFilePath string) error {
	return os.Rename(tempFilePath, targetFilePath)
}

// applyOperations 应用补丁操作
func (a *Applier) applyOperations(sourceFilePath string, patchFile *PatchFile, targetFilePath string) (*ApplyResult, error) {
	// 打开源文件
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return nil, fmt.Errorf("open source file: %w", err)
	}
	defer sourceFile.Close()

	// 创建目标文件
	targetFile, err := os.Create(targetFilePath)
	if err != nil {
		return nil, fmt.Errorf("create target file: %w", err)
	}
	defer targetFile.Close()

	result := &ApplyResult{
		SourceFilePath:    sourceFilePath,
		PatchFilePath:     "",
		OperationsApplied: 0,
		BytesProcessed:    0,
	}

	// 按顺序应用每个操作
	for i, op := range patchFile.Operations {
		if err := a.applyOperation(sourceFile, targetFile, &op, patchFile.Data, result); err != nil {
			return nil, fmt.Errorf("apply operation %d: %w", i, err)
		}
		result.OperationsApplied++
	}

	return result, nil
}

// applyOperation 应用单个操作
func (a *Applier) applyOperation(sourceFile, targetFile *os.File, op *PatchOperation, patchData []byte, result *ApplyResult) error {
	// 定位到目标文件的指定偏移量
	if _, err := targetFile.Seek(int64(op.Offset), 0); err != nil {
		return fmt.Errorf("seek target file: %w", err)
	}

	switch op.Type {
	case 0: // Copy操作
		return a.applyCopyOperation(sourceFile, targetFile, op, result)
	case 1: // Insert操作
		return a.applyInsertOperation(targetFile, op, patchData, result)
	case 2: // Delete操作
		return a.applyDeleteOperation(op, result)
	default:
		return fmt.Errorf("unknown operation type: %d", op.Type)
	}
}

// applyCopyOperation 应用复制操作
func (a *Applier) applyCopyOperation(sourceFile, targetFile *os.File, op *PatchOperation, result *ApplyResult) error {
	// 定位到源文件的指定位置
	if _, err := sourceFile.Seek(int64(op.SrcOffset), 0); err != nil {
		return fmt.Errorf("seek source file: %w", err)
	}

	// 复制指定大小的数据
	buffer := make([]byte, min(int(op.Size), a.config.BufferSize))
	remaining := int(op.Size)

	for remaining > 0 {
		toRead := min(remaining, len(buffer))
		n, err := sourceFile.Read(buffer[:toRead])
		if err != nil && err != io.EOF {
			return fmt.Errorf("read from source: %w", err)
		}

		if n == 0 {
			break
		}

		if _, err := targetFile.Write(buffer[:n]); err != nil {
			return fmt.Errorf("write to target: %w", err)
		}

		remaining -= n
		result.BytesProcessed += int64(n)
	}

	return nil
}

// applyInsertOperation 应用插入操作
func (a *Applier) applyInsertOperation(targetFile *os.File, op *PatchOperation, patchData []byte, result *ApplyResult) error {
	// 从补丁数据中获取要插入的数据
	if op.DataOffset+op.Size > uint32(len(patchData)) {
		return fmt.Errorf("insert data out of bounds: offset=%d, size=%d, total=%d",
			op.DataOffset, op.Size, len(patchData))
	}

	insertData := patchData[op.DataOffset : op.DataOffset+op.Size]

	// 写入数据到目标文件
	if _, err := targetFile.Write(insertData); err != nil {
		return fmt.Errorf("write insert data: %w", err)
	}

	result.BytesProcessed += int64(op.Size)
	return nil
}

// applyDeleteOperation 应用删除操作
func (a *Applier) applyDeleteOperation(op *PatchOperation, result *ApplyResult) error {
	// 删除操作在当前实现中是隐式的（不复制被删除的数据）
	// 这里只是记录操作，实际的删除通过不复制相应数据来实现
	result.BytesProcessed += int64(op.Size)
	return nil
}

// ApplyResult 补丁应用结果
type ApplyResult struct {
	SourceFilePath    string // 源文件路径
	PatchFilePath     string // 补丁文件路径
	TargetFilePath    string // 目标文件路径
	Success           bool   // 是否成功
	OperationsApplied int    // 已应用的操作数
	BytesProcessed    int64  // 处理的字节数
}

// String 返回结果的字符串表示
func (r *ApplyResult) String() string {
	status := "失败"
	if r.Success {
		status = "成功"
	}

	return fmt.Sprintf(`补丁应用结果:
  状态: %s
  源文件: %s
  补丁文件: %s
  目标文件: %s
  应用操作数: %d
  处理字节数: %d`,
		status,
		filepath.Base(r.SourceFilePath),
		filepath.Base(r.PatchFilePath),
		filepath.Base(r.TargetFilePath),
		r.OperationsApplied,
		r.BytesProcessed,
	)
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (a *Applier) ApplyDelta(sourceFilePath string, deltaData []byte, targetFilePath string) error {
	serializer := NewSerializer(CompressionNone)
	patchFile, err := serializer.DeserializeFromData(deltaData)
	if err != nil {
		return fmt.Errorf("deserialize delta: %w", err)
	}

	sourceChecksum := patchFile.Header.SourceChecksum
	isZeroChecksum := true
	for _, b := range sourceChecksum {
		if b != 0 {
			isZeroChecksum = false
			break
		}
	}

	if !isZeroChecksum {
		if err := a.verifySourceFile(sourceFilePath, patchFile.Header.SourceChecksum); err != nil {
			return fmt.Errorf("verify source file: %w", err)
		}
	}

	tempFile, err := a.createTempFile(targetFilePath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tempFile)

	_, err = a.applyOperations(sourceFilePath, patchFile, tempFile)
	if err != nil {
		return fmt.Errorf("apply operations: %w", err)
	}

	targetChecksum := patchFile.Header.TargetChecksum
	isZeroChecksum = true
	for _, b := range targetChecksum {
		if b != 0 {
			isZeroChecksum = false
			break
		}
	}

	if !isZeroChecksum && a.config.VerifyTarget {
		if err := a.verifyTargetFile(tempFile, patchFile.Header.TargetChecksum); err != nil {
			return fmt.Errorf("verify target file: %w", err)
		}
	}

	if err := a.atomicReplace(tempFile, targetFilePath); err != nil {
		return fmt.Errorf("atomic replace: %w", err)
	}

	return nil
}
