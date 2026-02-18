package performance

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// BenchmarkSuite 性能基准测试套件
type BenchmarkSuite struct {
	testDir     string
	testFiles   []string
	results     []BenchmarkResult
	ioOptimizer *IOOptimizer
	processor   *ConcurrentProcessor
	streamer    *StreamProcessor
}

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	TestName     string        // 测试名称
	FileSize     int64         // 文件大小
	Duration     time.Duration // 执行时间
	Throughput   float64       // 吞吐量 (MB/s)
	MemoryUsage  int64         // 内存使用量
	CPUUsage     float64       // CPU使用率
	IOOperations int64         // I/O操作数
	CacheHitRate float64       // 缓存命中率
	Success      bool          // 是否成功
	ErrorMessage string        // 错误信息
}

// NewBenchmarkSuite 创建基准测试套件
func NewBenchmarkSuite(testDir string) *BenchmarkSuite {
	return &BenchmarkSuite{
		testDir:     testDir,
		testFiles:   make([]string, 0),
		results:     make([]BenchmarkResult, 0),
		ioOptimizer: NewIOOptimizer(DefaultIOConfig()),
		processor:   NewConcurrentProcessor(DefaultConcurrentConfig()),
		streamer:    NewStreamProcessor(DefaultStreamConfig()),
	}
}

// PrepareTestFiles 准备测试文件
func (bs *BenchmarkSuite) PrepareTestFiles() error {
	// 确保测试目录存在
	if err := os.MkdirAll(bs.testDir, 0755); err != nil {
		return fmt.Errorf("创建测试目录失败: %w", err)
	}

	// 创建不同大小的测试文件
	fileSizes := []int64{
		1024,              // 1KB
		64 * 1024,         // 64KB
		1024 * 1024,       // 1MB
		10 * 1024 * 1024,  // 10MB
		100 * 1024 * 1024, // 100MB
	}

	for _, size := range fileSizes {
		fileName := fmt.Sprintf("test_%dMB.dat", size/(1024*1024))
		if size < 1024*1024 {
			fileName = fmt.Sprintf("test_%dKB.dat", size/1024)
		}

		filePath := filepath.Join(bs.testDir, fileName)

		if err := bs.createTestFile(filePath, size); err != nil {
			return fmt.Errorf("创建测试文件 %s 失败: %w", fileName, err)
		}

		bs.testFiles = append(bs.testFiles, filePath)
	}

	return nil
}

// createTestFile 创建指定大小的测试文件
func (bs *BenchmarkSuite) createTestFile(filePath string, size int64) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 生成测试数据
	buffer := make([]byte, 64*1024) // 64KB缓冲区
	for i := range buffer {
		buffer[i] = byte(i % 256)
	}

	written := int64(0)
	for written < size {
		toWrite := int64(len(buffer))
		if written+toWrite > size {
			toWrite = size - written
		}

		n, err := file.Write(buffer[:toWrite])
		if err != nil {
			return err
		}
		written += int64(n)
	}

	return file.Sync()
}

// RunIOBenchmarks 运行I/O基准测试
func (bs *BenchmarkSuite) RunIOBenchmarks() error {
	fmt.Println("开始I/O性能基准测试...")

	for _, filePath := range bs.testFiles {
		// 测试优化读取
		if err := bs.benchmarkOptimizedRead(filePath); err != nil {
			fmt.Printf("优化读取测试失败 %s: %v\n", filePath, err)
		}

		// 测试标准读取
		if err := bs.benchmarkStandardRead(filePath); err != nil {
			fmt.Printf("标准读取测试失败 %s: %v\n", filePath, err)
		}

		// 测试内存映射读取
		if err := bs.benchmarkMmapRead(filePath); err != nil {
			fmt.Printf("内存映射读取测试失败 %s: %v\n", filePath, err)
		}
	}

	return nil
}

// benchmarkOptimizedRead 基准测试优化读取
func (bs *BenchmarkSuite) benchmarkOptimizedRead(filePath string) error {
	startTime := time.Now()
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	reader, err := bs.ioOptimizer.NewOptimizedReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	buffer := make([]byte, 64*1024)
	totalBytes := int64(0)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			totalBytes += int64(n)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
	}

	duration := time.Since(startTime)
	runtime.ReadMemStats(&memAfter)

	// 获取I/O统计
	stats := bs.ioOptimizer.GetStats()

	result := BenchmarkResult{
		TestName:     fmt.Sprintf("OptimizedRead_%s", filepath.Base(filePath)),
		FileSize:     totalBytes,
		Duration:     duration,
		Throughput:   float64(totalBytes) / duration.Seconds() / (1024 * 1024), // MB/s
		MemoryUsage:  int64(memAfter.Alloc - memBefore.Alloc),
		IOOperations: stats.ReadOperations,
		CacheHitRate: float64(stats.CacheHits) / float64(stats.CacheHits+stats.CacheMisses) * 100,
		Success:      true,
	}

	bs.results = append(bs.results, result)
	return nil
}

