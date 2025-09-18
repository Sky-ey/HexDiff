package performance

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// StreamProcessor 流式处理器
type StreamProcessor struct {
	bufferSize  int          // 缓冲区大小
	workerCount int          // 工作协程数量
	chunkSize   int64        // 数据块大小
	maxMemory   int64        // 最大内存使用量
	enableCache bool         // 是否启用缓存
	cache       *LRUCache    // LRU缓存
	stats       *StreamStats // 流处理统计
	ctx         context.Context
	cancel      context.CancelFunc
}

// StreamConfig 流处理配置
type StreamConfig struct {
	BufferSize  int   // 缓冲区大小（默认64KB）
	WorkerCount int   // 工作协程数量（默认CPU核心数）
	ChunkSize   int64 // 数据块大小（默认1MB）
	MaxMemory   int64 // 最大内存使用量（默认100MB）
	EnableCache bool  // 是否启用缓存
	CacheSize   int   // 缓存大小（默认1000个条目）
}

// StreamStats 流处理统计
type StreamStats struct {
	TotalBytes      int64        // 总处理字节数
	ProcessedBytes  int64        // 已处理字节数
	ChunksProcessed int          // 已处理块数
	WorkersActive   int          // 活跃工作协程数
	StartTime       time.Time    // 开始时间
	LastUpdateTime  time.Time    // 最后更新时间
	Throughput      float64      // 吞吐量（字节/秒）
	MemoryUsage     int64        // 内存使用量
	CacheHitRate    float64      // 缓存命中率
	mutex           sync.RWMutex // 统计锁
}

// DefaultStreamConfig 默认流处理配置
func DefaultStreamConfig() *StreamConfig {
	return &StreamConfig{
		BufferSize:  64 * 1024,         // 64KB
		WorkerCount: runtime.NumCPU(),  // CPU核心数
		ChunkSize:   1024 * 1024,       // 1MB
		MaxMemory:   100 * 1024 * 1024, // 100MB
		EnableCache: true,
		CacheSize:   1000,
	}
}

// NewStreamProcessor 创建新的流处理器
func NewStreamProcessor(config *StreamConfig) *StreamProcessor {
	if config == nil {
		config = DefaultStreamConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	sp := &StreamProcessor{
		bufferSize:  config.BufferSize,
		workerCount: config.WorkerCount,
		chunkSize:   config.ChunkSize,
		maxMemory:   config.MaxMemory,
		enableCache: config.EnableCache,
		ctx:         ctx,
		cancel:      cancel,
		stats: &StreamStats{
			StartTime:      time.Now(),
			LastUpdateTime: time.Now(),
		},
	}

	// 初始化缓存
	if config.EnableCache {
		sp.cache = NewLRUCache(config.CacheSize)
	}

	return sp
}

// ProcessFile 流式处理文件
func (sp *StreamProcessor) ProcessFile(filePath string, processor func([]byte, int64) error) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// 获取文件大小
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	sp.stats.mutex.Lock()
	sp.stats.TotalBytes = stat.Size()
	sp.stats.mutex.Unlock()

	return sp.ProcessReader(file, processor)
}

