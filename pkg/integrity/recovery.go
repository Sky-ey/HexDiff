package integrity

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RecoveryManager 恢复管理器
type RecoveryManager struct {
	backupDir    string              // 备份目录
	maxBackups   int                 // 最大备份数量
	checker      *IntegrityChecker   // 完整性检查器
	errorHandler func(error)         // 错误处理函数
	recoveryLog  []RecoveryOperation // 恢复操作日志
}

// RecoveryOperation 恢复操作
type RecoveryOperation struct {
	Timestamp  time.Time     // 操作时间戳
	Operation  string        // 操作类型
	FilePath   string        // 文件路径
	BackupPath string        // 备份路径
	Success    bool          // 是否成功
	Error      error         // 错误信息
	Duration   time.Duration // 操作耗时
}

// RecoveryConfig 恢复配置
type RecoveryConfig struct {
	BackupDir    string      // 备份目录
	MaxBackups   int         // 最大备份数量
	ErrorHandler func(error) // 错误处理函数
}

// DefaultRecoveryConfig 默认恢复配置
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		BackupDir:  ".hexdiff_backups",
		MaxBackups: 5,
		ErrorHandler: func(err error) {
			fmt.Printf("恢复错误: %v\n", err)
		},
	}
}

// NewRecoveryManager 创建新的恢复管理器
func NewRecoveryManager(checker *IntegrityChecker, config *RecoveryConfig) *RecoveryManager {
	if config == nil {
		config = DefaultRecoveryConfig()
	}

	return &RecoveryManager{
		backupDir:    config.BackupDir,
		maxBackups:   config.MaxBackups,
		checker:      checker,
		errorHandler: config.ErrorHandler,
		recoveryLog:  make([]RecoveryOperation, 0),
	}
}

// CreateBackup 创建文件备份
func (rm *RecoveryManager) CreateBackup(filePath string) (string, error) {
	startTime := time.Now()

	// 确保备份目录存在
	if err := os.MkdirAll(rm.backupDir, 0755); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 生成备份文件名
	fileName := filepath.Base(filePath)
	timestamp := time.Now().Format("20060102_150405")
	backupFileName := fmt.Sprintf("%s.%s.backup", fileName, timestamp)
	backupPath := filepath.Join(rm.backupDir, backupFileName)

	// 复制文件
	err := rm.copyFile(filePath, backupPath)

	// 记录操作
	operation := RecoveryOperation{
		Timestamp:  startTime,
		Operation:  "CREATE_BACKUP",
		FilePath:   filePath,
		BackupPath: backupPath,
		Success:    err == nil,
		Error:      err,
		Duration:   time.Since(startTime),
	}
	rm.recoveryLog = append(rm.recoveryLog, operation)

	if err != nil {
		if rm.errorHandler != nil {
			rm.errorHandler(err)
		}
		return "", fmt.Errorf("创建备份失败: %w", err)
	}

	// 清理旧备份
	rm.cleanupOldBackups(fileName)

	return backupPath, nil
}

// RestoreFromBackup 从备份恢复文件
func (rm *RecoveryManager) RestoreFromBackup(filePath, backupPath string) error {
	startTime := time.Now()

	// 验证备份文件存在
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	// 验证备份文件完整性
	if rm.checker != nil {
		if err := rm.checker.GenerateFileChecksums(backupPath); err != nil {
			return fmt.Errorf("生成备份文件校验和失败: %w", err)
		}

		result, err := rm.checker.VerifyFile(backupPath)
		if err != nil {
			return fmt.Errorf("验证备份文件失败: %w", err)
		}

		if !result.Success {
			return fmt.Errorf("备份文件完整性验证失败")
		}
	}

	// 恢复文件
	err := rm.copyFile(backupPath, filePath)

	// 记录操作
	operation := RecoveryOperation{
		Timestamp:  startTime,
		Operation:  "RESTORE_FROM_BACKUP",
		FilePath:   filePath,
		BackupPath: backupPath,
		Success:    err == nil,
		Error:      err,
		Duration:   time.Since(startTime),
	}
	rm.recoveryLog = append(rm.recoveryLog, operation)

	if err != nil {
		if rm.errorHandler != nil {
			rm.errorHandler(err)
		}
		return fmt.Errorf("从备份恢复失败: %w", err)
	}

	return nil
}

