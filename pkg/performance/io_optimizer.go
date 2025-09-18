package performance

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// IOOptimizer I/O优化器
type IOOptimizer struct {
	config      *IOConfig
	bufferPool  *BufferPool
	memoryPool  *MemoryPool
	readAhead   *ReadAheadCache
	writeBuffer *WriteBuffer
	stats       *IOStats
}

// IOConfig I/O配置
type IOConfig struct {
	BufferSize       int           // 缓冲区大小
	ReadAheadSize    int           // 预读大小
	WriteBufferSize  int           // 写缓冲区大小
	MaxConcurrency   int           // 最大并发数
	EnableMmap       bool          // 是否启用内存映射
	EnableDirectIO   bool          // 是否启用直接I/O
	EnableReadAhead  bool          // 是否启用预读
	EnableWriteCache bool          // 是否启用写缓存
	SyncInterval     time.Duration // 同步间隔
}

// IOStats I/O统计
type IOStats struct {
	BytesRead       int64         // 读取字节数
	BytesWritten    int64         // 写入字节数
	ReadOperations  int64         // 读操作数
	WriteOperations int64         // 写操作数
	ReadLatency     time.Duration // 读延迟
	WriteLatency    time.Duration // 写延迟
	CacheHits       int64         // 缓存命中数
	CacheMisses     int64         // 缓存未命中数
	StartTime       time.Time     // 开始时间
	mutex           sync.RWMutex  // 统计锁
}

// DefaultIOConfig 默认I/O配置
func DefaultIOConfig() *IOConfig {
	return &IOConfig{
		BufferSize:       64 * 1024,  // 64KB
		ReadAheadSize:    256 * 1024, // 256KB
		WriteBufferSize:  128 * 1024, // 128KB
		MaxConcurrency:   runtime.NumCPU(),
		EnableMmap:       true,
		EnableDirectIO:   false,
		EnableReadAhead:  true,
		EnableWriteCache: true,
		SyncInterval:     time.Second,
	}
}

// NewIOOptimizer 创建新的I/O优化器
func NewIOOptimizer(config *IOConfig) *IOOptimizer {
	if config == nil {
		config = DefaultIOConfig()
	}

	io := &IOOptimizer{
		config:     config,
		bufferPool: NewBufferPool(config.BufferSize),
		memoryPool: NewMemoryPool(),
		stats: &IOStats{
			StartTime: time.Now(),
		},
	}

	// 初始化预读缓存
	if config.EnableReadAhead {
		io.readAhead = NewReadAheadCache(config.ReadAheadSize)
	}

	// 初始化写缓冲区
	if config.EnableWriteCache {
		io.writeBuffer = NewWriteBuffer(config.WriteBufferSize, config.SyncInterval)
	}

	return io
}

// OptimizedReader 优化的读取器
type OptimizedReader struct {
	file      *os.File
	optimizer *IOOptimizer
	buffer    []byte
	filePos   int64
	fileSize  int64
	mmapData  []byte
	useMmap   bool
}

// NewOptimizedReader 创建优化的读取器
func (io *IOOptimizer) NewOptimizedReader(filePath string) (*OptimizedReader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	reader := &OptimizedReader{
		file:      file,
		optimizer: io,
		fileSize:  stat.Size(),
		buffer:    io.bufferPool.Get(),
	}

	// 尝试使用内存映射
	if io.config.EnableMmap && stat.Size() > 0 {
		if mmapData, err := reader.setupMmap(); err == nil {
			reader.mmapData = mmapData
			reader.useMmap = true
		}
	}

	return reader, nil
}

// setupMmap 设置内存映射
func (r *OptimizedReader) setupMmap() ([]byte, error) {
	fd := int(r.file.Fd())

	// 使用mmap系统调用
	data, err := syscall.Mmap(fd, 0, int(r.fileSize), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("内存映射失败: %w", err)
	}

	return data, nil
}

// Read 读取数据
func (r *OptimizedReader) Read(p []byte) (int, error) {
	startTime := time.Now()
	defer func() {
		r.optimizer.updateReadStats(len(p), time.Since(startTime))
	}()

	if r.useMmap {
		return r.readFromMmap(p)
	}

	return r.readFromFile(p)
}

// readFromMmap 从内存映射读取
func (r *OptimizedReader) readFromMmap(p []byte) (int, error) {
	if r.filePos >= r.fileSize {
		return 0, io.EOF
	}

	remaining := r.fileSize - r.filePos
	toRead := int64(len(p))
	if toRead > remaining {
		toRead = remaining
	}

	copy(p, r.mmapData[r.filePos:r.filePos+toRead])
	r.filePos += toRead

	return int(toRead), nil
}