// ProcessReader 流式处理Reader
func (sp *StreamProcessor) ProcessReader(reader io.Reader, processor func([]byte, int64) error) error {
	// 创建工作协程池
	jobs := make(chan StreamJob, sp.workerCount*2)
	results := make(chan StreamResult, sp.workerCount*2)

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < sp.workerCount; i++ {
		wg.Add(1)
		go func() {
			sp.worker(jobs, results, processor, &wg)
		}()
	}

	// 启动结果收集协程
	go sp.resultCollector(results)

	// 读取数据并分发任务
	bufferedReader := bufio.NewReaderSize(reader, sp.bufferSize)
	var offset int64 = 0
	chunkID := 0

	for {
		// 检查上下文是否被取消
		select {
		case <-sp.ctx.Done():
			close(jobs)
			wg.Wait()
			return sp.ctx.Err()
		default:
		}

		// 检查内存使用量
		if err := sp.checkMemoryUsage(); err != nil {
			close(jobs)
			wg.Wait()
			return err
		}

		// 读取数据块
		chunk := make([]byte, sp.chunkSize)
		n, err := io.ReadFull(bufferedReader, chunk)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			close(jobs)
			wg.Wait()
			return fmt.Errorf("读取数据失败: %w", err)
		}

		if n == 0 {
			break
		}

		// 调整块大小
		chunk = chunk[:n]

		// 创建任务
		job := StreamJob{
			ID:     chunkID,
			Data:   chunk,
			Offset: offset,
		}

		// 发送任务
		select {
		case jobs <- job:
			chunkID++
			offset += int64(n)
		case <-sp.ctx.Done():
			close(jobs)
			wg.Wait()
			return sp.ctx.Err()
		}

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
	}

	// 关闭任务通道并等待完成
	close(jobs)
	wg.Wait()
	close(results)

	return nil
}

// worker 工作协程
func (sp *StreamProcessor) worker(jobs <-chan StreamJob, results chan<- StreamResult, processor func([]byte, int64) error, wg *sync.WaitGroup) {
	defer wg.Done()

	sp.stats.mutex.Lock()
	sp.stats.WorkersActive++
	sp.stats.mutex.Unlock()

	defer func() {
		sp.stats.mutex.Lock()
		sp.stats.WorkersActive--
		sp.stats.mutex.Unlock()
	}()

	for job := range jobs {
		startTime := time.Now()

		// 检查缓存
		var err error
		if sp.enableCache && sp.cache != nil {
			cacheKey := fmt.Sprintf("%d_%d", job.Offset, len(job.Data))
			if cached, found := sp.cache.Get(cacheKey); found {
				// 缓存命中
				sp.updateCacheStats(true)
				if cachedResult, ok := cached.(error); ok {
					err = cachedResult
				}
			} else {
				// 缓存未命中，处理数据
				sp.updateCacheStats(false)
				err = processor(job.Data, job.Offset)
				sp.cache.Put(cacheKey, err)
			}
		} else {
			// 不使用缓存，直接处理
			err = processor(job.Data, job.Offset)
		}

		// 发送结果
		result := StreamResult{
			JobID:          job.ID,
			Error:          err,
			Duration:       time.Since(startTime),
			BytesProcessed: int64(len(job.Data)),
		}

		select {
		case results <- result:
		case <-sp.ctx.Done():
			return
		}
	}
}

// resultCollector 结果收集器
func (sp *StreamProcessor) resultCollector(results <-chan StreamResult) {
	for result := range results {
		sp.updateStats(result)
	}
}

// updateStats 更新统计信息
func (sp *StreamProcessor) updateStats(result StreamResult) {
	sp.stats.mutex.Lock()
	defer sp.stats.mutex.Unlock()

	sp.stats.ProcessedBytes += result.BytesProcessed
	sp.stats.ChunksProcessed++
	sp.stats.LastUpdateTime = time.Now()

	// 计算吞吐量
	duration := sp.stats.LastUpdateTime.Sub(sp.stats.StartTime).Seconds()
	if duration > 0 {
		sp.stats.Throughput = float64(sp.stats.ProcessedBytes) / duration
	}
}

// updateCacheStats 更新缓存统计
func (sp *StreamProcessor) updateCacheStats(hit bool) {
	if !sp.enableCache || sp.cache == nil {
		return
	}

	sp.stats.mutex.Lock()
	defer sp.stats.mutex.Unlock()

	// 简化的缓存命中率计算
	if hit {
		sp.stats.CacheHitRate = (sp.stats.CacheHitRate + 1.0) / 2.0
	} else {
		sp.stats.CacheHitRate = sp.stats.CacheHitRate / 2.0
	}
}

