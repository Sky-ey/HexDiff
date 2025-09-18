package diff

import (
	"crypto/sha256"
	"hash/crc32"
)

// BlockSize 默认块大小
const (
	DefaultBlockSize = 4096  // 4KB
	MinBlockSize     = 64    // 最小块大小
	MaxBlockSize     = 65536 // 最大块大小 64KB
)

// OperationType 操作类型
type OperationType uint8

const (
	OpCopy   OperationType = iota // 复制操作
	OpInsert                      // 插入操作
	OpDelete                      // 删除操作
)

// String 返回操作类型的字符串表示
func (op OperationType) String() string {
	switch op {
	case OpCopy:
		return "COPY"
	case OpInsert:
		return "INSERT"
	case OpDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

// Block 数据块结构
type Block struct {
	Offset   int64  // 在原文件中的偏移量
	Size     int    // 块大小
	Hash     uint64 // 滚动哈希值
	Checksum uint32 // CRC32校验和
	Data     []byte // 块数据（仅在需要时存储）
}

// Operation 差异操作
type Operation struct {
	Type      OperationType // 操作类型
	Offset    int64         // 目标文件偏移量
	Size      int           // 数据大小
	Data      []byte        // 操作数据（插入时使用）
	SrcOffset int64         // 源文件偏移量（复制时使用）
}

// Signature 文件签名
type Signature struct {
	BlockSize int                // 块大小
	Blocks    map[uint64][]Block // 哈希值到块的映射
	FileSize  int64              // 文件大小
	Checksum  [32]byte           // 文件SHA-256校验和
}

// NewSignature 创建新的文件签名
func NewSignature(blockSize int, fileSize int64) *Signature {
	return &Signature{
		BlockSize: blockSize,
		Blocks:    make(map[uint64][]Block),
		FileSize:  fileSize,
	}
}

// AddBlock 添加块到签名
func (s *Signature) AddBlock(block Block) {
	s.Blocks[block.Hash] = append(s.Blocks[block.Hash], block)
}

// FindBlock 根据哈希值查找匹配的块
func (s *Signature) FindBlock(hash uint64, data []byte) *Block {
	blocks, exists := s.Blocks[hash]
	if !exists {
		return nil
	}

	// 计算数据的CRC32校验和
	checksum := crc32.ChecksumIEEE(data)

	// 查找校验和匹配的块
	for i := range blocks {
		if blocks[i].Checksum == checksum {
			return &blocks[i]
		}
	}

	return nil
}

// Delta 差异结果
type Delta struct {
	Operations []Operation // 操作列表
	SourceSize int64       // 源文件大小
	TargetSize int64       // 目标文件大小
	Checksum   [32]byte    // 目标文件SHA-256校验和
}

// NewDelta 创建新的差异结果
func NewDelta(sourceSize, targetSize int64) *Delta {
	return &Delta{
		Operations: make([]Operation, 0),
		SourceSize: sourceSize,
		TargetSize: targetSize,
	}
}

// AddOperation 添加操作到差异结果
func (d *Delta) AddOperation(op Operation) {
	d.Operations = append(d.Operations, op)
}

// SetChecksum 设置目标文件校验和
func (d *Delta) SetChecksum(data []byte) {
	d.Checksum = sha256.Sum256(data)
}

// MatchResult 匹配结果
type MatchResult struct {
	Found  bool   // 是否找到匹配
	Block  *Block // 匹配的块
	Offset int64  // 在目标文件中的偏移量
	Size   int    // 匹配的大小
}

// DiffConfig 差异检测配置
type DiffConfig struct {
	BlockSize    int   // 块大小
	WindowSize   int   // 滚动哈希窗口大小
	EnableCRC32  bool  // 是否启用CRC32校验
	EnableSHA256 bool  // 是否启用SHA256校验
	MaxMemory    int64 // 最大内存使用量（字节）
}

// DefaultDiffConfig 默认差异检测配置
func DefaultDiffConfig() *DiffConfig {
	return &DiffConfig{
		BlockSize:    DefaultBlockSize,
		WindowSize:   64,
		EnableCRC32:  true,
		EnableSHA256: true,
		MaxMemory:    100 * 1024 * 1024, // 100MB
	}
}

// Validate 验证配置参数
func (c *DiffConfig) Validate() error {
	if c.BlockSize < MinBlockSize || c.BlockSize > MaxBlockSize {
		return ErrInvalidBlockSize
	}
	if c.WindowSize < 8 || c.WindowSize > c.BlockSize {
		return ErrInvalidWindowSize
	}
	if c.MaxMemory < 1024*1024 { // 最小1MB
		return ErrInvalidMaxMemory
	}
	return nil
}
