package compression

import (
	"fmt"
	"io"
	"sync"
)

// CompressionManager 压缩管理器
type CompressionManager struct {
	compressors   map[CompressionType]Compressor
	decompressors map[CompressionType]Decompressor
	mutex         sync.RWMutex
	defaultType   CompressionType
}

// NewCompressionManager 创建压缩管理器
func NewCompressionManager() *CompressionManager {
	cm := &CompressionManager{
		compressors:   make(map[CompressionType]Compressor),
		decompressors: make(map[CompressionType]Decompressor),
		defaultType:   CompressionGzip,
	}

	// 注册默认压缩器
	cm.RegisterGzip(DefaultCompressionConfig())
	cm.RegisterLZ4(DefaultCompressionConfig())
	cm.RegisterZstd(DefaultCompressionConfig())

	return cm
}

// RegisterGzip 注册Gzip压缩器
func (cm *CompressionManager) RegisterGzip(config *CompressionConfig) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	config.Type = CompressionGzip
	cm.compressors[CompressionGzip] = NewGzipCompressor(config)
	cm.decompressors[CompressionGzip] = NewGzipDecompressor(config)
}

// RegisterLZ4 注册LZ4压缩器
func (cm *CompressionManager) RegisterLZ4(config *CompressionConfig) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	config.Type = CompressionLZ4
	cm.compressors[CompressionLZ4] = NewLZ4Compressor(config)
	cm.decompressors[CompressionLZ4] = NewLZ4Decompressor(config)
}

// RegisterZstd 注册Zstd压缩器
func (cm *CompressionManager) RegisterZstd(config *CompressionConfig) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	zstdConfig := ZstdConfig{
		Level:           config.Level,
		WindowSize:      1 << 20, // 1MB
		EnableChecksum:  true,
		EnableDict:      config.EnableDict,
		DictSize:        config.DictSize,
		ConcurrentLevel: 1,
	}

	cm.compressors[CompressionZstd] = NewZstdCompressor(zstdConfig)

	decompressConfig := ZstdDecompressConfig{
		MaxMemory:       128 * 1024 * 1024, // 128MB
		MaxWindowSize:   1 << 27,           // 128MB
		ConcurrentLevel: 1,
	}
	cm.decompressors[CompressionZstd] = NewZstdDecompressor(decompressConfig)
}

// RegisterCompressor 注册自定义压缩器
func (cm *CompressionManager) RegisterCompressor(compressor Compressor, decompressor Decompressor) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cType := compressor.GetType()
	cm.compressors[cType] = compressor
	cm.decompressors[cType] = decompressor
}

// GetCompressor 获取压缩器
func (cm *CompressionManager) GetCompressor(cType CompressionType) (Compressor, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	compressor, exists := cm.compressors[cType]
	if !exists {
		return nil, fmt.Errorf("不支持的压缩类型: %s", cType)
	}

	return compressor, nil
}

// GetDecompressor 获取解压器
func (cm *CompressionManager) GetDecompressor(cType CompressionType) (Decompressor, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	decompressor, exists := cm.decompressors[cType]
	if !exists {
		return nil, fmt.Errorf("不支持的解压类型: %s", cType)
	}

	return decompressor, nil
}

// SetDefaultType 设置默认压缩类型
func (cm *CompressionManager) SetDefaultType(cType CompressionType) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, exists := cm.compressors[cType]; !exists {
		return fmt.Errorf("压缩类型 %s 未注册", cType)
	}

	cm.defaultType = cType
	return nil
}

// GetDefaultType 获取默认压缩类型
func (cm *CompressionManager) GetDefaultType() CompressionType {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return cm.defaultType
}

// Compress 使用默认压缩器压缩数据
func (cm *CompressionManager) Compress(data []byte) ([]byte, CompressionType, error) {
	return cm.CompressWithType(data, cm.defaultType)
}

// CompressWithType 使用指定类型压缩数据
func (cm *CompressionManager) CompressWithType(data []byte, cType CompressionType) ([]byte, CompressionType, error) {
	compressor, err := cm.GetCompressor(cType)
	if err != nil {
		return nil, CompressionNone, err
	}

	compressed, err := compressor.Compress(data)
	if err != nil {
		return nil, CompressionNone, err
	}

	return compressed, cType, nil
}

