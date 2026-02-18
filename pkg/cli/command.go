package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Sky-ey/HexDiff/pkg/patch"
)

// Command 命令接口
type Command interface {
	Name() string
	Description() string
	Usage() string
	Execute(args []string) error
	SetFlags(fs *flag.FlagSet)
}

// CommandRegistry 命令注册器
type CommandRegistry struct {
	commands map[string]Command
	app      *App
}

// NewCommandRegistry 创建命令注册器
func NewCommandRegistry(app *App) *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]Command),
		app:      app,
	}
}

// Register 注册命令
func (cr *CommandRegistry) Register(cmd Command) {
	cr.commands[cmd.Name()] = cmd
}

// Get 获取命令
func (cr *CommandRegistry) Get(name string) (Command, bool) {
	cmd, exists := cr.commands[name]
	return cmd, exists
}

// List 列出所有命令
func (cr *CommandRegistry) List() []Command {
	commands := make([]Command, 0, len(cr.commands))
	for _, cmd := range cr.commands {
		commands = append(commands, cmd)
	}
	return commands
}

// SignatureCommand 生成签名命令
type SignatureCommand struct {
	app        *App
	outputFile string
	blockSize  int
	verbose    bool
}

// NewSignatureCommand 创建签名命令
func NewSignatureCommand(app *App) *SignatureCommand {
	return &SignatureCommand{
		app:       app,
		blockSize: 4096,
	}
}

func (c *SignatureCommand) Name() string {
	return "signature"
}

func (c *SignatureCommand) Description() string {
	return "为文件生成签名，用于后续的差异检测"
}

func (c *SignatureCommand) Usage() string {
	return "hexdiff signature [options] <input-file>"
}

func (c *SignatureCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.outputFile, "o", "", "输出签名文件路径")
	fs.StringVar(&c.outputFile, "output", "", "输出签名文件路径")
	fs.IntVar(&c.blockSize, "b", 4096, "块大小")
	fs.IntVar(&c.blockSize, "block-size", 4096, "块大小")
	fs.BoolVar(&c.verbose, "v", false, "详细输出")
	fs.BoolVar(&c.verbose, "verbose", false, "详细输出")
}

func (c *SignatureCommand) Execute(args []string) error {
	if len(args) < 1 {
		return ErrInvalidArgumentf("缺少输入文件参数")
	}

	inputFile := args[0]

	// 验证输入文件
	if err := c.validateInputFile(inputFile); err != nil {
		return err
	}

	// 确定输出文件
	outputFile := c.outputFile
	if outputFile == "" {
		outputFile = inputFile + ".sig"
	}

	// 显示操作信息
	c.app.logger.Info("开始生成文件签名...")
	c.app.logger.Info("输入文件: %s", inputFile)
	c.app.logger.Info("输出文件: %s", outputFile)
	c.app.logger.Info("块大小: %d", c.blockSize)

	// 创建进度条
	progress := c.app.progress.NewTask("生成签名", 100)
	defer progress.Finish()

	// 执行签名生成
	if err := c.app.engine.GenerateSignature(inputFile, outputFile, c.blockSize, progress); err != nil {
		return WrapError(ErrPatchGeneration, "生成签名失败", err)
	}

	c.app.logger.Success("签名生成完成: %s", outputFile)
	return nil
}

func (c *SignatureCommand) validateInputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFoundf("输入文件不存在: %s", path)
		}
		return WrapError(ErrFileRead, "无法访问输入文件", err)
	}

	if info.IsDir() {
		return ErrInvalidArgumentf("输入路径是目录，需要文件: %s", path)
	}

	return nil
}

// DiffCommand 差异检测命令
type DiffCommand struct {
	app        *App
	outputFile string
	signature  string
	verbose    bool
	compress   bool
}

// NewDiffCommand 创建差异检测命令
func NewDiffCommand(app *App) *DiffCommand {
	return &DiffCommand{
		app: app,
	}
}

func (c *DiffCommand) Name() string {
	return "diff"
}

func (c *DiffCommand) Description() string {
	return "比较两个文件并生成补丁"
}

func (c *DiffCommand) Usage() string {
	return "hexdiff diff [options] <old-file> <new-file>"
}