// readFromFile 从文件读取
func (r *OptimizedReader) readFromFile(p []byte) (int, error) {
	// 检查预读缓存
	if r.optimizer.config.EnableReadAhead && r.optimizer.readAhead != nil {
		if data, found := r.optimizer.readAhead.Get(r.filePos, len(p)); found {
			copy(p, data)
			r.filePos += int64(len(data))
			r.optimizer.updateCacheStats(true)
			return len(data), nil
		}
		r.optimizer.updateCacheStats(false)
	}

	// 从文件读取
	n, err := r.file.ReadAt(p, r.filePos)
	if n > 0 {
		r.filePos += int64(n)

		// 预读下一块数据
		if r.optimizer.config.EnableReadAhead && r.optimizer.readAhead != nil {
			go r.prefetchNext()
		}
	}

	return n, err
}

// prefetchNext 预读下一块数据
func (r *OptimizedReader) prefetchNext() {
	prefetchSize := r.optimizer.config.ReadAheadSize
	prefetchData := make([]byte, prefetchSize)

	n, err := r.file.ReadAt(prefetchData, r.filePos)
	if err == nil && n > 0 {
		r.optimizer.readAhead.Put(r.filePos, prefetchData[:n])
	}
}

// Seek 定位文件位置
func (r *OptimizedReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.filePos = offset
	case io.SeekCurrent:
		r.filePos += offset
	case io.SeekEnd:
		r.filePos = r.fileSize + offset
	}

	if r.filePos < 0 {
		r.filePos = 0
	}
	if r.filePos > r.fileSize {
		r.filePos = r.fileSize
	}

	return r.filePos, nil
}

// Close 关闭读取器
func (r *OptimizedReader) Close() error {
	var err error

	// 清理内存映射
	if r.useMmap && r.mmapData != nil {
		if unmapErr := syscall.Munmap(r.mmapData); unmapErr != nil {
			err = unmapErr
		}
	}

	// 归还缓冲区
	if r.buffer != nil {
		r.optimizer.bufferPool.Put(r.buffer)
	}

	// 关闭文件
	if closeErr := r.file.Close(); closeErr != nil && err == nil {
		err = closeErr
	}

	return err
}

// OptimizedWriter 优化的写入器
type OptimizedWriter struct {
	file      *os.File
	optimizer *IOOptimizer
	buffer    []byte
	filePos   int64
}

// NewOptimizedWriter 创建优化的写入器
func (io *IOOptimizer) NewOptimizedWriter(filePath string) (*OptimizedWriter, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}

	writer := &OptimizedWriter{
		file:      file,
		optimizer: io,
		buffer:    io.bufferPool.Get(),
	}

	return writer, nil
}

// Write 写入数据
func (w *OptimizedWriter) Write(p []byte) (int, error) {
	startTime := time.Now()
	defer func() {
		w.optimizer.updateWriteStats(len(p), time.Since(startTime))
	}()

	if w.optimizer.config.EnableWriteCache && w.optimizer.writeBuffer != nil {
		return w.optimizer.writeBuffer.Write(w.file, p)
	}

	return w.writeToFile(p)
}

// writeToFile 直接写入文件
func (w *OptimizedWriter) writeToFile(p []byte) (int, error) {
	n, err := w.file.Write(p)
	if n > 0 {
		w.filePos += int64(n)
	}
	return n, err
}

// Sync 同步数据到磁盘
func (w *OptimizedWriter) Sync() error {
	if w.optimizer.config.EnableWriteCache && w.optimizer.writeBuffer != nil {
		if err := w.optimizer.writeBuffer.Flush(w.file); err != nil {
			return err
		}
	}
	return w.file.Sync()
}

// Close 关闭写入器
func (w *OptimizedWriter) Close() error {
	var err error

	// 刷新缓冲区
	if syncErr := w.Sync(); syncErr != nil {
		err = syncErr
	}

	// 归还缓冲区
	if w.buffer != nil {
		w.optimizer.bufferPool.Put(w.buffer)
	}

	// 关闭文件
	if closeErr := w.file.Close(); closeErr != nil && err == nil {
		err = closeErr
	}

	return err
}

// ReadAheadCache 预读缓存
type ReadAheadCache struct {
	cache     map[int64][]byte
	maxSize   int
	totalSize int
	mutex     sync.RWMutex
}

// NewReadAheadCache 创建预读缓存
func NewReadAheadCache(maxSize int) *ReadAheadCache {
	return &ReadAheadCache{
		cache:   make(map[int64][]byte),
		maxSize: maxSize,
	}
}

// Get 获取缓存数据
func (rac *ReadAheadCache) Get(offset int64, size int) ([]byte, bool) {
	rac.mutex.RLock()
	defer rac.mutex.RUnlock()

	data, found := rac.cache[offset]
	if found && len(data) >= size {
		return data[:size], true
	}

	return nil, false
}

// Put 存储缓存数据
func (rac *ReadAheadCache) Put(offset int64, data []byte) {
	rac.mutex.Lock()
	defer rac.mutex.Unlock()

	// 检查缓存大小限制
	if rac.totalSize+len(data) > rac.maxSize {
		rac.evictOldest()
	}

	rac.cache[offset] = make([]byte, len(data))
	copy(rac.cache[offset], data)
	rac.totalSize += len(data)
}