// CompressStream 流式压缩
func (cm *CompressionManager) CompressStream(src io.Reader, dst io.Writer, cType CompressionType) error {
	compressor, err := cm.GetCompressor(cType)
	if err != nil {
		return err
	}

	return compressor.CompressStream(src, dst)
}

// Decompress 解压数据
func (cm *CompressionManager) Decompress(data []byte, cType CompressionType) ([]byte, error) {
	if cType == CompressionNone {
		return data, nil
	}

	decompressor, err := cm.GetDecompressor(cType)
	if err != nil {
		return nil, err
	}

	return decompressor.Decompress(data)
}

// DecompressStream 流式解压
func (cm *CompressionManager) DecompressStream(src io.Reader, dst io.Writer, cType CompressionType) error {
	if cType == CompressionNone {
		_, err := io.Copy(dst, src)
		return err
	}

	decompressor, err := cm.GetDecompressor(cType)
	if err != nil {
		return err
	}

	return decompressor.DecompressStream(src, dst)
}

// ValidateCompressedData 验证压缩数据
func (cm *CompressionManager) ValidateCompressedData(data []byte, cType CompressionType) error {
	if cType == CompressionNone {
		return nil
	}

	decompressor, err := cm.GetDecompressor(cType)
	if err != nil {
		return err
	}

	return decompressor.ValidateData(data)
}

// GetSupportedTypes 获取支持的压缩类型
func (cm *CompressionManager) GetSupportedTypes() []CompressionType {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	types := make([]CompressionType, 0, len(cm.compressors))
	for cType := range cm.compressors {
		types = append(types, cType)
	}

	return types
}

// EstimateCompressedSize 估算压缩后大小
func (cm *CompressionManager) EstimateCompressedSize(originalSize int64, cType CompressionType) (int64, error) {
	if cType == CompressionNone {
		return originalSize, nil
	}

	// 简化估算，根据压缩类型返回估算值
	switch cType {
	case CompressionGzip:
		return int64(float64(originalSize) * 0.6), nil
	case CompressionLZ4:
		return int64(float64(originalSize) * 0.65), nil
	case CompressionZstd:
		return int64(float64(originalSize) * 0.5), nil
	default:
		return originalSize, nil
	}
}

// CompareCompressionEfficiency 比较不同压缩算法的效率
func (cm *CompressionManager) CompareCompressionEfficiency(data []byte) ([]*CompressionStats, error) {
	types := cm.GetSupportedTypes()
	results := make([]*CompressionStats, 0, len(types))

	for _, cType := range types {
		if cType == CompressionNone {
			continue
		}

		compressor, err := cm.GetCompressor(cType)
		if err != nil {
			continue
		}

		// 检查是否支持统计功能
		if statsCompressor, ok := compressor.(interface {
			CompressWithStats([]byte) ([]byte, *CompressionStats, error)
		}); ok {
			_, stats, err := statsCompressor.CompressWithStats(data)
			if err == nil {
				results = append(results, stats)
			}
		}
	}

	return results, nil
}

// GetBestCompressionType 获取最佳压缩类型
func (cm *CompressionManager) GetBestCompressionType(data []byte, prioritizeSpeed bool) (CompressionType, error) {
	stats, err := cm.CompareCompressionEfficiency(data)
	if err != nil {
		return CompressionNone, err
	}

	if len(stats) == 0 {
		return cm.defaultType, nil
	}

	var bestType CompressionType
	var bestScore float64

	for _, stat := range stats {
		var score float64
		if prioritizeSpeed {
			// 优先考虑速度：压缩时间权重更高
			score = (1.0-stat.CompressionRatio)*0.3 + (1.0/float64(stat.CompressionTime))*0.7
		} else {
			// 优先考虑压缩率：压缩比权重更高
			score = (1.0-stat.CompressionRatio)*0.7 + (1.0/float64(stat.CompressionTime))*0.3
		}

		if score > bestScore {
			bestScore = score
			bestType = stat.Algorithm
		}
	}

	return bestType, nil
}