// checkMemoryUsage 检查内存使用量
func (sp *StreamProcessor) checkMemoryUsage() error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	sp.stats.mutex.Lock()
	sp.stats.MemoryUsage = int64(m.Alloc)
	sp.stats.mutex.Unlock()

	if int64(m.Alloc) > sp.maxMemory {
		runtime.GC() // 强制垃圾回收
		runtime.ReadMemStats(&m)

		if int64(m.Alloc) > sp.maxMemory {
			return fmt.Errorf("内存使用量超过限制: %d > %d", m.Alloc, sp.maxMemory)
		}
	}

	return nil
}

// GetStats 获取流处理统计信息
func (sp *StreamProcessor) GetStats() *StreamStats {
	sp.stats.mutex.RLock()
	defer sp.stats.mutex.RUnlock()

	return &StreamStats{
		TotalBytes:      sp.stats.TotalBytes,
		ProcessedBytes:  sp.stats.ProcessedBytes,
		ChunksProcessed: sp.stats.ChunksProcessed,
		WorkersActive:   sp.stats.WorkersActive,
		StartTime:       sp.stats.StartTime,
		LastUpdateTime:  sp.stats.LastUpdateTime,
		Throughput:      sp.stats.Throughput,
		MemoryUsage:     sp.stats.MemoryUsage,
		CacheHitRate:    sp.stats.CacheHitRate,
	}
}

// Stop 停止流处理器
func (sp *StreamProcessor) Stop() {
	sp.cancel()
}

// StreamJob 流处理任务
type StreamJob struct {
	ID     int    // 任务ID
	Data   []byte // 数据
	Offset int64  // 偏移量
}

// StreamResult 流处理结果
type StreamResult struct {
	JobID          int           // 任务ID
	Error          error         // 错误
	Duration       time.Duration // 处理耗时
	BytesProcessed int64         // 处理字节数
}

// String 返回统计信息的字符串表示
func (ss *StreamStats) String() string {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	duration := ss.LastUpdateTime.Sub(ss.StartTime)
	progress := float64(ss.ProcessedBytes) / float64(ss.TotalBytes) * 100
	if ss.TotalBytes == 0 {
		progress = 0
	}

	return fmt.Sprintf(`流处理统计:
  总字节数: %d
  已处理字节: %d
  处理进度: %.2f%%
  已处理块数: %d
  活跃工作协程: %d
  吞吐量: %.2f KB/s
  内存使用: %.2f MB
  缓存命中率: %.2f%%
  运行时间: %v`,
		ss.TotalBytes,
		ss.ProcessedBytes,
		progress,
		ss.ChunksProcessed,
		ss.WorkersActive,
		ss.Throughput/1024,
		float64(ss.MemoryUsage)/1024/1024,
		ss.CacheHitRate*100,
		duration)
}

// ParallelFileProcessor 并行文件处理器
type ParallelFileProcessor struct {
	processor *StreamProcessor
	semaphore chan struct{} // 信号量控制并发数
}

// NewParallelFileProcessor 创建并行文件处理器
func NewParallelFileProcessor(config *StreamConfig, maxConcurrency int) *ParallelFileProcessor {
	return &ParallelFileProcessor{
		processor: NewStreamProcessor(config),
		semaphore: make(chan struct{}, maxConcurrency),
	}
}

// ProcessFiles 并行处理多个文件
func (pfp *ParallelFileProcessor) ProcessFiles(filePaths []string, processor func([]byte, int64) error) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(filePaths))

	for _, filePath := range filePaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			// 获取信号量
			pfp.semaphore <- struct{}{}
			defer func() { <-pfp.semaphore }()

			// 处理文件
			if err := pfp.processor.ProcessFile(path, processor); err != nil {
				errChan <- fmt.Errorf("处理文件 %s 失败: %w", path, err)
			}
		}(filePath)
	}

	wg.Wait()
	close(errChan)

	// 收集错误
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("处理文件时发生 %d 个错误: %v", len(errors), errors[0])
	}

	return nil
}
