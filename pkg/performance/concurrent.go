package performance

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrentProcessor 并发处理器
type ConcurrentProcessor struct {
	workerCount  int
	queueSize    int
	workers      []*Worker
	jobQueue     chan Job
	resultQueue  chan Result
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	stats        *ConcurrentStats
	errorHandler func(error)
	paused       int32 // 原子操作标志
}

// ConcurrentConfig 并发配置
type ConcurrentConfig struct {
	WorkerCount  int           // 工作协程数量
	QueueSize    int           // 队列大小
	Timeout      time.Duration // 超时时间
	ErrorHandler func(error)   // 错误处理函数
}

// ConcurrentStats 并发统计
type ConcurrentStats struct {
	JobsSubmitted  int64         // 提交的任务数
	JobsCompleted  int64         // 完成的任务数
	JobsFailed     int64         // 失败的任务数
	WorkersActive  int32         // 活跃工作协程数
	WorkersIdle    int32         // 空闲工作协程数
	AverageLatency time.Duration // 平均延迟
	TotalLatency   time.Duration // 总延迟
	StartTime      time.Time     // 开始时间
	mutex          sync.RWMutex  // 统计锁
}

// Job 任务接口
type Job interface {
	Execute() (any, error)
	GetID() string
	GetPriority() int
}

// Result 结果
type Result struct {
	JobID    string
	Data     any
	Error    error
	Duration time.Duration
}

// Worker 工作协程
type Worker struct {
	id        int
	processor *ConcurrentProcessor
	active    int32 // 原子操作标志
}

// DefaultConcurrentConfig 默认并发配置
func DefaultConcurrentConfig() *ConcurrentConfig {
	return &ConcurrentConfig{
		WorkerCount: runtime.NumCPU(),
		QueueSize:   1000,
		Timeout:     30 * time.Second,
		ErrorHandler: func(err error) {
			fmt.Printf("并发处理错误: %v\n", err)
		},
	}
}

