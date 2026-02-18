package cli

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// App 应用程序主结构
type App struct {
	name        string
	version     string
	description string
	config      *Config
	logger      *Logger
	progress    *ProgressManager
	engine      Engine
	registry    *CommandRegistry
}

// Engine 引擎接口（需要在其他包中实现）
type Engine interface {
	GenerateSignature(inputFile, outputFile string, blockSize int, progress ProgressReporter) error
	GeneratePatch(oldFile, newFile, outputFile, signature string, compress bool, progress ProgressReporter) error
	GenerateDirDiff(oldDir, newDir, outputFile string, recursive, ignoreHidden bool, ignorePatterns string, compress bool, progress ProgressReporter) (interface{}, error)
	ApplyPatch(patchFile, targetFile, outputFile string, verify bool, progress ProgressReporter) error
	ApplyDirPatch(patchFile, targetDir string, verify bool, progress ProgressReporter) (interface{}, error)
	ValidatePatch(patchFile string, progress ProgressReporter) (*ValidationResult, error)
	GetPatchInfo(patchFile string) (*PatchInfo, error)
	GetDirPatchInfo(patchFile string) (*DirPatchInfo, error)
}

// NewApp 创建新的应用程序实例
func NewApp(name, version, description string, engine Engine) *App {
	app := &App{
		name:        name,
		version:     version,
		description: description,
		engine:      engine,
	}

	// 初始化组件
	app.config = NewConfig()
	app.logger = NewLogger(app.config.LogLevel, app.config.LogFile)
	app.progress = NewProgressManager(app.config.ShowProgress)
	app.registry = NewCommandRegistry(app)

	// 注册默认命令
	app.registerDefaultCommands()

	return app
}

// registerDefaultCommands 注册默认命令
func (app *App) registerDefaultCommands() {
	app.registry.Register(NewSignatureCommand(app))
	app.registry.Register(NewDiffCommand(app))
	app.registry.Register(NewDirDiffCommand(app))
	app.registry.Register(NewApplyCommand(app))
	app.registry.Register(NewValidateCommand(app))
	app.registry.Register(NewInfoCommand(app))
	app.registry.Register(NewHelpCommand(app))
	app.registry.Register(NewVersionCommand(app))
	app.registry.Register(NewBenchmarkCommand(app))
	app.registry.Register(NewConfigCommand(app))
}

// Run 运行应用程序
func (app *App) Run(args []string) error {
	// 解析全局参数
	if err := app.parseGlobalFlags(args); err != nil {
		return err
	}

	// 如果没有参数，显示帮助
	if len(args) <= 1 {
		return app.showHelp()
	}

	// 获取命令名称
	cmdName := args[1]
	cmdArgs := args[2:]

	// 处理特殊命令
	switch cmdName {
	case "help", "-h", "--help":
		return app.showHelp()
	case "version", "-v", "--version":
		return app.showVersion()
	}

	// 查找并执行命令
	cmd, exists := app.registry.Get(cmdName)
	if !exists {
		return fmt.Errorf("未知命令: %s\n\n使用 '%s help' 查看可用命令", cmdName, app.name)
	}

	// 解析命令参数
	fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: %s\n\n", cmd.Usage())
		fmt.Fprintf(os.Stderr, "%s\n\n", cmd.Description())
		fmt.Fprintf(os.Stderr, "选项:\n")
		fs.PrintDefaults()
	}

	// 设置命令标志
	cmd.SetFlags(fs)

	// 解析参数
	if err := fs.Parse(cmdArgs); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return fmt.Errorf("参数解析错误: %w", err)
	}

	// 执行命令
	app.logger.Debug("执行命令: %s", cmdName)
	startTime := time.Now()

	err := cmd.Execute(fs.Args())

	duration := time.Since(startTime)
	if err != nil {
		app.logger.Error("命令执行失败: %v (耗时: %v)", err, duration)
		return err
	}

	app.logger.Debug("命令执行完成，耗时: %v", duration)
	return nil
}

