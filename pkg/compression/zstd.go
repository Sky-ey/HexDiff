package compression

import (
	"bytes"
	"io"

	"github.com/klauspost/compress/zstd"
)

// ZstdCompressor Zstd压缩器
type ZstdCompressor struct {
	config ZstdConfig
}

// ZstdConfig Zstd压缩配置
type ZstdConfig struct {
	Level           CompressionLevel `json:"level"`            // 压缩级别
	WindowSize      int              `json:"window_size"`      // 窗口大小
	EnableChecksum  bool             `json:"enable_checksum"`  // 启用校验和
	EnableDict      bool             `json:"enable_dict"`      // 启用字典
	DictSize        int              `json:"dict_size"`        // 字典大小
	ConcurrentLevel int              `json:"concurrent_level"` // 并发级别
}

// NewZstdCompressor 创建Zstd压缩器
func NewZstdCompressor(config ZstdConfig) *ZstdCompressor {
	// 设置默认值
	if config.WindowSize == 0 {
		config.WindowSize = 1 << 20 // 1MB
	}
	if config.DictSize == 0 {
		config.DictSize = 64 * 1024 // 64KB
	}
	if config.ConcurrentLevel == 0 {
		config.ConcurrentLevel = 1
	}

	return &ZstdCompressor{
		config: config,
	}
}

