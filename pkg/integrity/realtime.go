package integrity

import (
	"crypto/sha256"
	"fmt"
	"hash/crc32"
	"io"
	"sync"
	"time"
)

// RealtimeVerifier 实时验证器
type RealtimeVerifier struct {
	checker       *IntegrityChecker
	streamBuffer  []byte
	bufferSize    int
	currentOffset int64
	mutex         sync.Mutex
	errorCallback func(error)
	stats         *VerificationStats
}

// VerificationStats 验证统计信息
type VerificationStats struct {
	TotalBytes       int64        // 总字节数
	VerifiedBytes    int64        // 已验证字节数
	FailedBytes      int64        // 失败字节数
	BlocksVerified   int          // 已验证块数
	BlocksFailed     int          // 失败块数
	StartTime        time.Time    // 开始时间
	LastUpdateTime   time.Time    // 最后更新时间
	VerificationRate float64      // 验证速率（字节/秒）
	ErrorCount       int          // 错误计数
	mutex            sync.RWMutex // 统计信息锁
}

// NewRealtimeVerifier 创建新的实时验证器
func NewRealtimeVerifier(checker *IntegrityChecker, bufferSize int) *RealtimeVerifier {
	if bufferSize <= 0 {
		bufferSize = 64 * 1024 // 默认64KB
	}

	return &RealtimeVerifier{
		checker:       checker,
		streamBuffer:  make([]byte, 0, bufferSize),
		bufferSize:    bufferSize,
		currentOffset: 0,
		stats: &VerificationStats{
			StartTime:      time.Now(),
			LastUpdateTime: time.Now(),
		},
	}
}

// SetErrorCallback 设置错误回调函数
func (rv *RealtimeVerifier) SetErrorCallback(callback func(error)) {
	rv.errorCallback = callback
}

// Write 实现io.Writer接口，用于实时验证数据流
func (rv *RealtimeVerifier) Write(data []byte) (int, error) {
	rv.mutex.Lock()
	defer rv.mutex.Unlock()

	totalWritten := 0

	for len(data) > 0 {
		// 计算可以写入缓冲区的数据量
		available := rv.bufferSize - len(rv.streamBuffer)
		toWrite := min(len(data), available)

		// 写入缓冲区
		rv.streamBuffer = append(rv.streamBuffer, data[:toWrite]...)
		data = data[toWrite:]
		totalWritten += toWrite

		// 如果缓冲区满了或者是最后一块数据，进行验证
		if len(rv.streamBuffer) == rv.bufferSize || len(data) == 0 {
			if err := rv.verifyBuffer(); err != nil {
				rv.handleError(err)
				return totalWritten, err
			}
		}
	}

	// 更新统计信息
	rv.updateStats(int64(totalWritten))

	return totalWritten, nil
}

// verifyBuffer 验证缓冲区中的数据
func (rv *RealtimeVerifier) verifyBuffer() error {
	if len(rv.streamBuffer) == 0 {
		return nil
	}

	// 验证当前块
	if err := rv.checker.VerifyBlock(rv.currentOffset, rv.streamBuffer); err != nil {
		rv.stats.mutex.Lock()
		rv.stats.BlocksFailed++
		rv.stats.FailedBytes += int64(len(rv.streamBuffer))
		rv.stats.ErrorCount++
		rv.stats.mutex.Unlock()
		return err
	}

	// 更新统计信息
	rv.stats.mutex.Lock()
	rv.stats.BlocksVerified++
	rv.stats.VerifiedBytes += int64(len(rv.streamBuffer))
	rv.stats.mutex.Unlock()

	// 更新偏移量并清空缓冲区
	rv.currentOffset += int64(len(rv.streamBuffer))
	rv.streamBuffer = rv.streamBuffer[:0]

	return nil
}

// Flush 刷新剩余数据
func (rv *RealtimeVerifier) Flush() error {
	rv.mutex.Lock()
	defer rv.mutex.Unlock()

	if len(rv.streamBuffer) > 0 {
		return rv.verifyBuffer()
	}
	return nil
}

// handleError 处理错误
func (rv *RealtimeVerifier) handleError(err error) {
	if rv.errorCallback != nil {
		rv.errorCallback(err)
	}
}

// updateStats 更新统计信息
func (rv *RealtimeVerifier) updateStats(bytesProcessed int64) {
	rv.stats.mutex.Lock()
	defer rv.stats.mutex.Unlock()

	rv.stats.TotalBytes += bytesProcessed
	now := time.Now()
	duration := now.Sub(rv.stats.StartTime).Seconds()
	if duration > 0 {
		rv.stats.VerificationRate = float64(rv.stats.TotalBytes) / duration
	}
	rv.stats.LastUpdateTime = now
}

