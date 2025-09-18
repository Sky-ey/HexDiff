package compression

import (
	"fmt"
	"time"
)

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	Algorithm          CompressionType  `json:"algorithm"`
	Level              CompressionLevel `json:"level"`
	OriginalSize       int64            `json:"original_size"`
	CompressedSize     int64            `json:"compressed_size"`
	CompressionTime    time.Duration    `json:"compression_time"`
	DecompressionTime  time.Duration    `json:"decompression_time"`
	CompressionRatio   float64          `json:"compression_ratio"`
	CompressionSpeed   float64          `json:"compression_speed"`   // MB/s
	DecompressionSpeed float64          `json:"decompression_speed"` // MB/s
}

// CalculateMetrics 计算性能指标
func (br *BenchmarkResult) CalculateMetrics() {
	if br.OriginalSize > 0 {
		br.CompressionRatio = float64(br.CompressedSize) / float64(br.OriginalSize)

		if br.CompressionTime > 0 {
			br.CompressionSpeed = float64(br.OriginalSize) / (1024 * 1024) / br.CompressionTime.Seconds()
		}

		if br.DecompressionTime > 0 {
			br.DecompressionSpeed = float64(br.OriginalSize) / (1024 * 1024) / br.DecompressionTime.Seconds()
		}
	}
}

// GetSavings 获取节省的空间百分比
func (br *BenchmarkResult) GetSavings() float64 {
	return (1.0 - br.CompressionRatio) * 100.0
}

// String 返回结果的字符串表示
func (br *BenchmarkResult) String() string {
	return fmt.Sprintf(
		"算法: %s, 级别: %d, 压缩比: %.2f%%, 压缩速度: %.2f MB/s, 解压速度: %.2f MB/s",
		br.Algorithm,
		br.Level,
		br.GetSavings(),
		br.CompressionSpeed,
		br.DecompressionSpeed,
	)
}

// CompressionBenchmark 压缩基准测试器
type CompressionBenchmark struct {
	manager *CompressionManager
}

// NewCompressionBenchmark 创建基准测试器
func NewCompressionBenchmark(manager *CompressionManager) *CompressionBenchmark {
	return &CompressionBenchmark{
		manager: manager,
	}
}

// BenchmarkAlgorithm 测试单个算法
func (cb *CompressionBenchmark) BenchmarkAlgorithm(data []byte, algorithm CompressionType, level CompressionLevel) (*BenchmarkResult, error) {
	// 获取压缩器和解压器
	compressor, err := cb.manager.GetCompressor(algorithm)
	if err != nil {
		return nil, err
	}

	decompressor, err := cb.manager.GetDecompressor(algorithm)
	if err != nil {
		return nil, err
	}

	result := &BenchmarkResult{
		Algorithm:    algorithm,
		Level:        level,
		OriginalSize: int64(len(data)),
	}

	// 测试压缩性能
	startTime := time.Now()
	compressed, err := compressor.Compress(data)
	if err != nil {
		return nil, fmt.Errorf("压缩失败: %w", err)
	}
	result.CompressionTime = time.Since(startTime)
	result.CompressedSize = int64(len(compressed))

	// 测试解压性能
	startTime = time.Now()
	decompressed, err := decompressor.Decompress(compressed)
	if err != nil {
		return nil, fmt.Errorf("解压失败: %w", err)
	}
	result.DecompressionTime = time.Since(startTime)

	// 验证数据完整性
	if len(decompressed) != len(data) {
		return nil, fmt.Errorf("解压后数据大小不匹配")
	}

	// 计算性能指标
	result.CalculateMetrics()

	return result, nil
}