// Compress 压缩数据
func (zc *ZstdCompressor) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	// 创建编码器选项
	var options []zstd.EOption

	// 设置压缩级别
	switch zc.config.Level {
	case LevelFastest:
		options = append(options, zstd.WithEncoderLevel(zstd.SpeedFastest))
	case LevelFast:
		options = append(options, zstd.WithEncoderLevel(zstd.SpeedDefault))
	case LevelDefault:
		options = append(options, zstd.WithEncoderLevel(zstd.SpeedDefault))
	case LevelBest:
		options = append(options, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	case LevelMax:
		options = append(options, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	}

	// 设置窗口大小
	if zc.config.WindowSize > 0 {
		options = append(options, zstd.WithWindowSize(zc.config.WindowSize))
	}

	// 启用校验和
	if zc.config.EnableChecksum {
		options = append(options, zstd.WithEncoderCRC(true))
	}

	// 设置并发级别
	if zc.config.ConcurrentLevel > 1 {
		options = append(options, zstd.WithEncoderConcurrency(zc.config.ConcurrentLevel))
	}

	// 创建编码器
	encoder, err := zstd.NewWriter(nil, options...)
	if err != nil {
		return nil, NewCompressionError(CompressionZstd, "创建zstd编码器失败", err)
	}
	defer encoder.Close()

	// 压缩数据
	compressed := encoder.EncodeAll(data, make([]byte, 0, len(data)))

	return compressed, nil
}

// CompressStream 流式压缩
func (zc *ZstdCompressor) CompressStream(reader io.Reader, writer io.Writer) error {
	// 创建编码器选项
	var options []zstd.EOption

	// 设置压缩级别
	switch zc.config.Level {
	case LevelFastest:
		options = append(options, zstd.WithEncoderLevel(zstd.SpeedFastest))
	case LevelFast:
		options = append(options, zstd.WithEncoderLevel(zstd.SpeedDefault))
	case LevelDefault:
		options = append(options, zstd.WithEncoderLevel(zstd.SpeedDefault))
	case LevelBest:
		options = append(options, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	case LevelMax:
		options = append(options, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	}

	// 设置窗口大小
	if zc.config.WindowSize > 0 {
		options = append(options, zstd.WithWindowSize(zc.config.WindowSize))
	}

	// 启用校验和
	if zc.config.EnableChecksum {
		options = append(options, zstd.WithEncoderCRC(true))
	}

	// 设置并发级别
	if zc.config.ConcurrentLevel > 1 {
		options = append(options, zstd.WithEncoderConcurrency(zc.config.ConcurrentLevel))
	}

	// 创建编码器
	encoder, err := zstd.NewWriter(writer, options...)
	if err != nil {
		return NewCompressionError(CompressionZstd, "创建zstd编码器失败", err)
	}
	defer encoder.Close()

	// 流式压缩
	_, err = io.Copy(encoder, reader)
	if err != nil {
		return NewCompressionError(CompressionZstd, "流式压缩失败", err)
	}

	// 确保所有数据都被写入
	err = encoder.Close()
	if err != nil {
		return NewCompressionError(CompressionZstd, "关闭编码器失败", err)
	}

	return nil
}

// GetCompressionRatio 获取压缩比
func (zc *ZstdCompressor) GetCompressionRatio(originalSize, compressedSize int64) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize)
}

// GetType 获取压缩类型
func (zc *ZstdCompressor) GetType() CompressionType {
	return CompressionZstd
}

// GetConfig 获取配置
func (zc *ZstdCompressor) GetConfig() any {
	return zc.config
}

// ZstdDecompressor Zstd解压器
type ZstdDecompressor struct {
	config ZstdDecompressConfig
}

// ZstdDecompressConfig Zstd解压配置
type ZstdDecompressConfig struct {
	MaxMemory       int64 `json:"max_memory"`       // 最大内存使用
	MaxWindowSize   int   `json:"max_window_size"`  // 最大窗口大小
	ConcurrentLevel int   `json:"concurrent_level"` // 并发级别
}

// NewZstdDecompressor 创建Zstd解压器
func NewZstdDecompressor(config ZstdDecompressConfig) *ZstdDecompressor {
	// 设置默认值
	if config.MaxMemory == 0 {
		config.MaxMemory = 128 * 1024 * 1024 // 128MB
	}
	if config.MaxWindowSize == 0 {
		config.MaxWindowSize = 1 << 27 // 128MB
	}
	if config.ConcurrentLevel == 0 {
		config.ConcurrentLevel = 1
	}

	return &ZstdDecompressor{
		config: config,
	}
}

// Decompress 解压数据
func (zd *ZstdDecompressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	// 创建解码器选项
	var options []zstd.DOption

	// 设置最大内存使用
	if zd.config.MaxMemory > 0 {
		options = append(options, zstd.WithDecoderMaxMemory(uint64(zd.config.MaxMemory)))
	}

	// 设置最大窗口大小
	if zd.config.MaxWindowSize > 0 {
		options = append(options, zstd.WithDecoderMaxWindow(uint64(zd.config.MaxWindowSize)))
	}

	// 设置并发级别
	if zd.config.ConcurrentLevel > 1 {
		options = append(options, zstd.WithDecoderConcurrency(zd.config.ConcurrentLevel))
	}

	// 创建解码器
	decoder, err := zstd.NewReader(nil, options...)
	if err != nil {
		return nil, NewCompressionError(CompressionZstd, "创建zstd解码器失败", err)
	}
	defer decoder.Close()

	// 解压数据
	decompressed, err := decoder.DecodeAll(data, nil)
	if err != nil {
		return nil, NewCompressionError(CompressionZstd, "解压数据失败", err)
	}

	return decompressed, nil
}

// DecompressStream 流式解压
func (zd *ZstdDecompressor) DecompressStream(reader io.Reader, writer io.Writer) error {
	// 创建解码器选项
	var options []zstd.DOption

	// 设置最大内存使用
	if zd.config.MaxMemory > 0 {
		options = append(options, zstd.WithDecoderMaxMemory(uint64(zd.config.MaxMemory)))
	}

	// 设置最大窗口大小
	if zd.config.MaxWindowSize > 0 {
		options = append(options, zstd.WithDecoderMaxWindow(uint64(zd.config.MaxWindowSize)))
	}

	// 设置并发级别
	if zd.config.ConcurrentLevel > 1 {
		options = append(options, zstd.WithDecoderConcurrency(zd.config.ConcurrentLevel))
	}

	// 创建解码器
	decoder, err := zstd.NewReader(reader, options...)
	if err != nil {
		return NewCompressionError(CompressionZstd, "创建zstd解码器失败", err)
	}
	defer decoder.Close()

	// 流式解压
	_, err = io.Copy(writer, decoder)
	if err != nil {
		return NewCompressionError(CompressionZstd, "流式解压失败", err)
	}

	return nil
}

// GetType 获取压缩类型
func (zd *ZstdDecompressor) GetType() CompressionType {
	return CompressionZstd
}

// GetConfig 获取配置
func (zd *ZstdDecompressor) GetConfig() any {
	return zd.config
}

// ValidateData 验证数据
func (zd *ZstdDecompressor) ValidateData(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// 尝试解压一小部分数据来验证格式
	decoder, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return NewCompressionError(CompressionZstd, "无效的zstd数据格式", err)
	}
	defer decoder.Close()

	// 读取少量数据进行验证
	buffer := make([]byte, 1024)
	_, err = decoder.Read(buffer)
	if err != nil && err != io.EOF {
		return NewCompressionError(CompressionZstd, "zstd数据验证失败", err)
	}

	return nil
}

// EstimateDecompressedSize 估算解压后大小
func (zd *ZstdDecompressor) EstimateDecompressedSize(data []byte) (int64, error) {
	if len(data) == 0 {
		return 0, nil
	}

	// Zstd格式包含原始大小信息，可以直接获取
	decoder, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return 0, NewCompressionError(CompressionZstd, "创建解码器失败", err)
	}
	defer decoder.Close()

	// 这里简化处理，实际可以通过解析帧头获取更准确的大小
	// 对于Zstd，可以通过帧头中的内容大小字段获取
	return int64(len(data) * 3), nil // 估算为压缩数据的3倍
}