// GetStats 获取验证统计信息
func (rv *RealtimeVerifier) GetStats() *VerificationStats {
	rv.stats.mutex.RLock()
	defer rv.stats.mutex.RUnlock()

	// 返回统计信息的副本
	return &VerificationStats{
		TotalBytes:       rv.stats.TotalBytes,
		VerifiedBytes:    rv.stats.VerifiedBytes,
		FailedBytes:      rv.stats.FailedBytes,
		BlocksVerified:   rv.stats.BlocksVerified,
		BlocksFailed:     rv.stats.BlocksFailed,
		StartTime:        rv.stats.StartTime,
		LastUpdateTime:   rv.stats.LastUpdateTime,
		VerificationRate: rv.stats.VerificationRate,
		ErrorCount:       rv.stats.ErrorCount,
	}
}

// Reset 重置验证器状态
func (rv *RealtimeVerifier) Reset() {
	rv.mutex.Lock()
	defer rv.mutex.Unlock()

	rv.streamBuffer = rv.streamBuffer[:0]
	rv.currentOffset = 0

	rv.stats.mutex.Lock()
	rv.stats.TotalBytes = 0
	rv.stats.VerifiedBytes = 0
	rv.stats.FailedBytes = 0
	rv.stats.BlocksVerified = 0
	rv.stats.BlocksFailed = 0
	rv.stats.StartTime = time.Now()
	rv.stats.LastUpdateTime = time.Now()
	rv.stats.VerificationRate = 0
	rv.stats.ErrorCount = 0
	rv.stats.mutex.Unlock()
}

// String 返回统计信息的字符串表示
func (vs *VerificationStats) String() string {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	duration := vs.LastUpdateTime.Sub(vs.StartTime)
	successRate := float64(vs.VerifiedBytes) / float64(vs.TotalBytes) * 100
	if vs.TotalBytes == 0 {
		successRate = 0
	}

	return fmt.Sprintf(`实时验证统计:
  总字节数: %d
  已验证字节: %d
  失败字节: %d
  成功率: %.2f%%
  已验证块数: %d
  失败块数: %d
  验证速率: %.2f KB/s
  错误计数: %d
  运行时间: %v`,
		vs.TotalBytes,
		vs.VerifiedBytes,
		vs.FailedBytes,
		successRate,
		vs.BlocksVerified,
		vs.BlocksFailed,
		vs.VerificationRate/1024,
		vs.ErrorCount,
		duration)
}

// ProgressiveVerifier 渐进式验证器（用于大文件分块验证）
type ProgressiveVerifier struct {
	checker          *IntegrityChecker
	blockSize        int
	totalSize        int64
	processedSize    int64
	progressCallback func(processed, total int64, percentage float64)
	errorCallback    func(error)
	mutex            sync.Mutex
}

// NewProgressiveVerifier 创建新的渐进式验证器
func NewProgressiveVerifier(checker *IntegrityChecker, totalSize int64) *ProgressiveVerifier {
	return &ProgressiveVerifier{
		checker:   checker,
		blockSize: checker.blockSize,
		totalSize: totalSize,
	}
}

// SetProgressCallback 设置进度回调函数
func (pv *ProgressiveVerifier) SetProgressCallback(callback func(processed, total int64, percentage float64)) {
	pv.progressCallback = callback
}

// SetErrorCallback 设置错误回调函数
func (pv *ProgressiveVerifier) SetErrorCallback(callback func(error)) {
	pv.errorCallback = callback
}

// VerifyReader 验证Reader中的数据
func (pv *ProgressiveVerifier) VerifyReader(reader io.Reader) error {
	buffer := make([]byte, pv.blockSize)
	var offset int64 = 0

	for {
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("读取数据失败: %w", err)
		}

		if n == 0 {
			break
		}

		// 验证当前块
		blockData := buffer[:n]
		if verifyErr := pv.checker.VerifyBlock(offset, blockData); verifyErr != nil {
			if pv.errorCallback != nil {
				pv.errorCallback(verifyErr)
			}
			return verifyErr
		}

		// 更新进度
		pv.mutex.Lock()
		pv.processedSize += int64(n)
		processed := pv.processedSize
		pv.mutex.Unlock()

		if pv.progressCallback != nil {
			percentage := float64(processed) / float64(pv.totalSize) * 100
			pv.progressCallback(processed, pv.totalSize, percentage)
		}

		offset += int64(n)

		if err == io.EOF {
			break
		}
	}

	return nil
}

// GetProgress 获取当前进度
func (pv *ProgressiveVerifier) GetProgress() (processed, total int64, percentage float64) {
	pv.mutex.Lock()
	defer pv.mutex.Unlock()

	processed = pv.processedSize
	total = pv.totalSize
	if total > 0 {
		percentage = float64(processed) / float64(total) * 100
	}

	return processed, total, percentage
}

