package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config 应用程序配置
type Config struct {
	// 日志配置
	LogLevel     string `json:"log_level"`     // 日志级别
	LogFile      string `json:"log_file"`      // 日志文件路径
	ShowProgress bool   `json:"show_progress"` // 是否显示进度条

	// 性能配置
	BlockSize   int  `json:"block_size"`   // 默认块大小
	MaxMemory   int  `json:"max_memory"`   // 最大内存使用量(MB)
	WorkerCount int  `json:"worker_count"` // 工作协程数
	EnableMmap  bool `json:"enable_mmap"`  // 是否启用内存映射
	EnableCache bool `json:"enable_cache"` // 是否启用缓存
	CacheSize   int  `json:"cache_size"`   // 缓存大小(MB)

	// 完整性配置
	EnableIntegrity bool   `json:"enable_integrity"` // 是否启用完整性检查
	EnableBackup    bool   `json:"enable_backup"`    // 是否自动创建备份
	BackupDir       string `json:"backup_dir"`       // 备份目录

	// 压缩配置
	DefaultCompression string `json:"default_compression"` // 默认压缩算法
	CompressionLevel   int    `json:"compression_level"`   // 压缩级别

	// 输出配置
	OutputFormat string `json:"output_format"` // 输出格式 (text, json)
	Quiet        bool   `json:"quiet"`         // 静默模式
	Verbose      bool   `json:"verbose"`       // 详细模式
}

// NewConfig 创建默认配置
func NewConfig() *Config {
	return &Config{
		// 日志配置
		LogLevel:     "info",
		LogFile:      "",
		ShowProgress: true,

		// 性能配置
		BlockSize:   4096,
		MaxMemory:   512,
		WorkerCount: 4,
		EnableMmap:  true,
		EnableCache: true,
		CacheSize:   64,

		// 完整性配置
		EnableIntegrity: true,
		EnableBackup:    true,
		BackupDir:       ".hexdiff_backup",

		// 压缩配置
		DefaultCompression: "gzip",
		CompressionLevel:   6,

		// 输出配置
		OutputFormat: "text",
		Quiet:        false,
		Verbose:      false,
	}
}

// LoadFromFile 从文件加载配置
func (c *Config) LoadFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	return c.Validate()
}

// SaveToFile 保存配置到文件
func (c *Config) SaveToFile(filename string) error {
	// 确保目录存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 验证日志级别
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("无效的日志级别: %s", c.LogLevel)
	}

	// 验证数值范围
	if c.BlockSize <= 0 {
		return fmt.Errorf("块大小必须大于0: %d", c.BlockSize)
	}
	if c.MaxMemory <= 0 {
		return fmt.Errorf("最大内存必须大于0: %d", c.MaxMemory)
	}
	if c.WorkerCount <= 0 {
		return fmt.Errorf("工作协程数必须大于0: %d", c.WorkerCount)
	}
	if c.CacheSize < 0 {
		return fmt.Errorf("缓存大小不能为负数: %d", c.CacheSize)
	}

	// 验证压缩算法
	validCompressions := map[string]bool{
		"none": true, "gzip": true, "lz4": true,
	}
	if !validCompressions[c.DefaultCompression] {
		return fmt.Errorf("无效的压缩算法: %s", c.DefaultCompression)
	}

	// 验证压缩级别
	if c.CompressionLevel < 0 || c.CompressionLevel > 9 {
		return fmt.Errorf("压缩级别必须在0-9之间: %d", c.CompressionLevel)
	}

	// 验证输出格式
	validFormats := map[string]bool{
		"text": true, "json": true,
	}
	if !validFormats[c.OutputFormat] {
		return fmt.Errorf("无效的输出格式: %s", c.OutputFormat)
	}

	return nil
}

// GetConfigPath 获取默认配置文件路径
func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".hexdiff.json"
	}
	return filepath.Join(homeDir, ".hexdiff", "config.json")
}

// LoadDefaultConfig 加载默认配置
func LoadDefaultConfig() *Config {
	config := NewConfig()

	// 尝试从默认位置加载配置
	configPath := GetConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		if err := config.LoadFromFile(configPath); err != nil {
			// 配置文件存在但加载失败，使用默认配置
			fmt.Fprintf(os.Stderr, "警告: 加载配置文件失败，使用默认配置: %v\n", err)
		}
	}

	return config
}

// CreateDefaultConfigFile 创建默认配置文件
func CreateDefaultConfigFile() error {
	config := NewConfig()
	configPath := GetConfigPath()

	// 检查文件是否已存在
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("配置文件已存在: %s", configPath)
	}

	return config.SaveToFile(configPath)
}