// FindLatestBackup 查找最新的备份文件
func (rm *RecoveryManager) FindLatestBackup(fileName string) (string, error) {
	backupPattern := fmt.Sprintf("%s.*.backup", fileName)
	backupGlob := filepath.Join(rm.backupDir, backupPattern)

	matches, err := filepath.Glob(backupGlob)
	if err != nil {
		return "", fmt.Errorf("查找备份文件失败: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("未找到备份文件: %s", fileName)
	}

	// 找到最新的备份文件（按修改时间）
	var latestBackup string
	var latestTime time.Time

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestBackup = match
		}
	}

	if latestBackup == "" {
		return "", fmt.Errorf("未找到有效的备份文件")
	}

	return latestBackup, nil
}

// AutoRecover 自动恢复损坏的文件
func (rm *RecoveryManager) AutoRecover(filePath string) error {
	// 首先验证文件完整性
	if rm.checker != nil {
		result, err := rm.checker.VerifyFile(filePath)
		if err != nil {
			return fmt.Errorf("验证文件完整性失败: %w", err)
		}

		if result.Success {
			// 文件完整，无需恢复
			return nil
		}
	}

	// 查找最新备份
	fileName := filepath.Base(filePath)
	backupPath, err := rm.FindLatestBackup(fileName)
	if err != nil {
		return fmt.Errorf("查找备份文件失败: %w", err)
	}

	// 从备份恢复
	return rm.RestoreFromBackup(filePath, backupPath)
}

// copyFile 复制文件
func (rm *RecoveryManager) copyFile(src, dst string) error {
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
	if err != nil {
		return err
	}

	// 同步到磁盘
	return destFile.Sync()
}

// cleanupOldBackups 清理旧备份
func (rm *RecoveryManager) cleanupOldBackups(fileName string) {
	backupPattern := fmt.Sprintf("%s.*.backup", fileName)
	backupGlob := filepath.Join(rm.backupDir, backupPattern)

	matches, err := filepath.Glob(backupGlob)
	if err != nil {
		return
	}

	if len(matches) <= rm.maxBackups {
		return
	}

	// 按修改时间排序，删除最旧的备份
	type backupInfo struct {
		path    string
		modTime time.Time
	}

	backups := make([]backupInfo, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		backups = append(backups, backupInfo{
			path:    match,
			modTime: info.ModTime(),
		})
	}

	// 简单排序（按时间）
	for i := 0; i < len(backups)-1; i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].modTime.After(backups[j].modTime) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	// 删除多余的备份
	toDelete := len(backups) - rm.maxBackups
	for i := 0; i < toDelete; i++ {
		os.Remove(backups[i].path)
	}
}

// GetRecoveryLog 获取恢复操作日志
func (rm *RecoveryManager) GetRecoveryLog() []RecoveryOperation {
	return rm.recoveryLog
}

// ClearRecoveryLog 清空恢复操作日志
func (rm *RecoveryManager) ClearRecoveryLog() {
	rm.recoveryLog = rm.recoveryLog[:0]
}

// GetBackupInfo 获取备份信息
func (rm *RecoveryManager) GetBackupInfo() (*BackupInfo, error) {
	info := &BackupInfo{
		BackupDir:   rm.backupDir,
		MaxBackups:  rm.maxBackups,
		BackupFiles: make([]BackupFileInfo, 0),
	}

	// 扫描备份目录
	entries, err := os.ReadDir(rm.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return info, nil // 备份目录不存在，返回空信息
		}
		return nil, fmt.Errorf("读取备份目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(rm.backupDir, entry.Name())
		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}

		backupFile := BackupFileInfo{
			Name:    entry.Name(),
			Path:    filePath,
			Size:    fileInfo.Size(),
			ModTime: fileInfo.ModTime(),
		}

		info.BackupFiles = append(info.BackupFiles, backupFile)
		info.TotalSize += fileInfo.Size()
	}

	info.TotalFiles = len(info.BackupFiles)
	return info, nil
}

// BackupInfo 备份信息
type BackupInfo struct {
	BackupDir   string           // 备份目录
	MaxBackups  int              // 最大备份数量
	TotalFiles  int              // 总文件数
	TotalSize   int64            // 总大小
	BackupFiles []BackupFileInfo // 备份文件列表
}

// BackupFileInfo 备份文件信息
type BackupFileInfo struct {
	Name    string    // 文件名
	Path    string    // 文件路径
	Size    int64     // 文件大小
	ModTime time.Time // 修改时间
}

// String 返回备份信息的字符串表示
func (bi *BackupInfo) String() string {
	return fmt.Sprintf(`备份信息:
  备份目录: %s
  最大备份数: %d
  当前文件数: %d
  总大小: %d 字节`,
		bi.BackupDir,
		bi.MaxBackups,
		bi.TotalFiles,
		bi.TotalSize)
}