// parseGlobalFlags 解析全局标志
func (app *App) parseGlobalFlags(args []string) error {
	// 创建全局标志集
	fs := flag.NewFlagSet("global", flag.ContinueOnError)
	fs.Usage = func() {} // 禁用默认用法输出

	var (
		configFile = fs.String("config", "", "配置文件路径")
		logLevel   = fs.String("log-level", "info", "日志级别 (debug, info, warn, error)")
		logFile    = fs.String("log-file", "", "日志文件路径")
		noProgress = fs.Bool("no-progress", false, "禁用进度显示")
		quiet      = fs.Bool("quiet", false, "静默模式")
		verbose    = fs.Bool("verbose", false, "详细模式")
	)

	// 解析全局参数
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		// 忽略未知参数，让具体命令处理
	}

	// 加载配置文件
	if *configFile != "" {
		if err := app.config.LoadFromFile(*configFile); err != nil {
			return fmt.Errorf("加载配置文件失败: %w", err)
		}
	}

	// 应用命令行参数覆盖
	if *logLevel != "info" {
		app.config.LogLevel = *logLevel
	}
	if *logFile != "" {
		app.config.LogFile = *logFile
	}
	if *noProgress {
		app.config.ShowProgress = false
	}
	if *quiet {
		app.config.LogLevel = "error"
		app.config.ShowProgress = false
	}
	if *verbose {
		app.config.LogLevel = "debug"
	}

	// 重新初始化日志器和进度管理器
	app.logger = NewLogger(app.config.LogLevel, app.config.LogFile)
	app.progress = NewProgressManager(app.config.ShowProgress)

	return nil
}

// showHelp 显示帮助信息
func (app *App) showHelp() error {
	fmt.Printf("%s - %s\n\n", app.name, app.description)
	fmt.Printf("版本: %s\n\n", app.version)

	fmt.Printf("用法:\n")
	fmt.Printf("  %s [全局选项] <命令> [命令选项] [参数...]\n\n", app.name)

	fmt.Printf("全局选项:\n")
	fmt.Printf("  --config <file>     配置文件路径\n")
	fmt.Printf("  --log-level <level> 日志级别 (debug, info, warn, error)\n")
	fmt.Printf("  --log-file <file>   日志文件路径\n")
	fmt.Printf("  --no-progress       禁用进度显示\n")
	fmt.Printf("  --quiet             静默模式\n")
	fmt.Printf("  --verbose           详细模式\n")
	fmt.Printf("  --help              显示帮助信息\n")
	fmt.Printf("  --version           显示版本信息\n\n")

	fmt.Printf("可用命令:\n")
	commands := app.registry.List()
	for _, cmd := range commands {
		fmt.Printf("  %-12s %s\n", cmd.Name(), cmd.Description())
	}

	fmt.Printf("\n使用 '%s <命令> --help' 查看具体命令的帮助信息\n", app.name)

	return nil
}

// showVersion 显示版本信息
func (app *App) showVersion() error {
	fmt.Printf("%s version %s\n", app.name, app.version)
	return nil
}

// GetName 获取应用程序名称
func (app *App) GetName() string {
	return app.name
}

// GetVersion 获取应用程序版本
func (app *App) GetVersion() string {
	return app.version
}

// GetLogger 获取日志器
func (app *App) GetLogger() *Logger {
	return app.logger
}

// GetConfig 获取配置
func (app *App) GetConfig() *Config {
	return app.config
}

// GetProgress 获取进度管理器
func (app *App) GetProgress() *ProgressManager {
	return app.progress
}

// GetEngine 获取引擎
func (app *App) GetEngine() Engine {
	return app.engine
}

// SetEngine 设置引擎
func (app *App) SetEngine(engine Engine) {
	app.engine = engine
}

// HelpCommand 帮助命令
type HelpCommand struct {
	app *App
}

// NewHelpCommand 创建帮助命令
func NewHelpCommand(app *App) *HelpCommand {
	return &HelpCommand{app: app}
}

func (c *HelpCommand) Name() string {
	return "help"
}

func (c *HelpCommand) Description() string {
	return "显示帮助信息"
}

func (c *HelpCommand) Usage() string {
	return "hexdiff help [command]"
}

func (c *HelpCommand) SetFlags(fs *flag.FlagSet) {
	// 帮助命令不需要额外标志
}

func (c *HelpCommand) Execute(args []string) error {
	if len(args) == 0 {
		return c.app.showHelp()
	}

	// 显示特定命令的帮助
	cmdName := args[0]
	cmd, exists := c.app.registry.Get(cmdName)
	if !exists {
		return fmt.Errorf("未知命令: %s", cmdName)
	}

	fmt.Printf("命令: %s\n\n", cmd.Name())
	fmt.Printf("描述: %s\n\n", cmd.Description())
	fmt.Printf("用法: %s\n\n", cmd.Usage())

	// 创建临时标志集来显示选项
	fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	cmd.SetFlags(fs)

	fmt.Printf("选项:\n")
	fs.PrintDefaults()

	return nil
}

// VersionCommand 版本命令
type VersionCommand struct {
	app *App
}

// NewVersionCommand 创建版本命令
func NewVersionCommand(app *App) *VersionCommand {
	return &VersionCommand{app: app}
}

