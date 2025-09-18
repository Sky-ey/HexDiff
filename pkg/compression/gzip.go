package compression

import (
	"bytes"
	"compress/gzip"
	"io"
	"time"
)

// GzipCompressor Gzip压缩器
type GzipCompressor struct {
	config *CompressionConfig
}

// NewGzipCompressor 创建Gzip压缩器
func NewGzipCompressor(config *CompressionConfig) *GzipCompressor {
	if config == nil {
		config = DefaultCompressionConfig()
		config.Type = CompressionGzip
	}
	return &GzipCompressor{
		config: config,
	}
}

// Compress 压缩数据
func (gc *GzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	writer, err := gzip.NewWriterLevel(&buf, int(gc.config.Level))
	if err != nil {
		return nil, NewCompressionError(CompressionGzip, "创建gzip writer失败", err)
	}
	defer writer.Close()

	_, err = writer.Write(data)
	if err != nil {
		return nil, NewCompressionError(CompressionGzip, "写入数据失败", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, NewCompressionError(CompressionGzip, "关闭writer失败", err)
	}

	return buf.Bytes(), nil
}

// CompressStream 流式压缩
func (gc *GzipCompressor) CompressStream(src io.Reader, dst io.Writer) error {
	writer, err := gzip.NewWriterLevel(dst, int(gc.config.Level))
	if err != nil {
		return NewCompressionError(CompressionGzip, "创建gzip writer失败", err)
	}
	defer writer.Close()

	buffer := make([]byte, gc.config.BlockSize)
	for {
		n, err := src.Read(buffer)
		if n > 0 {
			_, writeErr := writer.Write(buffer[:n])
			if writeErr != nil {
				return NewCompressionError(CompressionGzip, "写入压缩数据失败", writeErr)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return NewCompressionError(CompressionGzip, "读取源数据失败", err)
		}
	}

	return writer.Close()
}

// GetType 获取压缩类型
func (gc *GzipCompressor) GetType() CompressionType {
	return CompressionGzip
}

// GetConfig 获取压缩配置
func (gc *GzipCompressor) GetConfig() interface{} {
	return gc.config
}

// GetCompressionRatio 获取压缩比
func (gc *GzipCompressor) GetCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize)
}

// EstimateSize 估算压缩后大小
func (gc *GzipCompressor) EstimateSize(originalSize int64) int64 {
	// Gzip压缩比通常在30%-70%之间，这里使用保守估计
	ratio := 0.6
	switch gc.config.Level {
	case LevelFastest:
		ratio = 0.7
	case LevelFast:
		ratio = 0.65
	case LevelDefault:
		ratio = 0.6
	case LevelBest:
		ratio = 0.5
	case LevelMax:
		ratio = 0.45
	}

	return int64(float64(originalSize) * ratio)
}

// GzipDecompressor Gzip解压器
type GzipDecompressor struct {
	config *CompressionConfig
}

// NewGzipDecompressor 创建Gzip解压器
func NewGzipDecompressor(config *CompressionConfig) *GzipDecompressor {
	if config == nil {
		config = DefaultCompressionConfig()
		config.Type = CompressionGzip
	}
	return &GzipDecompressor{
		config: config,
	}
}

// Decompress 解压数据
func (gd *GzipDecompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, NewCompressionError(CompressionGzip, "创建gzip reader失败", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, reader)
	if err != nil {
		return nil, NewCompressionError(CompressionGzip, "解压数据失败", err)
	}

	return buf.Bytes(), nil
}

// DecompressStream 流式解压
func (gd *GzipDecompressor) DecompressStream(src io.Reader, dst io.Writer) error {
	reader, err := gzip.NewReader(src)
	if err != nil {
		return NewCompressionError(CompressionGzip, "创建gzip reader失败", err)
	}
	defer reader.Close()

	buffer := make([]byte, gd.config.BlockSize)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			_, writeErr := dst.Write(buffer[:n])
			if writeErr != nil {
				return NewCompressionError(CompressionGzip, "写入解压数据失败", writeErr)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return NewCompressionError(CompressionGzip, "读取压缩数据失败", err)
		}
	}

	return nil
}

// GetType 获取压缩类型
func (gd *GzipDecompressor) GetType() CompressionType {
	return CompressionGzip
}

// GetConfig 获取配置
func (gd *GzipDecompressor) GetConfig() interface{} {
	return gd.config
}

// ValidateData 验证压缩数据
func (gd *GzipDecompressor) ValidateData(data []byte) error {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return NewCompressionError(CompressionGzip, "无效的gzip数据", err)
	}
	defer reader.Close()

	// 尝试读取一小部分数据来验证
	buffer := make([]byte, 1024)
	_, err = reader.Read(buffer)
	if err != nil && err != io.EOF {
		return NewCompressionError(CompressionGzip, "gzip数据损坏", err)
	}

	return nil
}

// CompressWithStats 带统计的压缩
func (gc *GzipCompressor) CompressWithStats(data []byte) ([]byte, *CompressionStats, error) {
	startTime := time.Now()

	compressed, err := gc.Compress(data)
	if err != nil {
		return nil, nil, err
	}

	duration := time.Since(startTime)

	stats := &CompressionStats{
		OriginalSize:    int64(len(data)),
		CompressedSize:  int64(len(compressed)),
		CompressionTime: duration.Nanoseconds(),
		Algorithm:       CompressionGzip,
		Level:           gc.config.Level,
	}
	stats.CalculateRatio()

	return compressed, stats, nil
}