// benchmarkStandardRead 基准测试标准读取
func (bs *BenchmarkSuite) benchmarkStandardRead(filePath string) error {
	startTime := time.Now()
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, 64*1024)
	totalBytes := int64(0)

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			totalBytes += int64(n)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
	}

	duration := time.Since(startTime)
	runtime.ReadMemStats(&memAfter)

	result := BenchmarkResult{
		TestName:    fmt.Sprintf("StandardRead_%s", filepath.Base(filePath)),
		FileSize:    totalBytes,
		Duration:    duration,
		Throughput:  float64(totalBytes) / duration.Seconds() / (1024 * 1024), // MB/s
		MemoryUsage: int64(memAfter.Alloc - memBefore.Alloc),
		Success:     true,
	}

	bs.results = append(bs.results, result)
	return nil
}

// benchmarkMmapRead 基准测试内存映射读取
func (bs *BenchmarkSuite) benchmarkMmapRead(filePath string) error {
	startTime := time.Now()
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// 创建启用内存映射的优化读取器
	config := DefaultIOConfig()
	config.EnableMmap = true
	optimizer := NewIOOptimizer(config)

	reader, err := optimizer.NewOptimizedReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	buffer := make([]byte, 64*1024)
	totalBytes := int64(0)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			totalBytes += int64(n)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
	}

	duration := time.Since(startTime)
	runtime.ReadMemStats(&memAfter)

	result := BenchmarkResult{
		TestName:    fmt.Sprintf("MmapRead_%s", filepath.Base(filePath)),
		FileSize:    totalBytes,
		Duration:    duration,
		Throughput:  float64(totalBytes) / duration.Seconds() / (1024 * 1024), // MB/s
		MemoryUsage: int64(memAfter.Alloc - memBefore.Alloc),
		Success:     true,
	}

	bs.results = append(bs.results, result)
	return nil
}

// RunConcurrentBenchmarks 运行并发基准测试
func (bs *BenchmarkSuite) RunConcurrentBenchmarks() error {
	fmt.Println("开始并发处理基准测试...")

	// 启动并发处理器
	bs.processor.Start()
	defer bs.processor.Stop()

	// 测试不同并发级别
	concurrencyLevels := []int{1, 2, 4, 8, 16}

	for _, level := range concurrencyLevels {
		if err := bs.benchmarkConcurrentProcessing(level); err != nil {
			fmt.Printf("并发级别 %d 测试失败: %v\n", level, err)
		}
	}

	return nil
}