func (c *DiffCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.outputFile, "o", "", "输出补丁文件路径")
	fs.StringVar(&c.outputFile, "output", "", "输出补丁文件路径")
	fs.StringVar(&c.signature, "s", "", "使用现有签名文件")
	fs.StringVar(&c.signature, "signature", "", "使用现有签名文件")
	fs.BoolVar(&c.verbose, "v", false, "详细输出")
	fs.BoolVar(&c.verbose, "verbose", false, "详细输出")
	fs.BoolVar(&c.compress, "c", true, "压缩补丁文件")
	fs.BoolVar(&c.compress, "compress", true, "压缩补丁文件")
}

func (c *DiffCommand) Execute(args []string) error {
	if len(args) < 2 {
		return ErrInvalidArgumentf("需要两个文件参数: <old-file> <new-file>")
	}

	oldFile := args[0]
	newFile := args[1]

	// 验证输入文件
	if err := c.validateInputFile(oldFile); err != nil {
		return WrapError(ErrFileRead, "旧文件错误", err)
	}
	if err := c.validateInputFile(newFile); err != nil {
		return WrapError(ErrFileRead, "新文件错误", err)
	}

	// 确定输出文件
	outputFile := c.outputFile
	if outputFile == "" {
		outputFile = fmt.Sprintf("%s_to_%s.patch",
			filepath.Base(oldFile), filepath.Base(newFile))
	}

	// 显示操作信息
	c.app.logger.Info("开始生成补丁...")
	c.app.logger.Info("旧文件: %s", oldFile)
	c.app.logger.Info("新文件: %s", newFile)
	c.app.logger.Info("补丁文件: %s", outputFile)
	if c.signature != "" {
		c.app.logger.Info("使用签名文件: %s", c.signature)
	}

	// 创建进度条
	progress := c.app.progress.NewTask("生成补丁", 100)
	defer progress.Finish()

	// 执行差异检测
	if err := c.app.engine.GeneratePatch(oldFile, newFile, outputFile, c.signature, c.compress, progress); err != nil {
		return WrapError(ErrPatchGeneration, "生成补丁失败", err)
	}

	// 显示补丁信息
	if err := c.showPatchInfo(outputFile); err != nil {
		c.app.logger.Warning("无法显示补丁信息: %v", err)
	}

	c.app.logger.Success("补丁生成完成: %s", outputFile)
	return nil
}

func (c *DiffCommand) validateInputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFoundf("文件不存在: %s", path)
		}
		return WrapError(ErrFileRead, "无法访问文件", err)
	}

	if info.IsDir() {
		return ErrInvalidArgumentf("路径是目录，需要文件: %s", path)
	}

	return nil
}