// NewConcurrentProcessor 创建并发处理器
func NewConcurrentProcessor(config *ConcurrentConfig) *ConcurrentProcessor {
	if config == nil {
		config = DefaultConcurrentConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	cp := &ConcurrentProcessor{
		workerCount:  config.WorkerCount,
		queueSize:    config.QueueSize,
		jobQueue:     make(chan Job, config.QueueSize),
		resultQueue:  make(chan Result, config.QueueSize),
		ctx:          ctx,
		cancel:       cancel,
		errorHandler: config.ErrorHandler,
		stats: &ConcurrentStats{
			StartTime: time.Now(),
		},
	}

	// 创建工作协程
	cp.workers = make([]*Worker, config.WorkerCount)
	for i := 0; i < config.WorkerCount; i++ {
		cp.workers[i] = &Worker{
			id:        i,
			processor: cp,
		}
	}

	return cp
}

// Start 启动并发处理器
func (cp *ConcurrentProcessor) Start() {
	// 启动工作协程
	for _, worker := range cp.workers {
		cp.wg.Add(1)
		go worker.run()
	}

	// 启动结果收集协程
	go cp.collectResults()
}

// Stop 停止并发处理器
func (cp *ConcurrentProcessor) Stop() {
	cp.cancel()
	close(cp.jobQueue)
	cp.wg.Wait()
	close(cp.resultQueue)
}

// Submit 提交任务
func (cp *ConcurrentProcessor) Submit(job Job) error {
	select {
	case cp.jobQueue <- job:
		atomic.AddInt64(&cp.stats.JobsSubmitted, 1)
		return nil
	case <-cp.ctx.Done():
		return cp.ctx.Err()
	default:
		return fmt.Errorf("任务队列已满")
	}
}

// SubmitWithTimeout 带超时的任务提交
func (cp *ConcurrentProcessor) SubmitWithTimeout(job Job, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(cp.ctx, timeout)
	defer cancel()

	select {
	case cp.jobQueue <- job:
		atomic.AddInt64(&cp.stats.JobsSubmitted, 1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Pause 暂停处理器
func (cp *ConcurrentProcessor) Pause() {
	atomic.StoreInt32(&cp.paused, 1)
}

// Resume 恢复处理器
func (cp *ConcurrentProcessor) Resume() {
	atomic.StoreInt32(&cp.paused, 0)
}

// IsPaused 检查是否暂停
func (cp *ConcurrentProcessor) IsPaused() bool {
	return atomic.LoadInt32(&cp.paused) == 1
}

// GetStats 获取统计信息
func (cp *ConcurrentProcessor) GetStats() *ConcurrentStats {
	cp.stats.mutex.RLock()
	defer cp.stats.mutex.RUnlock()

	return &ConcurrentStats{
		JobsSubmitted:  atomic.LoadInt64(&cp.stats.JobsSubmitted),
		JobsCompleted:  atomic.LoadInt64(&cp.stats.JobsCompleted),
		JobsFailed:     atomic.LoadInt64(&cp.stats.JobsFailed),
		WorkersActive:  atomic.LoadInt32(&cp.stats.WorkersActive),
		WorkersIdle:    atomic.LoadInt32(&cp.stats.WorkersIdle),
		AverageLatency: cp.stats.AverageLatency,
		TotalLatency:   cp.stats.TotalLatency,
		StartTime:      cp.stats.StartTime,
	}
}

// run 工作协程运行方法
func (w *Worker) run() {
	defer w.processor.wg.Done()

	for {
		select {
		case job, ok := <-w.processor.jobQueue:
			if !ok {
				return // 队列已关闭
			}

			// 检查是否暂停
			for w.processor.IsPaused() {
				time.Sleep(100 * time.Millisecond)
				select {
				case <-w.processor.ctx.Done():
					return
				default:
				}
			}

			w.processJob(job)

		case <-w.processor.ctx.Done():
			return
		}
	}
}

// processJob 处理任务
func (w *Worker) processJob(job Job) {
	startTime := time.Now()

	// 标记为活跃
	atomic.StoreInt32(&w.active, 1)
	atomic.AddInt32(&w.processor.stats.WorkersActive, 1)
	atomic.AddInt32(&w.processor.stats.WorkersIdle, -1)

	defer func() {
		// 标记为空闲
		atomic.StoreInt32(&w.active, 0)
		atomic.AddInt32(&w.processor.stats.WorkersActive, -1)
		atomic.AddInt32(&w.processor.stats.WorkersIdle, 1)
	}()

	// 执行任务
	data, err := job.Execute()
	duration := time.Since(startTime)

	// 创建结果
	result := Result{
		JobID:    job.GetID(),
		Data:     data,
		Error:    err,
		Duration: duration,
	}

	// 更新统计
	if err != nil {
		atomic.AddInt64(&w.processor.stats.JobsFailed, 1)
		if w.processor.errorHandler != nil {
			w.processor.errorHandler(err)
		}
	} else {
		atomic.AddInt64(&w.processor.stats.JobsCompleted, 1)
	}

	// 更新延迟统计
	w.processor.updateLatencyStats(duration)

	// 发送结果
	select {
	case w.processor.resultQueue <- result:
	case <-w.processor.ctx.Done():
		return
	}
}

// updateLatencyStats 更新延迟统计
func (cp *ConcurrentProcessor) updateLatencyStats(duration time.Duration) {
	cp.stats.mutex.Lock()
	defer cp.stats.mutex.Unlock()

	cp.stats.TotalLatency += duration
	completed := atomic.LoadInt64(&cp.stats.JobsCompleted)
	if completed > 0 {
		cp.stats.AverageLatency = cp.stats.TotalLatency / time.Duration(completed)
	}
}

// collectResults 收集结果
func (cp *ConcurrentProcessor) collectResults() {
	for result := range cp.resultQueue {
		// 这里可以添加结果处理逻辑
		_ = result
	}
}

// String 返回统计信息的字符串表示
func (cs *ConcurrentStats) String() string {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	duration := time.Since(cs.StartTime)
	throughput := float64(cs.JobsCompleted) / duration.Seconds()
	successRate := float64(0)
	if cs.JobsSubmitted > 0 {
		successRate = float64(cs.JobsCompleted) / float64(cs.JobsSubmitted) * 100
	}

	return fmt.Sprintf(`并发处理统计:
  提交任务数: %d
  完成任务数: %d
  失败任务数: %d
  成功率: %.2f%%
  活跃工作协程: %d
  空闲工作协程: %d
  平均延迟: %v
  吞吐量: %.2f 任务/秒
  运行时间: %v`,
		cs.JobsSubmitted,
		cs.JobsCompleted,
		cs.JobsFailed,
		successRate,
		cs.WorkersActive,
		cs.WorkersIdle,
		cs.AverageLatency,
		throughput,
		duration)
}

// PriorityJob 优先级任务
type PriorityJob struct {
	ID       string
	Priority int
	Handler  func() (any, error)
}

// Execute 执行任务
func (pj *PriorityJob) Execute() (any, error) {
	return pj.Handler()
}

// GetID 获取任务ID
func (pj *PriorityJob) GetID() string {
	return pj.ID
}

// GetPriority 获取优先级
func (pj *PriorityJob) GetPriority() int {
	return pj.Priority
}

// BatchProcessor 批处理器
type BatchProcessor struct {
	processor   *ConcurrentProcessor
	batchSize   int
	batchBuffer []Job
	mutex       sync.Mutex
}

// NewBatchProcessor 创建批处理器
func NewBatchProcessor(config *ConcurrentConfig, batchSize int) *BatchProcessor {
	return &BatchProcessor{
		processor:   NewConcurrentProcessor(config),
		batchSize:   batchSize,
		batchBuffer: make([]Job, 0, batchSize),
	}
}

// AddJob 添加任务到批处理
func (bp *BatchProcessor) AddJob(job Job) error {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	bp.batchBuffer = append(bp.batchBuffer, job)

	// 如果达到批大小，提交批处理
	if len(bp.batchBuffer) >= bp.batchSize {
		return bp.submitBatch()
	}

	return nil
}

// submitBatch 提交批处理
func (bp *BatchProcessor) submitBatch() error {
	if len(bp.batchBuffer) == 0 {
		return nil
	}

	// 创建批任务
	batchJob := &BatchJob{
		ID:   fmt.Sprintf("batch_%d", time.Now().UnixNano()),
		Jobs: make([]Job, len(bp.batchBuffer)),
	}
	copy(batchJob.Jobs, bp.batchBuffer)

	// 清空缓冲区
	bp.batchBuffer = bp.batchBuffer[:0]

	// 提交批任务
	return bp.processor.Submit(batchJob)
}

// Flush 刷新剩余任务
func (bp *BatchProcessor) Flush() error {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()
	return bp.submitBatch()
}

// BatchJob 批任务
type BatchJob struct {
	ID   string
	Jobs []Job
}

// Execute 执行批任务
func (bj *BatchJob) Execute() (any, error) {
	results := make([]Result, len(bj.Jobs))

	for i, job := range bj.Jobs {
		startTime := time.Now()
		data, err := job.Execute()
		results[i] = Result{
			JobID:    job.GetID(),
			Data:     data,
			Error:    err,
			Duration: time.Since(startTime),
		}
	}

	return results, nil
}

// GetID 获取批任务ID
func (bj *BatchJob) GetID() string {
	return bj.ID
}

// GetPriority 获取批任务优先级
func (bj *BatchJob) GetPriority() int {
	return 0 // 默认优先级
}

// WorkerPool 工作协程池
type WorkerPool struct {
	workers     []*PoolWorker
	jobQueue    chan func()
	workerCount int
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// PoolWorker 池工作协程
type PoolWorker struct {
	id   int
	pool *WorkerPool
}

// NewWorkerPool 创建工作协程池
func NewWorkerPool(workerCount int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	wp := &WorkerPool{
		workers:     make([]*PoolWorker, workerCount),
		jobQueue:    make(chan func(), workerCount*2),
		workerCount: workerCount,
		ctx:         ctx,
		cancel:      cancel,
	}

	// 创建工作协程
	for i := range workerCount {
		wp.workers[i] = &PoolWorker{
			id:   i,
			pool: wp,
		}
	}

	return wp
}

// Start 启动工作协程池
func (wp *WorkerPool) Start() {
	for _, worker := range wp.workers {
		wp.wg.Add(1)
		go worker.run()
	}
}

// Stop 停止工作协程池
func (wp *WorkerPool) Stop() {
	wp.cancel()
	close(wp.jobQueue)
	wp.wg.Wait()
}

// Submit 提交任务
func (wp *WorkerPool) Submit(job func()) {
	select {
	case wp.jobQueue <- job:
	case <-wp.ctx.Done():
	}
}

// run 工作协程运行方法
func (pw *PoolWorker) run() {
	defer pw.pool.wg.Done()

	for {
		select {
		case job, ok := <-pw.pool.jobQueue:
			if !ok {
				return
			}
			job()
		case <-pw.pool.ctx.Done():
			return
		}
	}
}