// BenchmarkAllAlgorithms 测试所有算法
func (cb *CompressionBenchmark) BenchmarkAllAlgorithms(data []byte) ([]*BenchmarkResult, error) {
	algorithms := cb.manager.GetSupportedTypes()
	var results []*BenchmarkResult

	levels := []CompressionLevel{LevelFastest, LevelFast, LevelDefault, LevelBest}

	for _, algorithm := range algorithms {
		if algorithm == CompressionNone {
			continue
		}

		for _, level := range levels {
			result, err := cb.BenchmarkAlgorithm(data, algorithm, level)
			if err != nil {
				fmt.Printf("测试 %s (级别 %d) 失败: %v\n", algorithm, level, err)
				continue
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// FindBestAlgorithm 找到最佳算法
func (cb *CompressionBenchmark) FindBestAlgorithm(data []byte, prioritizeSpeed bool) (*BenchmarkResult, error) {
	results, err := cb.BenchmarkAllAlgorithms(data)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("没有可用的压缩算法")
	}

	var best *BenchmarkResult
	var bestScore float64

	for _, result := range results {
		var score float64
		if prioritizeSpeed {
			// 优先考虑速度：压缩速度权重更高
			score = result.CompressionSpeed*0.7 + (1.0-result.CompressionRatio)*100*0.3
		} else {
			// 优先考虑压缩率：压缩比权重更高
			score = (1.0-result.CompressionRatio)*100*0.7 + result.CompressionSpeed*0.3
		}

		if best == nil || score > bestScore {
			best = result
			bestScore = score
		}
	}

	return best, nil
}

// CompareAlgorithms 比较算法性能
func (cb *CompressionBenchmark) CompareAlgorithms(data []byte) (*ComparisonReport, error) {
	results, err := cb.BenchmarkAllAlgorithms(data)
	if err != nil {
		return nil, err
	}

	report := &ComparisonReport{
		DataSize: int64(len(data)),
		Results:  results,
	}

	// 找到最佳压缩比
	var bestRatio *BenchmarkResult
	for _, result := range results {
		if bestRatio == nil || result.CompressionRatio < bestRatio.CompressionRatio {
			bestRatio = result
		}
	}
	report.BestCompressionRatio = bestRatio

	// 找到最快压缩速度
	var fastestCompression *BenchmarkResult
	for _, result := range results {
		if fastestCompression == nil || result.CompressionSpeed > fastestCompression.CompressionSpeed {
			fastestCompression = result
		}
	}
	report.FastestCompression = fastestCompression

	// 找到最快解压速度
	var fastestDecompression *BenchmarkResult
	for _, result := range results {
		if fastestDecompression == nil || result.DecompressionSpeed > fastestDecompression.DecompressionSpeed {
			fastestDecompression = result
		}
	}
	report.FastestDecompression = fastestDecompression

	return report, nil
}

// ComparisonReport 比较报告
type ComparisonReport struct {
	DataSize             int64              `json:"data_size"`
	Results              []*BenchmarkResult `json:"results"`
	BestCompressionRatio *BenchmarkResult   `json:"best_compression_ratio"`
	FastestCompression   *BenchmarkResult   `json:"fastest_compression"`
	FastestDecompression *BenchmarkResult   `json:"fastest_decompression"`
}

// PrintReport 打印报告
func (cr *ComparisonReport) PrintReport() {
	fmt.Printf("=== 压缩算法性能比较报告 ===\n")
	fmt.Printf("数据大小: %d 字节 (%.2f MB)\n\n", cr.DataSize, float64(cr.DataSize)/(1024*1024))

	fmt.Printf("所有测试结果:\n")
	for _, result := range cr.Results {
		fmt.Printf("  %s\n", result.String())
	}

	fmt.Printf("\n最佳结果:\n")
	if cr.BestCompressionRatio != nil {
		fmt.Printf("  最佳压缩比: %s (%.2f%%)\n",
			cr.BestCompressionRatio.Algorithm,
			cr.BestCompressionRatio.GetSavings())
	}

	if cr.FastestCompression != nil {
		fmt.Printf("  最快压缩: %s (%.2f MB/s)\n",
			cr.FastestCompression.Algorithm,
			cr.FastestCompression.CompressionSpeed)
	}

	if cr.FastestDecompression != nil {
		fmt.Printf("  最快解压: %s (%.2f MB/s)\n",
			cr.FastestDecompression.Algorithm,
			cr.FastestDecompression.DecompressionSpeed)
	}
}