func (c *DiffCommand) showPatchInfo(patchFile string) error {
	info, err := os.Stat(patchFile)
	if err != nil {
		return err
	}

	c.app.logger.Info("补丁文件大小: %s", formatFileSize(info.Size()))

	// 如果启用详细模式，显示更多信息
	if c.verbose {
		c.app.logger.Info("补丁文件路径: %s", patchFile)
		c.app.logger.Info("创建时间: %s", info.ModTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

// ApplyCommand 应用补丁命令
type ApplyCommand struct {
	app        *App
	outputFile string
	backup     bool
	verify     bool
	verbose    bool
}

// NewApplyCommand 创建应用补丁命令
func NewApplyCommand(app *App) *ApplyCommand {
	return &ApplyCommand{
		app:    app,
		backup: true,
		verify: true,
	}
}

func (c *ApplyCommand) Name() string {
	return "apply"
}

func (c *ApplyCommand) Description() string {
	return "将补丁应用到文件"
}

func (c *ApplyCommand) Usage() string {
	return "hexdiff apply [options] <patch-file> <target-file>"
}

func (c *ApplyCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.outputFile, "o", "", "输出文件路径")
	fs.StringVar(&c.outputFile, "output", "", "输出文件路径")
	fs.BoolVar(&c.backup, "backup", true, "创建备份文件")
	fs.BoolVar(&c.verify, "verify", true, "验证补丁应用结果")
	fs.BoolVar(&c.verbose, "v", false, "详细输出")
	fs.BoolVar(&c.verbose, "verbose", false, "详细输出")
}

func (c *ApplyCommand) Execute(args []string) error {
	if len(args) < 2 {
		return ErrInvalidArgumentf("需要两个参数: <patch-file> <target-file>")
	}

	patchFile := args[0]
	targetFile := args[1]

	// 验证补丁文件
	if err := c.validateInputFile(patchFile); err != nil {
		return WrapError(ErrFileRead, "补丁文件错误", err)
	}

	// 检查是否是目录补丁
	isDirPatch, err := c.isDirectoryPatch(patchFile)
	if err != nil {
		return WrapError(ErrFileRead, "检查补丁类型失败", err)
	}

	if isDirPatch {
		return c.applyDirectoryPatch(patchFile, targetFile)
	}

	// 应用单文件补丁
	return c.applySingleFilePatch(patchFile, targetFile)
}

func (c *ApplyCommand) isDirectoryPatch(patchFile string) (bool, error) {
	isDir, err := patch.IsDirPatch(patchFile)
	if err != nil {
		return false, err
	}
	return isDir, nil
}

func (c *ApplyCommand) applyDirectoryPatch(patchFile, targetDir string) error {
	c.app.logger.Info("检测到目录补丁，正在应用...")
	c.app.logger.Info("补丁文件: %s", patchFile)
	c.app.logger.Info("目标目录: %s", targetDir)

	progress := c.app.progress.NewTask("应用目录补丁", 100)
	defer progress.Finish()

	result, err := c.app.engine.ApplyDirPatch(patchFile, targetDir, true, progress)
	if err != nil {
		return WrapError(ErrPatchApplication, "应用目录补丁失败", err)
	}

	_ = result
	c.app.logger.Success("目录补丁应用完成: %s", targetDir)
	return nil
}

func (c *ApplyCommand) applySingleFilePatch(patchFile, targetFile string) error {
	outputFile := c.outputFile
	if outputFile == "" {
		outputFile = targetFile + ".new"
	}

	// 验证目标文件
	if err := c.validateInputFile(targetFile); err != nil {
		return WrapError(ErrFileRead, "目标文件错误", err)
	}

	// 显示操作信息
	c.app.logger.Info("开始应用补丁...")
	c.app.logger.Info("补丁文件: %s", patchFile)
	c.app.logger.Info("目标文件: %s", targetFile)
	c.app.logger.Info("输出文件: %s", outputFile)

	// 创建备份
	var backupFile string
	if c.backup {
		backupFile = targetFile + ".backup"
		if err := c.createBackup(targetFile, backupFile); err != nil {
			return WrapError(ErrBackupFailed, "创建备份失败", err)
		}
		c.app.logger.Info("备份文件: %s", backupFile)
	}

	// 创建进度条
	progress := c.app.progress.NewTask("应用补丁", 100)
	defer progress.Finish()

	// 应用补丁
	if err := c.app.engine.ApplyPatch(patchFile, targetFile, outputFile, c.verify, progress); err != nil {
		// 如果失败且有备份，提示恢复
		if c.backup && backupFile != "" {
			c.app.logger.Error("补丁应用失败，可以使用备份文件恢复: %s", backupFile)
		}
		return WrapError(ErrPatchApplication, "应用补丁失败", err)
	}

	c.app.logger.Success("补丁应用完成: %s", outputFile)

	// 显示结果信息
	if err := c.showResultInfo(outputFile); err != nil {
		c.app.logger.Warning("无法显示结果信息: %v", err)
	}

	return nil
}

func (c *ApplyCommand) validateInputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFoundf("文件不存在: %s", path)
		}
		return WrapError(ErrFileRead, "无法访问文件", err)
	}

	if info.IsDir() {
		return ErrInvalidArgumentf("路径是目录，需要文件: %s", path)
	}

	return nil
}

func (c *ApplyCommand) createBackup(source, backup string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	backupFile, err := os.Create(backup)
	if err != nil {
		return err
	}
	defer backupFile.Close()

	_, err = sourceFile.WriteTo(backupFile)
	return err
}

