package hash

const (
	// RollingHashBase 滚动哈希的基数
	RollingHashBase = 257
	// RollingHashMod 滚动哈希的模数
	RollingHashMod = 1000000007
)

// RollingHash 滚动哈希结构
type RollingHash struct {
	hash       uint64 // 当前哈希值
	base       uint64 // 基数
	mod        uint64 // 模数
	window     []byte // 当前窗口数据
	windowSize int    // 窗口大小
	basePow    uint64 // base^(windowSize-1) mod mod
}

// NewRollingHash 创建新的滚动哈希实例
func NewRollingHash(windowSize int) *RollingHash {
	rh := &RollingHash{
		base:       RollingHashBase,
		mod:        RollingHashMod,
		window:     make([]byte, 0, windowSize),
		windowSize: windowSize,
		basePow:    1,
	}

	// 计算 base^(windowSize-1) mod mod
	for i := 0; i < windowSize-1; i++ {
		rh.basePow = (rh.basePow * rh.base) % rh.mod
	}

	return rh
}

// Add 向滚动哈希中添加一个字节
func (rh *RollingHash) Add(b byte) {
	if len(rh.window) < rh.windowSize {
		// 窗口未满，直接添加
		rh.window = append(rh.window, b)
		rh.hash = (rh.hash*rh.base + uint64(b)) % rh.mod
	} else {
		// 窗口已满，滚动更新
		oldByte := rh.window[0]
		copy(rh.window, rh.window[1:])
		rh.window[rh.windowSize-1] = b

		// 更新哈希值：移除最老的字节，添加新字节
		rh.hash = (rh.hash + rh.mod - (uint64(oldByte)*rh.basePow)%rh.mod) % rh.mod
		rh.hash = (rh.hash*rh.base + uint64(b)) % rh.mod
	}
}

// Hash 获取当前哈希值
func (rh *RollingHash) Hash() uint64 {
	return rh.hash
}

// Window 获取当前窗口数据的副本
func (rh *RollingHash) Window() []byte {
	result := make([]byte, len(rh.window))
	copy(result, rh.window)
	return result
}

// IsFull 检查窗口是否已满
func (rh *RollingHash) IsFull() bool {
	return len(rh.window) == rh.windowSize
}

// Reset 重置滚动哈希
func (rh *RollingHash) Reset() {
	rh.hash = 0
	rh.window = rh.window[:0]
}

// Size 返回当前窗口大小
func (rh *RollingHash) Size() int {
	return len(rh.window)
}

// FastHash 快速计算字节切片的哈希值（非滚动）
func FastHash(data []byte) uint64 {
	var hash uint64 = 0
	base := uint64(RollingHashBase)
	mod := uint64(RollingHashMod)

	for _, b := range data {
		hash = (hash*base + uint64(b)) % mod
	}

	return hash
}

// Adler32RollingHash Adler-32滚动哈希实现（备选方案）
type Adler32RollingHash struct {
	a, b       uint32
	window     []byte
	windowSize int
}

// NewAdler32RollingHash 创建Adler-32滚动哈希
func NewAdler32RollingHash(windowSize int) *Adler32RollingHash {
	return &Adler32RollingHash{
		a:          1,
		b:          0,
		window:     make([]byte, 0, windowSize),
		windowSize: windowSize,
	}
}

// Add 添加字节到Adler-32滚动哈希
func (ah *Adler32RollingHash) Add(data byte) {
	if len(ah.window) < ah.windowSize {
		ah.window = append(ah.window, data)
		ah.a = (ah.a + uint32(data)) % 65521
		ah.b = (ah.b + ah.a) % 65521
	} else {
		oldByte := ah.window[0]
		copy(ah.window, ah.window[1:])
		ah.window[ah.windowSize-1] = data

		ah.a = (ah.a - uint32(oldByte) + uint32(data)) % 65521
		ah.b = (ah.b - uint32(ah.windowSize)*uint32(oldByte) + ah.a - 1) % 65521
	}
}

// Hash 获取Adler-32哈希值
func (ah *Adler32RollingHash) Hash() uint32 {
	return (ah.b << 16) | ah.a
}

// Reset 重置Adler-32哈希
func (ah *Adler32RollingHash) Reset() {
	ah.a = 1
	ah.b = 0
	ah.window = ah.window[:0]
}
