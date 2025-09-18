package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PatchMetadata 补丁元数据
type PatchMetadata struct {
	// 基本信息
	Version     string    `json:"version"`     // 补丁版本
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	CreatedBy   string    `json:"created_by"`  // 创建者
	Description string    `json:"description"` // 描述信息

	// 文件信息
	SourceFile struct {
		Name     string `json:"name"`     // 源文件名
		Size     int64  `json:"size"`     // 文件大小
		Checksum string `json:"checksum"` // 校验和
		Path     string `json:"path"`     // 文件路径
	} `json:"source_file"`

	TargetFile struct {
		Name     string `json:"name"`     // 目标文件名
		Size     int64  `json:"size"`     // 文件大小
		Checksum string `json:"checksum"` // 校验和
		Path     string `json:"path"`     // 文件路径
	} `json:"target_file"`

	// 补丁信息
	PatchInfo struct {
		Size             int64   `json:"size"`              // 补丁大小
		CompressionType  string  `json:"compression_type"`  // 压缩类型
		CompressionRatio float64 `json:"compression_ratio"` // 压缩比
		OperationCount   int     `json:"operation_count"`   // 操作数量
		Algorithm        string  `json:"algorithm"`         // 差异算法
	} `json:"patch_info"`

	// 性能统计
	Performance struct {
		GenerationTime  int64   `json:"generation_time"`  // 生成耗时(毫秒)
		CompressionTime int64   `json:"compression_time"` // 压缩耗时(毫秒)
		MemoryUsage     int64   `json:"memory_usage"`     // 内存使用(字节)
		Throughput      float64 `json:"throughput"`       // 吞吐量(MB/s)
	} `json:"performance"`

	// 兼容性信息
	Compatibility struct {
		MinVersion   string   `json:"min_version"`  // 最小支持版本
		MaxVersion   string   `json:"max_version"`  // 最大支持版本
		Platforms    []string `json:"platforms"`    // 支持平台
		Dependencies []string `json:"dependencies"` // 依赖项
	} `json:"compatibility"`

	// 自定义属性
	CustomAttributes map[string]interface{} `json:"custom_attributes,omitempty"`

	// 验证信息
	Verification struct {
		Signature  string    `json:"signature"`   // 数字签名
		SignedBy   string    `json:"signed_by"`   // 签名者
		SignedAt   time.Time `json:"signed_at"`   // 签名时间
		Verified   bool      `json:"verified"`    // 是否已验证
		VerifiedAt time.Time `json:"verified_at"` // 验证时间
	} `json:"verification"`
}

// MetadataManager 元数据管理器
type MetadataManager struct {
	metadataDir string // 元数据存储目录
}

// NewMetadataManager 创建元数据管理器
func NewMetadataManager(metadataDir string) *MetadataManager {
	return &MetadataManager{
		metadataDir: metadataDir,
	}
}

// CreateMetadata 创建补丁元数据
func (mm *MetadataManager) CreateMetadata(patchPath string) *PatchMetadata {
	metadata := &PatchMetadata{
		Version:          "1.0.0",
		CreatedAt:        time.Now(),
		CreatedBy:        getCurrentUser(),
		CustomAttributes: make(map[string]interface{}),
	}

	// 设置默认兼容性信息
	metadata.Compatibility.MinVersion = "1.0.0"
	metadata.Compatibility.MaxVersion = "2.0.0"
	metadata.Compatibility.Platforms = []string{"linux", "darwin", "windows"}

	return metadata
}

// SaveMetadata 保存元数据到文件
func (mm *MetadataManager) SaveMetadata(patchPath string, metadata *PatchMetadata) error {
	// 确保元数据目录存在
	if err := os.MkdirAll(mm.metadataDir, 0755); err != nil {
		return fmt.Errorf("创建元数据目录失败: %w", err)
	}

	// 生成元数据文件路径
	metadataPath := mm.getMetadataPath(patchPath)

	// 序列化元数据
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %w", err)
	}

	// 写入文件
	err = os.WriteFile(metadataPath, data, 0644)
	if err != nil {
		return fmt.Errorf("写入元数据文件失败: %w", err)
	}

	return nil
}

// LoadMetadata 从文件加载元数据
func (mm *MetadataManager) LoadMetadata(patchPath string) (*PatchMetadata, error) {
	metadataPath := mm.getMetadataPath(patchPath)

	// 检查文件是否存在
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("元数据文件不存在: %s", metadataPath)
	}

	// 读取文件
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("读取元数据文件失败: %w", err)
	}

	// 反序列化
	var metadata PatchMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, fmt.Errorf("解析元数据失败: %w", err)
	}

	return &metadata, nil
}

// UpdateMetadata 更新元数据
func (mm *MetadataManager) UpdateMetadata(patchPath string, updater func(*PatchMetadata)) error {
	// 加载现有元数据
	metadata, err := mm.LoadMetadata(patchPath)
	if err != nil {
		// 如果不存在，创建新的
		metadata = mm.CreateMetadata(patchPath)
	}

	// 应用更新
	updater(metadata)

	// 保存更新后的元数据
	return mm.SaveMetadata(patchPath, metadata)
}

// DeleteMetadata 删除元数据
func (mm *MetadataManager) DeleteMetadata(patchPath string) error {
	metadataPath := mm.getMetadataPath(patchPath)

	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return nil // 文件不存在，认为删除成功
	}

	return os.Remove(metadataPath)
}