// evictOldest 驱逐最旧的缓存条目
func (rac *ReadAheadCache) evictOldest() {
	// 简化实现：清空所有缓存
	rac.cache = make(map[int64][]byte)
	rac.totalSize = 0
}

// WriteBuffer 写缓冲区
type WriteBuffer struct {
	buffer       []byte
	maxSize      int
	syncInterval time.Duration
	lastSync     time.Time
	mutex        sync.Mutex
}

// NewWriteBuffer 创建写缓冲区
func NewWriteBuffer(maxSize int, syncInterval time.Duration) *WriteBuffer {
	return &WriteBuffer{
		buffer:       make([]byte, 0, maxSize),
		maxSize:      maxSize,
		syncInterval: syncInterval,
		lastSync:     time.Now(),
	}
}

// Write 写入缓冲区
func (wb *WriteBuffer) Write(file *os.File, data []byte) (int, error) {
	wb.mutex.Lock()
	defer wb.mutex.Unlock()

	// 检查是否需要刷新
	if len(wb.buffer)+len(data) > wb.maxSize || time.Since(wb.lastSync) > wb.syncInterval {
		if err := wb.flushLocked(file); err != nil {
			return 0, err
		}
	}

	// 添加到缓冲区
	wb.buffer = append(wb.buffer, data...)
	return len(data), nil
}

// Flush 刷新缓冲区
func (wb *WriteBuffer) Flush(file *os.File) error {
	wb.mutex.Lock()
	defer wb.mutex.Unlock()
	return wb.flushLocked(file)
}

// flushLocked 刷新缓冲区（已加锁）
func (wb *WriteBuffer) flushLocked(file *os.File) error {
	if len(wb.buffer) == 0 {
		return nil
	}

	_, err := file.Write(wb.buffer)
	if err != nil {
		return err
	}

	wb.buffer = wb.buffer[:0]
	wb.lastSync = time.Now()
	return nil
}

// 统计更新方法
func (io *IOOptimizer) updateReadStats(bytes int, latency time.Duration) {
	io.stats.mutex.Lock()
	defer io.stats.mutex.Unlock()

	io.stats.BytesRead += int64(bytes)
	io.stats.ReadOperations++
	io.stats.ReadLatency += latency
}

func (io *IOOptimizer) updateWriteStats(bytes int, latency time.Duration) {
	io.stats.mutex.Lock()
	defer io.stats.mutex.Unlock()

	io.stats.BytesWritten += int64(bytes)
	io.stats.WriteOperations++
	io.stats.WriteLatency += latency
}

func (io *IOOptimizer) updateCacheStats(hit bool) {
	io.stats.mutex.Lock()
	defer io.stats.mutex.Unlock()

	if hit {
		io.stats.CacheHits++
	} else {
		io.stats.CacheMisses++
	}
}

// GetStats 获取I/O统计信息
func (io *IOOptimizer) GetStats() *IOStats {
	io.stats.mutex.RLock()
	defer io.stats.mutex.RUnlock()

	return &IOStats{
		BytesRead:       io.stats.BytesRead,
		BytesWritten:    io.stats.BytesWritten,
		ReadOperations:  io.stats.ReadOperations,
		WriteOperations: io.stats.WriteOperations,
		ReadLatency:     io.stats.ReadLatency,
		WriteLatency:    io.stats.WriteLatency,
		CacheHits:       io.stats.CacheHits,
		CacheMisses:     io.stats.CacheMisses,
		StartTime:       io.stats.StartTime,
	}
}

// String 返回I/O统计信息的字符串表示
func (ios *IOStats) String() string {
	ios.mutex.RLock()
	defer ios.mutex.RUnlock()

	duration := time.Since(ios.StartTime)
	readThroughput := float64(ios.BytesRead) / duration.Seconds()
	writeThroughput := float64(ios.BytesWritten) / duration.Seconds()

	avgReadLatency := time.Duration(0)
	if ios.ReadOperations > 0 {
		avgReadLatency = ios.ReadLatency / time.Duration(ios.ReadOperations)
	}

	avgWriteLatency := time.Duration(0)
	if ios.WriteOperations > 0 {
		avgWriteLatency = ios.WriteLatency / time.Duration(ios.WriteOperations)
	}

	cacheHitRate := float64(0)
	totalCacheOps := ios.CacheHits + ios.CacheMisses
	if totalCacheOps > 0 {
		cacheHitRate = float64(ios.CacheHits) / float64(totalCacheOps) * 100
	}

	return fmt.Sprintf(`I/O性能统计:
  读取字节数: %d
  写入字节数: %d
  读操作数: %d
  写操作数: %d
  读取吞吐量: %.2f KB/s
  写入吞吐量: %.2f KB/s
  平均读延迟: %v
  平均写延迟: %v
  缓存命中率: %.2f%%
  运行时间: %v`,
		ios.BytesRead,
		ios.BytesWritten,
		ios.ReadOperations,
		ios.WriteOperations,
		readThroughput/1024,
		writeThroughput/1024,
		avgReadLatency,
		avgWriteLatency,
		cacheHitRate,
		duration)
}
