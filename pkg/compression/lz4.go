package compression

import (
	"io"
	"time"

	"github.com/pierrec/lz4/v4"
)

// LZ4Compressor LZ4压缩器
type LZ4Compressor struct {
	config *CompressionConfig
}

// NewLZ4Compressor 创建LZ4压缩器
func NewLZ4Compressor(config *CompressionConfig) *LZ4Compressor {
	if config == nil {
		config = DefaultCompressionConfig()
		config.Type = CompressionLZ4
	}
	return &LZ4Compressor{
		config: config,
	}
}

// Compress 压缩数据
func (lc *LZ4Compressor) Compress(data []byte) ([]byte, error) {
	// 使用LZ4块压缩
	compressed := make([]byte, lz4.CompressBlockBound(len(data)))

	compressedSize, err := lz4.CompressBlock(data, compressed, nil)
	if err != nil {
		return nil, NewCompressionError(CompressionLZ4, "LZ4压缩失败", err)
	}

	// 返回实际压缩的数据
	return compressed[:compressedSize], nil
}

// CompressStream 流式压缩
func (lc *LZ4Compressor) CompressStream(src io.Reader, dst io.Writer) error {
	writer := lz4.NewWriter(dst)
	defer writer.Close()

	buffer := make([]byte, lc.config.BlockSize)
	for {
		n, err := src.Read(buffer)
		if n > 0 {
			_, writeErr := writer.Write(buffer[:n])
			if writeErr != nil {
				return NewCompressionError(CompressionLZ4, "写入压缩数据失败", writeErr)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return NewCompressionError(CompressionLZ4, "读取源数据失败", err)
		}
	}

	return writer.Close()
}

// GetType 获取压缩类型
func (lc *LZ4Compressor) GetType() CompressionType {
	return CompressionLZ4
}

// GetConfig 获取压缩配置
func (lc *LZ4Compressor) GetConfig() interface{} {
	return lc.config
}

// GetCompressionRatio 获取压缩比
func (lc *LZ4Compressor) GetCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize)
}

// EstimateSize 估算压缩后大小
func (lc *LZ4Compressor) EstimateSize(originalSize int64) int64 {
	// LZ4压缩比通常在40%-80%之间，速度很快
	ratio := 0.65
	switch lc.config.Level {
	case LevelFastest:
		ratio = 0.8
	case LevelFast:
		ratio = 0.75
	case LevelDefault:
		ratio = 0.65
	case LevelBest:
		ratio = 0.55
	case LevelMax:
		ratio = 0.5
	}

	return int64(float64(originalSize) * ratio)
}

// LZ4Decompressor LZ4解压器
type LZ4Decompressor struct {
	config *CompressionConfig
}

// NewLZ4Decompressor 创建LZ4解压器
func NewLZ4Decompressor(config *CompressionConfig) *LZ4Decompressor {
	if config == nil {
		config = DefaultCompressionConfig()
		config.Type = CompressionLZ4
	}
	return &LZ4Decompressor{
		config: config,
	}
}

// Decompress 解压数据
func (ld *LZ4Decompressor) Decompress(data []byte) ([]byte, error) {
	// 估算解压后的大小（通常是压缩数据的2-4倍）
	decompressed := make([]byte, len(data)*4)

	decompressedSize, err := lz4.UncompressBlock(data, decompressed)
	if err != nil {
		return nil, NewCompressionError(CompressionLZ4, "LZ4解压失败", err)
	}

	return decompressed[:decompressedSize], nil
}

// DecompressStream 流式解压
func (ld *LZ4Decompressor) DecompressStream(src io.Reader, dst io.Writer) error {
	reader := lz4.NewReader(src)

	buffer := make([]byte, ld.config.BlockSize)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			_, writeErr := dst.Write(buffer[:n])
			if writeErr != nil {
				return NewCompressionError(CompressionLZ4, "写入解压数据失败", writeErr)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return NewCompressionError(CompressionLZ4, "读取压缩数据失败", err)
		}
	}

	return nil
}

// GetType 获取压缩类型
func (ld *LZ4Decompressor) GetType() CompressionType {
	return CompressionLZ4
}

// GetConfig 获取配置
func (ld *LZ4Decompressor) GetConfig() interface{} {
	return ld.config
}

// ValidateData 验证压缩数据
func (ld *LZ4Decompressor) ValidateData(data []byte) error {
	// 尝试解压少量数据来验证格式
	if len(data) == 0 {
		return nil
	}

	// 简单验证：尝试解压
	testBuffer := make([]byte, len(data)*2)
	_, err := lz4.UncompressBlock(data, testBuffer)
	if err != nil {
		return NewCompressionError(CompressionLZ4, "LZ4数据格式无效", err)
	}

	return nil
}

// CompressWithStats 带统计的压缩
func (lc *LZ4Compressor) CompressWithStats(data []byte) ([]byte, *CompressionStats, error) {
	startTime := time.Now()

	compressed, err := lc.Compress(data)
	if err != nil {
		return nil, nil, err
	}

	duration := time.Since(startTime)

	stats := &CompressionStats{
		OriginalSize:    int64(len(data)),
		CompressedSize:  int64(len(compressed)),
		CompressionTime: duration.Nanoseconds(),
		Algorithm:       CompressionLZ4,
		Level:           lc.config.Level,
	}
	stats.CalculateRatio()

	return compressed, stats, nil
}