// ListMetadata 列出所有元数据
func (mm *MetadataManager) ListMetadata() ([]string, error) {
	if _, err := os.Stat(mm.metadataDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(mm.metadataDir)
	if err != nil {
		return nil, fmt.Errorf("读取元数据目录失败: %w", err)
	}

	var metadataFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			metadataFiles = append(metadataFiles, entry.Name())
		}
	}

	return metadataFiles, nil
}

// ValidateMetadata 验证元数据完整性
func (mm *MetadataManager) ValidateMetadata(metadata *PatchMetadata) []string {
	var issues []string

	// 检查必填字段
	if metadata.Version == "" {
		issues = append(issues, "版本信息缺失")
	}

	if metadata.SourceFile.Name == "" {
		issues = append(issues, "源文件名缺失")
	}

	if metadata.TargetFile.Name == "" {
		issues = append(issues, "目标文件名缺失")
	}

	if metadata.PatchInfo.Size <= 0 {
		issues = append(issues, "补丁大小无效")
	}

	// 检查时间字段
	if metadata.CreatedAt.IsZero() {
		issues = append(issues, "创建时间缺失")
	}

	// 检查校验和格式
	if len(metadata.SourceFile.Checksum) != 64 && len(metadata.SourceFile.Checksum) != 0 {
		issues = append(issues, "源文件校验和格式无效")
	}

	if len(metadata.TargetFile.Checksum) != 64 && len(metadata.TargetFile.Checksum) != 0 {
		issues = append(issues, "目标文件校验和格式无效")
	}

	return issues
}

// GetMetadataStats 获取元数据统计信息
func (mm *MetadataManager) GetMetadataStats() (*MetadataStats, error) {
	files, err := mm.ListMetadata()
	if err != nil {
		return nil, err
	}

	stats := &MetadataStats{
		TotalCount:       len(files),
		CreatedToday:     0,
		CreatedThisWeek:  0,
		CreatedThisMonth: 0,
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, -1, 0)

	for _, file := range files {
		patchPath := filepath.Join(mm.metadataDir, file)
		metadata, err := mm.LoadMetadata(patchPath)
		if err != nil {
			continue
		}

		if metadata.CreatedAt.After(today) {
			stats.CreatedToday++
		}
		if metadata.CreatedAt.After(weekAgo) {
			stats.CreatedThisWeek++
		}
		if metadata.CreatedAt.After(monthAgo) {
			stats.CreatedThisMonth++
		}
	}

	return stats, nil
}

// MetadataStats 元数据统计信息
type MetadataStats struct {
	TotalCount       int `json:"total_count"`
	CreatedToday     int `json:"created_today"`
	CreatedThisWeek  int `json:"created_this_week"`
	CreatedThisMonth int `json:"created_this_month"`
}

// getMetadataPath 获取元数据文件路径
func (mm *MetadataManager) getMetadataPath(patchPath string) string {
	baseName := filepath.Base(patchPath)
	metadataName := baseName + ".meta.json"
	return filepath.Join(mm.metadataDir, metadataName)
}

// getCurrentUser 获取当前用户
func getCurrentUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "unknown"
}

// SetSourceFileInfo 设置源文件信息
func (metadata *PatchMetadata) SetSourceFileInfo(name, path string, size int64, checksum string) {
	metadata.SourceFile.Name = name
	metadata.SourceFile.Path = path
	metadata.SourceFile.Size = size
	metadata.SourceFile.Checksum = checksum
}

// SetTargetFileInfo 设置目标文件信息
func (metadata *PatchMetadata) SetTargetFileInfo(name, path string, size int64, checksum string) {
	metadata.TargetFile.Name = name
	metadata.TargetFile.Path = path
	metadata.TargetFile.Size = size
	metadata.TargetFile.Checksum = checksum
}

// SetPatchInfo 设置补丁信息
func (metadata *PatchMetadata) SetPatchInfo(size int64, compressionType string, compressionRatio float64, operationCount int, algorithm string) {
	metadata.PatchInfo.Size = size
	metadata.PatchInfo.CompressionType = compressionType
	metadata.PatchInfo.CompressionRatio = compressionRatio
	metadata.PatchInfo.OperationCount = operationCount
	metadata.PatchInfo.Algorithm = algorithm
}

// SetPerformanceInfo 设置性能信息
func (metadata *PatchMetadata) SetPerformanceInfo(generationTime, compressionTime, memoryUsage int64, throughput float64) {
	metadata.Performance.GenerationTime = generationTime
	metadata.Performance.CompressionTime = compressionTime
	metadata.Performance.MemoryUsage = memoryUsage
	metadata.Performance.Throughput = throughput
}

// AddCustomAttribute 添加自定义属性
func (metadata *PatchMetadata) AddCustomAttribute(key string, value interface{}) {
	if metadata.CustomAttributes == nil {
		metadata.CustomAttributes = make(map[string]interface{})
	}
	metadata.CustomAttributes[key] = value
}

// GetCustomAttribute 获取自定义属性
func (metadata *PatchMetadata) GetCustomAttribute(key string) (interface{}, bool) {
	if metadata.CustomAttributes == nil {
		return nil, false
	}
	value, exists := metadata.CustomAttributes[key]
	return value, exists
}
