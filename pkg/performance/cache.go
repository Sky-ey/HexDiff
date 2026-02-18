package performance

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// LRUCache LRU缓存实现
type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	lruList  *list.List
	mutex    sync.RWMutex
	stats    *CacheStats
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key        string
	Value      any
	AccessTime time.Time
	Size       int64
}

// CacheStats 缓存统计
type CacheStats struct {
	Hits       int64 // 命中次数
	Misses     int64 // 未命中次数
	Evictions  int64 // 驱逐次数
	TotalSize  int64 // 总大小
	EntryCount int   // 条目数量
	mutex      sync.RWMutex
}

// NewLRUCache 创建新的LRU缓存
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		lruList:  list.New(),
		stats:    &CacheStats{},
	}
}

// Get 获取缓存值
func (c *LRUCache) Get(key string) (any, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if elem, found := c.cache[key]; found {
		// 移动到链表前端
		c.lruList.MoveToFront(elem)
		entry := elem.Value.(*CacheEntry)
		entry.AccessTime = time.Now()

		// 更新统计
		c.stats.mutex.Lock()
		c.stats.Hits++
		c.stats.mutex.Unlock()

		return entry.Value, true
	}

	// 更新统计
	c.stats.mutex.Lock()
	c.stats.Misses++
	c.stats.mutex.Unlock()

	return nil, false
}

// Put 设置缓存值
func (c *LRUCache) Put(key string, value any) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 如果键已存在，更新值
	if elem, found := c.cache[key]; found {
		c.lruList.MoveToFront(elem)
		entry := elem.Value.(*CacheEntry)
		oldSize := entry.Size
		entry.Value = value
		entry.AccessTime = time.Now()
		entry.Size = c.calculateSize(value)

		// 更新统计
		c.stats.mutex.Lock()
		c.stats.TotalSize += entry.Size - oldSize
		c.stats.mutex.Unlock()
		return
	}

	// 创建新条目
	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		AccessTime: time.Now(),
		Size:       c.calculateSize(value),
	}

	elem := c.lruList.PushFront(entry)
	c.cache[key] = elem

	// 更新统计
	c.stats.mutex.Lock()
	c.stats.TotalSize += entry.Size
	c.stats.EntryCount++
	c.stats.mutex.Unlock()

	// 检查容量限制
	if c.lruList.Len() > c.capacity {
		c.evictOldest()
	}
}

// evictOldest 驱逐最旧的条目
func (c *LRUCache) evictOldest() {
	elem := c.lruList.Back()
	if elem != nil {
		c.lruList.Remove(elem)
		entry := elem.Value.(*CacheEntry)
		delete(c.cache, entry.Key)

		// 更新统计
		c.stats.mutex.Lock()
		c.stats.Evictions++
		c.stats.TotalSize -= entry.Size
		c.stats.EntryCount--
		c.stats.mutex.Unlock()
	}
}

// calculateSize 计算值的大小（简化实现）
func (c *LRUCache) calculateSize(value any) int64 {
	switch v := value.(type) {
	case []byte:
		return int64(len(v))
	case string:
		return int64(len(v))
	default:
		return 64 // 默认大小
	}
}

// Clear 清空缓存
func (c *LRUCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache = make(map[string]*list.Element)
	c.lruList = list.New()

	c.stats.mutex.Lock()
	c.stats.TotalSize = 0
	c.stats.EntryCount = 0
	c.stats.mutex.Unlock()
}

// Size 获取缓存大小
func (c *LRUCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.lruList.Len()
}

// GetStats 获取缓存统计
func (c *LRUCache) GetStats() *CacheStats {
	c.stats.mutex.RLock()
	defer c.stats.mutex.RUnlock()

	return &CacheStats{
		Hits:       c.stats.Hits,
		Misses:     c.stats.Misses,
		Evictions:  c.stats.Evictions,
		TotalSize:  c.stats.TotalSize,
		EntryCount: c.stats.EntryCount,
	}
}

// HitRate 获取缓存命中率
func (c *LRUCache) HitRate() float64 {
	c.stats.mutex.RLock()
	defer c.stats.mutex.RUnlock()

	total := c.stats.Hits + c.stats.Misses
	if total == 0 {
		return 0
	}
	return float64(c.stats.Hits) / float64(total)
}

// String 返回缓存统计的字符串表示
func (cs *CacheStats) String() string {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	total := cs.Hits + cs.Misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(cs.Hits) / float64(total) * 100
	}

	return fmt.Sprintf(`缓存统计:
  命中次数: %d
  未命中次数: %d
  驱逐次数: %d
  命中率: %.2f%%
  条目数量: %d
  总大小: %d 字节`,
		cs.Hits,
		cs.Misses,
		cs.Evictions,
		hitRate,
		cs.EntryCount,
		cs.TotalSize)
}

// MemoryPool 内存池
type MemoryPool struct {
	pools map[int]*sync.Pool // 不同大小的内存池
	mutex sync.RWMutex
}

// NewMemoryPool 创建新的内存池
func NewMemoryPool() *MemoryPool {
	mp := &MemoryPool{
		pools: make(map[int]*sync.Pool),
	}

	// 预定义常用大小的内存池
	sizes := []int{1024, 4096, 8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576}
	for _, size := range sizes {
		mp.createPool(size)
	}

	return mp
}

// createPool 创建指定大小的内存池
func (mp *MemoryPool) createPool(size int) {
	mp.pools[size] = &sync.Pool{
		New: func() any {
			return make([]byte, size)
		},
	}
}

// Get 获取指定大小的内存块
func (mp *MemoryPool) Get(size int) []byte {
	// 找到最接近的池大小
	poolSize := mp.findPoolSize(size)

	mp.mutex.RLock()
	pool, exists := mp.pools[poolSize]
	mp.mutex.RUnlock()

	if !exists {
		mp.mutex.Lock()
		mp.createPool(poolSize)
		pool = mp.pools[poolSize]
		mp.mutex.Unlock()
	}

	buf := pool.Get().([]byte)
	return buf[:size] // 返回所需大小的切片
}

// Put 归还内存块
func (mp *MemoryPool) Put(buf []byte) {
	if buf == nil {
		return
	}

	// 重置切片长度到容量
	buf = buf[:cap(buf)]
	poolSize := cap(buf)

	mp.mutex.RLock()
	pool, exists := mp.pools[poolSize]
	mp.mutex.RUnlock()

	if exists {
		pool.Put(buf)
	}
}

// findPoolSize 找到最接近的池大小
func (mp *MemoryPool) findPoolSize(size int) int {
	// 找到大于等于size的最小2的幂
	poolSize := 1024
	for poolSize < size {
		poolSize *= 2
	}
	return poolSize
}

// BufferPool 缓冲区池
type BufferPool struct {
	pool *sync.Pool
	size int
}

// NewBufferPool 创建新的缓冲区池
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		size: size,
		pool: &sync.Pool{
			New: func() any {
				return make([]byte, size)
			},
		},
	}
}

// Get 获取缓冲区
func (bp *BufferPool) Get() []byte {
	return bp.pool.Get().([]byte)
}

// Put 归还缓冲区
func (bp *BufferPool) Put(buf []byte) {
	if len(buf) == bp.size {
		bp.pool.Put(buf)
	}
}