func (c *VersionCommand) Name() string {
	return "version"
}

func (c *VersionCommand) Description() string {
	return "显示版本信息"
}

func (c *VersionCommand) Usage() string {
	return "hexdiff version"
}

func (c *VersionCommand) SetFlags(fs *flag.FlagSet) {
	// 版本命令不需要额外标志
}

func (c *VersionCommand) Execute(args []string) error {
	return c.app.showVersion()
}

// BenchmarkCommand 性能测试命令
type BenchmarkCommand struct {
	app     *App
	testDir string
	cleanup bool
	verbose bool
}

// NewBenchmarkCommand 创建性能测试命令
func NewBenchmarkCommand(app *App) *BenchmarkCommand {
	return &BenchmarkCommand{
		app:     app,
		testDir: "./benchmark_test",
		cleanup: true,
	}
}

func (c *BenchmarkCommand) Name() string {
	return "benchmark"
}

func (c *BenchmarkCommand) Description() string {
	return "运行性能基准测试"
}

func (c *BenchmarkCommand) Usage() string {
	return "hexdiff benchmark [options]"
}

func (c *BenchmarkCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.testDir, "test-dir", "./benchmark_test", "测试目录")
	fs.BoolVar(&c.cleanup, "cleanup", true, "测试后清理文件")
	fs.BoolVar(&c.verbose, "v", false, "详细输出")
	fs.BoolVar(&c.verbose, "verbose", false, "详细输出")
}

func (c *BenchmarkCommand) Execute(args []string) error {
	c.app.logger.Info("开始性能基准测试...")
	c.app.logger.Info("测试目录: %s", c.testDir)

	// 这里应该调用性能测试模块
	// 由于性能测试模块在不同的包中，这里只是示例
	c.app.logger.Info("性能测试功能需要集成性能测试模块")
	c.app.logger.Info("请参考 pkg/performance/benchmark.go 中的实现")

	return nil
}

// ConfigCommand 配置管理命令
type ConfigCommand struct {
	app    *App
	action string
	key    string
	value  string
}

// NewConfigCommand 创建配置管理命令
func NewConfigCommand(app *App) *ConfigCommand {
	return &ConfigCommand{
		app: app,
	}
}

func (c *ConfigCommand) Name() string {
	return "config"
}

func (c *ConfigCommand) Description() string {
	return "管理配置文件"
}

func (c *ConfigCommand) Usage() string {
	return "hexdiff config <action> [key] [value]"
}

func (c *ConfigCommand) SetFlags(fs *flag.FlagSet) {
	// 配置命令通过位置参数处理
}

func (c *ConfigCommand) Execute(args []string) error {
	if len(args) < 1 {
		return ErrInvalidArgumentf("缺少操作参数 (init, get, set, list)")
	}

	action := args[0]

	switch action {
	case "init":
		return c.initConfig()
	case "get":
		if len(args) < 2 {
			return ErrInvalidArgumentf("缺少配置键名")
		}
		return c.getConfig(args[1])
	case "set":
		if len(args) < 3 {
			return ErrInvalidArgumentf("缺少配置键名或值")
		}
		return c.setConfig(args[1], args[2])
	case "list":
		return c.listConfig()
	default:
		return ErrInvalidArgumentf("未知操作: %s", action)
	}
}

func (c *ConfigCommand) initConfig() error {
	configPath := GetConfigPath()

	if err := CreateDefaultConfigFile(); err != nil {
		return WrapError(ErrConfigInvalid, "创建配置文件失败", err)
	}

	c.app.logger.Success("配置文件已创建: %s", configPath)
	return nil
}

func (c *ConfigCommand) getConfig(key string) error {
	// 这里应该实现获取配置值的逻辑
	c.app.logger.Info("获取配置: %s", key)
	return nil
}

func (c *ConfigCommand) setConfig(key, value string) error {
	// 这里应该实现设置配置值的逻辑
	c.app.logger.Info("设置配置: %s = %s", key, value)
	return nil
}

func (c *ConfigCommand) listConfig() error {
	// 这里应该实现列出所有配置的逻辑
	c.app.logger.Info("当前配置:")
	c.app.logger.Info("  日志级别: %s", c.app.config.LogLevel)
	c.app.logger.Info("  显示进度: %t", c.app.config.ShowProgress)
	c.app.logger.Info("  块大小: %d", c.app.config.BlockSize)
	c.app.logger.Info("  最大内存: %d MB", c.app.config.MaxMemory)
	c.app.logger.Info("  工作协程数: %d", c.app.config.WorkerCount)
	return nil
}