// ConcurrentVerifier 并发验证器
type ConcurrentVerifier struct {
	checker     *IntegrityChecker
	workerCount int
	jobQueue    chan VerificationJob
	resultQueue chan VerificationResult
	wg          sync.WaitGroup
	errors      []error
	mutex       sync.Mutex
}

// VerificationJob 验证任务
type VerificationJob struct {
	Offset int64
	Data   []byte
	ID     int
}

// NewConcurrentVerifier 创建新的并发验证器
func NewConcurrentVerifier(checker *IntegrityChecker, workerCount int) *ConcurrentVerifier {
	if workerCount <= 0 {
		workerCount = 4 // 默认4个工作协程
	}

	return &ConcurrentVerifier{
		checker:     checker,
		workerCount: workerCount,
		jobQueue:    make(chan VerificationJob, workerCount*2),
		resultQueue: make(chan VerificationResult, workerCount*2),
		errors:      make([]error, 0),
	}
}

// Start 启动并发验证器
func (cv *ConcurrentVerifier) Start() {
	// 启动工作协程
	for i := 0; i < cv.workerCount; i++ {
		cv.wg.Add(1)
		go cv.worker(i)
	}
}

// worker 工作协程
func (cv *ConcurrentVerifier) worker(id int) {
	defer cv.wg.Done()

	for job := range cv.jobQueue {
		result := VerificationResult{
			FilePath: fmt.Sprintf("worker-%d-job-%d", id, job.ID),
			Success:  true,
		}

		// 验证数据块
		if err := cv.checker.VerifyBlock(job.Offset, job.Data); err != nil {
			result.Success = false
			result.Errors = []error{err}

			cv.mutex.Lock()
			cv.errors = append(cv.errors, err)
			cv.mutex.Unlock()
		}

		cv.resultQueue <- result
	}
}

// SubmitJob 提交验证任务
func (cv *ConcurrentVerifier) SubmitJob(offset int64, data []byte, id int) {
	job := VerificationJob{
		Offset: offset,
		Data:   make([]byte, len(data)), // 创建数据副本
		ID:     id,
	}
	copy(job.Data, data)

	cv.jobQueue <- job
}

// Stop 停止并发验证器
func (cv *ConcurrentVerifier) Stop() {
	close(cv.jobQueue)
	cv.wg.Wait()
	close(cv.resultQueue)
}

// GetResults 获取所有验证结果
func (cv *ConcurrentVerifier) GetResults() []VerificationResult {
	results := make([]VerificationResult, 0)

	for result := range cv.resultQueue {
		results = append(results, result)
	}

	return results
}

// GetErrors 获取所有错误
func (cv *ConcurrentVerifier) GetErrors() []error {
	cv.mutex.Lock()
	defer cv.mutex.Unlock()

	// 返回错误副本
	errors := make([]error, len(cv.errors))
	copy(errors, cv.errors)
	return errors
}

// HasErrors 检查是否有错误
func (cv *ConcurrentVerifier) HasErrors() bool {
	cv.mutex.Lock()
	defer cv.mutex.Unlock()
	return len(cv.errors) > 0
}

// DualHashVerifier 双重哈希验证器（SHA-256 + CRC32）
type DualHashVerifier struct {
	enableSHA256 bool
	enableCRC32  bool
}

// NewDualHashVerifier 创建新的双重哈希验证器
func NewDualHashVerifier(enableSHA256, enableCRC32 bool) *DualHashVerifier {
	return &DualHashVerifier{
		enableSHA256: enableSHA256,
		enableCRC32:  enableCRC32,
	}
}

// VerifyData 验证数据的双重哈希
func (dhv *DualHashVerifier) VerifyData(data []byte, expectedSHA256 [32]byte, expectedCRC32 uint32) error {
	// 验证SHA-256
	if dhv.enableSHA256 {
		actualSHA256 := sha256.Sum256(data)
		if actualSHA256 != expectedSHA256 {
			return fmt.Errorf("SHA-256校验和不匹配")
		}
	}

	// 验证CRC32
	if dhv.enableCRC32 {
		actualCRC32 := crc32.ChecksumIEEE(data)
		if actualCRC32 != expectedCRC32 {
			return fmt.Errorf("CRC32校验和不匹配")
		}
	}

	return nil
}

// ComputeHashes 计算数据的双重哈希
func (dhv *DualHashVerifier) ComputeHashes(data []byte) (sha256Hash [32]byte, crc32Hash uint32) {
	if dhv.enableSHA256 {
		sha256Hash = sha256.Sum256(data)
	}

	if dhv.enableCRC32 {
		crc32Hash = crc32.ChecksumIEEE(data)
	}

	return sha256Hash, crc32Hash
}
