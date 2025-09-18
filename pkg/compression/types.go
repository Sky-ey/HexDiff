package compression

import (
	"fmt"
	"io"
)

// CompressionType 压缩类型
type CompressionType uint8

const (
	CompressionNone CompressionType = iota // 无压缩
	CompressionGzip                        // Gzip压缩
	CompressionLZ4                         // LZ4压缩
	CompressionZstd                        // Zstandard压缩
)

// String 返回压缩类型的字符串表示
func (ct CompressionType) String() string {
	switch ct {
	case CompressionNone:
		return "None"
	case CompressionGzip:
		return "Gzip"
	case CompressionLZ4:
		return "LZ4"
	case CompressionZstd:
		return "Zstd"
	default:
		return fmt.Sprintf("Unknown(%d)", ct)
	}
}

// CompressionLevel 压缩级别
type CompressionLevel int

const (
	LevelFastest CompressionLevel = 1  // 最快压缩
	LevelFast    CompressionLevel = 3  // 快速压缩
	LevelDefault CompressionLevel = 6  // 默认压缩
	LevelBest    CompressionLevel = 9  // 最佳压缩
	LevelMax     CompressionLevel = 11 // 最大压缩
)

// CompressionConfig 压缩配置
type CompressionConfig struct {
	Type         CompressionType  // 压缩类型
	Level        CompressionLevel // 压缩级别
	BlockSize    int              // 块大小
	EnableDict   bool             // 是否启用字典压缩
	DictSize     int              // 字典大小
	EnableStream bool             // 是否启用流式压缩
}

// DefaultCompressionConfig 默认压缩配置
func DefaultCompressionConfig() *CompressionConfig {
	return &CompressionConfig{
		Type:         CompressionGzip,
		Level:        LevelDefault,
		BlockSize:    64 * 1024, // 64KB
		EnableDict:   false,
		DictSize:     32 * 1024, // 32KB
		EnableStream: true,
	}
}

// Compressor 压缩器接口
type Compressor interface {
	// Compress 压缩数据
	Compress(data []byte) ([]byte, error)

	// CompressStream 流式压缩
	CompressStream(src io.Reader, dst io.Writer) error

	// GetType 获取压缩类型
	GetType() CompressionType

	// GetConfig 获取压缩配置
	GetConfig() interface{}

	// GetCompressionRatio 获取压缩比
	GetCompressionRatio(originalSize, compressedSize int64) float64
}

// Decompressor 解压器接口
type Decompressor interface {
	// Decompress 解压数据
	Decompress(data []byte) ([]byte, error)

	// DecompressStream 流式解压
	DecompressStream(src io.Reader, dst io.Writer) error

	// GetType 获取压缩类型
	GetType() CompressionType

	// GetConfig 获取配置
	GetConfig() interface{}

	// ValidateData 验证压缩数据
	ValidateData(data []byte) error
}

// CompressionStats 压缩统计
type CompressionStats struct {
	OriginalSize     int64            // 原始大小
	CompressedSize   int64            // 压缩后大小
	CompressionTime  int64            // 压缩耗时(纳秒)
	CompressionRatio float64          // 压缩比
	Algorithm        CompressionType  // 压缩算法
	Level            CompressionLevel // 压缩级别
}

// CalculateRatio 计算压缩比
func (cs *CompressionStats) CalculateRatio() {
	if cs.OriginalSize > 0 {
		cs.CompressionRatio = float64(cs.CompressedSize) / float64(cs.OriginalSize)
	}
}

// GetSavings 获取节省的空间百分比
func (cs *CompressionStats) GetSavings() float64 {
	return (1.0 - cs.CompressionRatio) * 100.0
}

// CompressionError 压缩错误
type CompressionError struct {
	Type    CompressionType
	Message string
	Cause   error
}

func (e *CompressionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("compression error (%s): %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("compression error (%s): %s", e.Type, e.Message)
}

// NewCompressionError 创建压缩错误
func NewCompressionError(cType CompressionType, message string, cause error) *CompressionError {
	return &CompressionError{
		Type:    cType,
		Message: message,
		Cause:   cause,
	}
}