// benchmarkConcurrentProcessing 基准测试并发处理
func (bs *BenchmarkSuite) benchmarkConcurrentProcessing(concurrencyLevel int) error {
	startTime := time.Now()
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// 创建测试任务
	jobCount := 1000
	jobs := make([]Job, jobCount)

	for i := range jobCount {
		jobs[i] = &PriorityJob{
			ID:       fmt.Sprintf("job_%d", i),
			Priority: i % 10,
			Handler: func() (any, error) {
				// 模拟计算密集型任务
				sum := 0
				for j := range 10000 {
					sum += j
				}
				return sum, nil
			},
		}
	}

	// 提交任务
	for _, job := range jobs {
		if err := bs.processor.Submit(job); err != nil {
			return fmt.Errorf("提交任务失败: %w", err)
		}
	}

	// 等待所有任务完成
	for {
		stats := bs.processor.GetStats()
		if stats.JobsCompleted >= int64(jobCount) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	duration := time.Since(startTime)
	runtime.ReadMemStats(&memAfter)

	stats := bs.processor.GetStats()

	result := BenchmarkResult{
		TestName:     fmt.Sprintf("Concurrent_%d_workers", concurrencyLevel),
		FileSize:     int64(jobCount),
		Duration:     duration,
		Throughput:   float64(jobCount) / duration.Seconds(), // jobs/s
		MemoryUsage:  int64(memAfter.Alloc - memBefore.Alloc),
		IOOperations: stats.JobsCompleted,
		Success:      stats.JobsFailed == 0,
	}

	bs.results = append(bs.results, result)
	return nil
}

// RunStreamBenchmarks 运行流处理基准测试
func (bs *BenchmarkSuite) RunStreamBenchmarks() error {
	fmt.Println("开始流处理基准测试...")

	for _, filePath := range bs.testFiles {
		if err := bs.benchmarkStreamProcessing(filePath); err != nil {
			fmt.Printf("流处理测试失败 %s: %v\n", filePath, err)
		}
	}

	return nil
}

// benchmarkStreamProcessing 基准测试流处理
func (bs *BenchmarkSuite) benchmarkStreamProcessing(filePath string) error {
	startTime := time.Now()
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	totalBytes := int64(0)
	processor := func(data []byte, offset int64) error {
		totalBytes += int64(len(data))
		// 模拟数据处理
		checksum := uint32(0)
		for _, b := range data {
			checksum += uint32(b)
		}
		return nil
	}

	if err := bs.streamer.ProcessReader(file, processor); err != nil {
		return err
	}

	duration := time.Since(startTime)
	runtime.ReadMemStats(&memAfter)

	stats := bs.streamer.GetStats()

	result := BenchmarkResult{
		TestName:     fmt.Sprintf("StreamProcessing_%s", filepath.Base(filePath)),
		FileSize:     totalBytes,
		Duration:     duration,
		Throughput:   float64(totalBytes) / duration.Seconds() / (1024 * 1024), // MB/s
		MemoryUsage:  int64(memAfter.Alloc - memBefore.Alloc),
		IOOperations: int64(stats.ChunksProcessed),
		CacheHitRate: stats.CacheHitRate,
		Success:      true,
	}

	bs.results = append(bs.results, result)
	return nil
}

// GenerateReport 生成性能报告
func (bs *BenchmarkSuite) GenerateReport() string {
	var report strings.Builder
	report.WriteString("HexDiff 性能基准测试报告\n")
	report.WriteString("================================\n\n")

	// 按测试类型分组
	ioTests := make([]BenchmarkResult, 0)
	concurrentTests := make([]BenchmarkResult, 0)
	streamTests := make([]BenchmarkResult, 0)

	for _, result := range bs.results {
		switch {
		case contains(result.TestName, "Read"):
			ioTests = append(ioTests, result)
		case contains(result.TestName, "Concurrent"):
			concurrentTests = append(concurrentTests, result)
		case contains(result.TestName, "Stream"):
			streamTests = append(streamTests, result)
		}
	}

	// I/O性能报告
	if len(ioTests) > 0 {
		report.WriteString("I/O性能测试结果:\n")
		report.WriteString("----------------\n")
		for _, result := range ioTests {
			report.WriteString(fmt.Sprintf("测试: %s\n", result.TestName))
			report.WriteString(fmt.Sprintf("  文件大小: %.2f MB\n", float64(result.FileSize)/(1024*1024)))
			report.WriteString(fmt.Sprintf("  执行时间: %v\n", result.Duration))
			report.WriteString(fmt.Sprintf("  吞吐量: %.2f MB/s\n", result.Throughput))
			report.WriteString(fmt.Sprintf("  内存使用: %.2f MB\n", float64(result.MemoryUsage)/(1024*1024)))
			if result.CacheHitRate > 0 {
				report.WriteString(fmt.Sprintf("  缓存命中率: %.2f%%\n", result.CacheHitRate))
			}
			report.WriteString(fmt.Sprintf("  状态: %s\n\n", getStatusString(result.Success)))
		}
	}

	// 并发性能报告
	if len(concurrentTests) > 0 {
		report.WriteString("并发处理测试结果:\n")
		report.WriteString("----------------\n")
		for _, result := range concurrentTests {
			report.WriteString(fmt.Sprintf("测试: %s\n", result.TestName))
			report.WriteString(fmt.Sprintf("  任务数量: %d\n", result.FileSize))
			report.WriteString(fmt.Sprintf("  执行时间: %v\n", result.Duration))
			report.WriteString(fmt.Sprintf("  吞吐量: %.2f 任务/秒\n", result.Throughput))
			report.WriteString(fmt.Sprintf("  内存使用: %.2f MB\n", float64(result.MemoryUsage)/(1024*1024)))
			report.WriteString(fmt.Sprintf("  状态: %s\n\n", getStatusString(result.Success)))
		}
	}

	// 流处理性能报告
	if len(streamTests) > 0 {
		report.WriteString("流处理测试结果:\n")
		report.WriteString("----------------\n")
		for _, result := range streamTests {
			report.WriteString(fmt.Sprintf("测试: %s\n", result.TestName))
			report.WriteString(fmt.Sprintf("  文件大小: %.2f MB\n", float64(result.FileSize)/(1024*1024)))
			report.WriteString(fmt.Sprintf("  执行时间: %v\n", result.Duration))
			report.WriteString(fmt.Sprintf("  吞吐量: %.2f MB/s\n", result.Throughput))
			report.WriteString(fmt.Sprintf("  内存使用: %.2f MB\n", float64(result.MemoryUsage)/(1024*1024)))
			if result.CacheHitRate > 0 {
				report.WriteString(fmt.Sprintf("  缓存命中率: %.2f%%\n", result.CacheHitRate))
			}
			report.WriteString(fmt.Sprintf("  状态: %s\n\n", getStatusString(result.Success)))
		}
	}

	// 性能总结
	report.WriteString("性能总结:\n")
	report.WriteString("--------\n")

	if len(ioTests) > 0 {
		maxThroughput := 0.0
		bestTest := ""
		for _, result := range ioTests {
			if result.Throughput > maxThroughput {
				maxThroughput = result.Throughput
				bestTest = result.TestName
			}
		}
		report.WriteString(fmt.Sprintf("最佳I/O性能: %s (%.2f MB/s)\n", bestTest, maxThroughput))
	}

	return report.String()
}

// Cleanup 清理测试文件
func (bs *BenchmarkSuite) Cleanup() error {
	return os.RemoveAll(bs.testDir)
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && s[len(s)-len(substr):] == substr ||
		(len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func getStatusString(success bool) string {
	if success {
		return "成功 ✅"
	}
	return "失败 ❌"
}