func (c *ApplyCommand) showResultInfo(outputFile string) error {
	info, err := os.Stat(outputFile)
	if err != nil {
		return err
	}

	c.app.logger.Info("输出文件大小: %s", formatFileSize(info.Size()))

	if c.verbose {
		c.app.logger.Info("输出文件路径: %s", outputFile)
		c.app.logger.Info("修改时间: %s", info.ModTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

// ValidateCommand 验证命令
type ValidateCommand struct {
	app     *App
	verbose bool
}

// NewValidateCommand 创建验证命令
func NewValidateCommand(app *App) *ValidateCommand {
	return &ValidateCommand{
		app: app,
	}
}

func (c *ValidateCommand) Name() string {
	return "validate"
}

func (c *ValidateCommand) Description() string {
	return "验证补丁文件的完整性"
}

func (c *ValidateCommand) Usage() string {
	return "hexdiff validate [options] <patch-file>"
}

func (c *ValidateCommand) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&c.verbose, "v", false, "详细输出")
	fs.BoolVar(&c.verbose, "verbose", false, "详细输出")
}

func (c *ValidateCommand) Execute(args []string) error {
	if len(args) < 1 {
		return ErrInvalidArgumentf("缺少补丁文件参数")
	}

	patchFile := args[0]

	// 验证文件存在
	if err := c.validateInputFile(patchFile); err != nil {
		return err
	}

	c.app.logger.Info("开始验证补丁文件...")
	c.app.logger.Info("补丁文件: %s", patchFile)

	// 创建进度条
	progress := c.app.progress.NewTask("验证补丁", 100)
	defer progress.Finish()

	// 执行验证
	result, err := c.app.engine.ValidatePatch(patchFile, progress)
	if err != nil {
		return WrapError(ErrPatchValidation, "验证失败", err)
	}

	// 显示验证结果
	c.showValidationResult(result)

	if result.Valid {
		c.app.logger.Success("补丁文件验证通过")
	} else {
		c.app.logger.Error("补丁文件验证失败")
		return NewCLIError(ErrPatchValidation, "补丁文件无效")
	}

	return nil
}

func (c *ValidateCommand) validateInputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFoundf("补丁文件不存在: %s", path)
		}
		return WrapError(ErrFileRead, "无法访问补丁文件", err)
	}

	if info.IsDir() {
		return ErrInvalidArgumentf("路径是目录，需要文件: %s", path)
	}

	return nil
}

func (c *ValidateCommand) showValidationResult(result *ValidationResult) {
	c.app.logger.Info("验证结果:")
	c.app.logger.Info("  文件格式: %s", getStatusString(result.ValidFormat))
	c.app.logger.Info("  校验和: %s", getStatusString(result.ValidChecksum))
	c.app.logger.Info("  数据完整性: %s", getStatusString(result.ValidData))

	if c.verbose && len(result.Errors) > 0 {
		c.app.logger.Info("错误详情:")
		for _, err := range result.Errors {
			c.app.logger.Error("  - %s", err)
		}
	}
}

// InfoCommand 信息查看命令
type InfoCommand struct {
	app     *App
	verbose bool
}

// NewInfoCommand 创建信息查看命令
func NewInfoCommand(app *App) *InfoCommand {
	return &InfoCommand{
		app: app,
	}
}

func (c *InfoCommand) Name() string {
	return "info"
}

func (c *InfoCommand) Description() string {
	return "显示补丁文件信息"
}

func (c *InfoCommand) Usage() string {
	return "hexdiff info [options] <patch-file>"
}

func (c *InfoCommand) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&c.verbose, "v", false, "详细输出")
	fs.BoolVar(&c.verbose, "verbose", false, "详细输出")
}

func (c *InfoCommand) Execute(args []string) error {
	if len(args) < 1 {
		return ErrInvalidArgumentf("缺少补丁文件参数")
	}

	patchFile := args[0]

	// 验证文件存在
	if err := c.validateInputFile(patchFile); err != nil {
		return err
	}

	c.app.logger.Info("读取补丁文件信息...")

	// 获取补丁信息
	info, err := c.app.engine.GetPatchInfo(patchFile)
	if err != nil {
		return WrapError(ErrFileRead, "读取补丁信息失败", err)
	}

	// 显示信息
	c.showPatchInfo(info)

	return nil
}

func (c *InfoCommand) validateInputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFoundf("补丁文件不存在: %s", path)
		}
		return WrapError(ErrFileRead, "无法访问补丁文件", err)
	}

	if info.IsDir() {
		return ErrInvalidArgumentf("路径是目录，需要文件: %s", path)
	}

	return nil
}

func (c *InfoCommand) showPatchInfo(info *PatchInfo) {
	c.app.logger.Info("补丁文件信息:")
	c.app.logger.Info("  版本: %d", info.Version)
	c.app.logger.Info("  压缩: %s", getCompressionString(info.Compression))
	c.app.logger.Info("  源文件校验和: %x", info.SourceChecksum)
	c.app.logger.Info("  目标文件校验和: %x", info.TargetChecksum)
	c.app.logger.Info("  操作数量: %d", info.OperationCount)
	c.app.logger.Info("  补丁大小: %s", formatFileSize(info.PatchSize))

	if c.verbose {
		c.app.logger.Info("  创建时间: %s", info.CreatedAt.Format("2006-01-02 15:04:05"))
		if info.Metadata != nil {
			c.app.logger.Info("  元数据:")
			for key, value := range info.Metadata {
				c.app.logger.Info("    %s: %s", key, value)
			}
		}
	}
}

// 辅助函数
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func getStatusString(valid bool) string {
	if valid {
		return "✅ 通过"
	}
	return "❌ 失败"
}

func getCompressionString(compression CompressionType) string {
	switch compression {
	case CompressionNone:
		return "无"
	case CompressionGzip:
		return "Gzip"
	case CompressionLZ4:
		return "LZ4"
	default:
		return "未知"
	}
}

// 类型定义（这些应该在其他包中定义，这里为了编译通过临时定义）
type ValidationResult struct {
	Valid         bool
	ValidFormat   bool
	ValidChecksum bool
	ValidData     bool
	Errors        []string
}

type PatchInfo struct {
	Version        uint16
	Compression    CompressionType
	SourceChecksum []byte
	TargetChecksum []byte
	OperationCount int
	PatchSize      int64
	CreatedAt      time.Time
	Metadata       map[string]string
}

type CompressionType int

const (
	CompressionNone CompressionType = iota
	CompressionGzip
	CompressionLZ4
)

// DirDiffCommand 目录差异检测命令
type DirDiffCommand struct {
	app          *App
	outputFile   string
	recursive    bool
	ignoreHidden bool
	ignore       string
	compress     bool
	verbose      bool
}

// NewDirDiffCommand 创建目录差异检测命令
func NewDirDiffCommand(app *App) *DirDiffCommand {
	return &DirDiffCommand{
		app:       app,
		recursive: true,
		compress:  true,
	}
}

func (c *DirDiffCommand) Name() string {
	return "dir-diff"
}

func (c *DirDiffCommand) Description() string {
	return "比较两个目录并生成补丁"
}

func (c *DirDiffCommand) Usage() string {
	return "hexdiff dir-diff [options] <old-dir> <new-dir>"
}

func (c *DirDiffCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.outputFile, "o", "", "输出补丁文件路径")
	fs.StringVar(&c.outputFile, "output", "", "输出补丁文件路径")
	fs.BoolVar(&c.recursive, "r", true, "递归遍历子目录")
	fs.BoolVar(&c.recursive, "recursive", true, "递归遍历子目录")
	fs.BoolVar(&c.ignoreHidden, "ignore-hidden", false, "忽略隐藏文件")
	fs.StringVar(&c.ignore, "ignore", "", "忽略的文件模式（逗号分隔）")
	fs.BoolVar(&c.compress, "c", true, "压缩补丁文件")
	fs.BoolVar(&c.compress, "compress", true, "压缩补丁文件")
	fs.BoolVar(&c.verbose, "v", false, "详细输出")
	fs.BoolVar(&c.verbose, "verbose", false, "详细输出")
}

func (c *DirDiffCommand) Execute(args []string) error {
	if len(args) < 2 {
		return ErrInvalidArgumentf("需要两个目录参数: <old-dir> <new-dir>")
	}

	oldDir := args[0]
	newDir := args[1]

	if err := c.validateDirectory(oldDir); err != nil {
		return WrapError(ErrFileRead, "旧目录错误", err)
	}
	if err := c.validateDirectory(newDir); err != nil {
		return WrapError(ErrFileRead, "新目录错误", err)
	}

	outputFile := c.outputFile
	if outputFile == "" {
		outputFile = fmt.Sprintf("%s_to_%s.dir.patch",
			filepath.Base(oldDir), filepath.Base(newDir))
	}

	c.app.logger.Info("开始生成目录补丁...")
	c.app.logger.Info("旧目录: %s", oldDir)
	c.app.logger.Info("新目录: %s", newDir)
	c.app.logger.Info("输出文件: %s", outputFile)

	progress := c.app.progress.NewTask("生成目录补丁", 100)
	defer progress.Finish()

	result, err := c.app.engine.GenerateDirDiff(oldDir, newDir, outputFile, c.recursive, !c.ignoreHidden, c.ignore, c.compress, progress)
	if err != nil {
		return WrapError(ErrPatchGeneration, "生成目录补丁失败", err)
	}

	c.showDirDiffResult(result)

	c.app.logger.Success("目录补丁生成完成: %s", outputFile)
	return nil
}

func (c *DirDiffCommand) validateDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFoundf("目录不存在: %s", path)
		}
		return WrapError(ErrFileRead, "无法访问目录", err)
	}

	if !info.IsDir() {
		return ErrInvalidArgumentf("路径不是目录: %s", path)
	}

	return nil
}

func (c *DirDiffCommand) showDirDiffResult(result interface{}) {
	c.app.logger.Info("目录差异统计:")
}
