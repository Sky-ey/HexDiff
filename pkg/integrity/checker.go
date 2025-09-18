package integrity

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"sync"
	"time"
)

// ChecksumType 校验和类型
type ChecksumType int

const (
	ChecksumSHA256 ChecksumType = iota
	ChecksumCRC32
	ChecksumMD5
)

// BlockChecksum 数据块校验和
type BlockChecksum struct {
	Offset   int64        // 块偏移量
	Size     int          // 块大小
	SHA256   [32]byte     // SHA-256校验和
	CRC32    uint32       // CRC32校验和
	Type     ChecksumType // 校验和类型
	Verified bool         // 是否已验证
}

// IntegrityChecker 完整性检查器
type IntegrityChecker struct {
	blockSize     int                      // 块大小
	enableSHA256  bool                     // 是否启用SHA-256
	enableCRC32   bool                     // 是否启用CRC32
	checksums     map[int64]*BlockChecksum // 块校验和映射
	mutex         sync.RWMutex             // 读写锁
	errorCallback func(error)              // 错误回调函数
}

// CheckerConfig 检查器配置
type CheckerConfig struct {
	BlockSize     int         // 块大小（默认64KB）
	EnableSHA256  bool        // 启用SHA-256校验
	EnableCRC32   bool        // 启用CRC32校验
	ErrorCallback func(error) // 错误回调函数
}

// DefaultCheckerConfig 默认检查器配置
func DefaultCheckerConfig() *CheckerConfig {
	return &CheckerConfig{
		BlockSize:    64 * 1024, // 64KB
		EnableSHA256: true,
		EnableCRC32:  true,
		ErrorCallback: func(err error) {
			fmt.Printf("完整性检查错误: %v\n", err)
		},
	}
}

// NewIntegrityChecker 创建新的完整性检查器
func NewIntegrityChecker(config *CheckerConfig) *IntegrityChecker {
	if config == nil {
		config = DefaultCheckerConfig()
	}

	return &IntegrityChecker{
		blockSize:     config.BlockSize,
		enableSHA256:  config.EnableSHA256,
		enableCRC32:   config.EnableCRC32,
		checksums:     make(map[int64]*BlockChecksum),
		errorCallback: config.ErrorCallback,
	}
}

// GenerateFileChecksums 生成文件的块级校验和
func (ic *IntegrityChecker) GenerateFileChecksums(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	// 清空现有校验和
	ic.checksums = make(map[int64]*BlockChecksum)

	buffer := make([]byte, ic.blockSize)
	var offset int64 = 0

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("读取文件失败: %w", err)
		}

		if n == 0 {
			break
		}

		blockData := buffer[:n]
		checksum := &BlockChecksum{
			Offset: offset,
			Size:   n,
		}

		// 计算SHA-256校验和
		if ic.enableSHA256 {
			sha256Hash := sha256.Sum256(blockData)
			checksum.SHA256 = sha256Hash
			checksum.Type = ChecksumSHA256
		}

		// 计算CRC32校验和
		if ic.enableCRC32 {
			checksum.CRC32 = crc32.ChecksumIEEE(blockData)
		}

		ic.checksums[offset] = checksum
		offset += int64(n)

		if err == io.EOF {
			break
		}
	}

	return nil
}

// VerifyBlock 验证数据块
func (ic *IntegrityChecker) VerifyBlock(offset int64, data []byte) error {
	ic.mutex.RLock()
	expectedChecksum, exists := ic.checksums[offset]
	ic.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("未找到偏移量 %d 的校验和", offset)
	}

	if len(data) != expectedChecksum.Size {
		return fmt.Errorf("数据块大小不匹配: 期望 %d，实际 %d", expectedChecksum.Size, len(data))
	}

	// 验证SHA-256
	if ic.enableSHA256 {
		actualSHA256 := sha256.Sum256(data)
		if actualSHA256 != expectedChecksum.SHA256 {
			return fmt.Errorf("SHA-256校验和不匹配: 偏移量 %d", offset)
		}
	}

	// 验证CRC32
	if ic.enableCRC32 {
		actualCRC32 := crc32.ChecksumIEEE(data)
		if actualCRC32 != expectedChecksum.CRC32 {
			return fmt.Errorf("CRC32校验和不匹配: 偏移量 %d", offset)
		}
	}

	// 标记为已验证
	ic.mutex.Lock()
	expectedChecksum.Verified = true
	ic.mutex.Unlock()

	return nil
}

// VerifyFile 验证整个文件
func (ic *IntegrityChecker) VerifyFile(filePath string) (*VerificationResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	result := &VerificationResult{
		FilePath:       filePath,
		StartTime:      time.Now(),
		TotalBlocks:    len(ic.checksums),
		VerifiedBlocks: 0,
		FailedBlocks:   0,
		Errors:         make([]error, 0),
	}

	buffer := make([]byte, ic.blockSize)
	var offset int64 = 0

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			result.Errors = append(result.Errors, fmt.Errorf("读取文件失败: %w", err))
			break
		}

		if n == 0 {
			break
		}

		blockData := buffer[:n]

		// 验证当前块
		if verifyErr := ic.VerifyBlock(offset, blockData); verifyErr != nil {
			result.FailedBlocks++
			result.Errors = append(result.Errors, verifyErr)
			if ic.errorCallback != nil {
				ic.errorCallback(verifyErr)
			}
		} else {
			result.VerifiedBlocks++
		}

		offset += int64(n)

		if err == io.EOF {
			break
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = result.FailedBlocks == 0

	return result, nil
}

// GetBlockChecksum 获取指定偏移量的块校验和
func (ic *IntegrityChecker) GetBlockChecksum(offset int64) (*BlockChecksum, bool) {
	ic.mutex.RLock()
	defer ic.mutex.RUnlock()

	checksum, exists := ic.checksums[offset]
	return checksum, exists
}

// GetAllChecksums 获取所有块校验和
func (ic *IntegrityChecker) GetAllChecksums() map[int64]*BlockChecksum {
	ic.mutex.RLock()
	defer ic.mutex.RUnlock()

	// 创建副本以避免并发访问问题
	result := make(map[int64]*BlockChecksum)
	for offset, checksum := range ic.checksums {
		result[offset] = &BlockChecksum{
			Offset:   checksum.Offset,
			Size:     checksum.Size,
			SHA256:   checksum.SHA256,
			CRC32:    checksum.CRC32,
			Type:     checksum.Type,
			Verified: checksum.Verified,
		}
	}

	return result
}

// VerificationResult 验证结果
type VerificationResult struct {
	FilePath       string        // 文件路径
	Success        bool          // 验证是否成功
	TotalBlocks    int           // 总块数
	VerifiedBlocks int           // 已验证块数
	FailedBlocks   int           // 失败块数
	StartTime      time.Time     // 开始时间
	EndTime        time.Time     // 结束时间
	Duration       time.Duration // 验证耗时
	Errors         []error       // 错误列表
}

// String 返回验证结果的字符串表示
func (vr *VerificationResult) String() string {
	status := "失败"
	if vr.Success {
		status = "成功"
	}

	return fmt.Sprintf(`文件完整性验证结果:
  文件: %s
  状态: %s
  总块数: %d
  验证成功: %d
  验证失败: %d
  验证耗时: %v
  错误数量: %d`,
		vr.FilePath,
		status,
		vr.TotalBlocks,
		vr.VerifiedBlocks,
		vr.FailedBlocks,
		vr.Duration,
		len(vr.Errors))
}

// StreamVerifier 流式验证器（用于实时验证）
type StreamVerifier struct {
	checker    *IntegrityChecker
	hasher     hash.Hash
	crc32Hash  hash.Hash32
	offset     int64
	buffer     []byte
	bufferSize int
}

// NewStreamVerifier 创建新的流式验证器
func NewStreamVerifier(checker *IntegrityChecker) *StreamVerifier {
	sv := &StreamVerifier{
		checker:    checker,
		offset:     0,
		bufferSize: checker.blockSize,
		buffer:     make([]byte, 0, checker.blockSize),
	}

	if checker.enableSHA256 {
		sv.hasher = sha256.New()
	}

	if checker.enableCRC32 {
		sv.crc32Hash = crc32.NewIEEE()
	}

	return sv
}

// Write 写入数据进行实时验证
func (sv *StreamVerifier) Write(data []byte) (int, error) {
	written := 0

	for len(data) > 0 {
		// 计算当前可以写入缓冲区的数据量
		available := sv.bufferSize - len(sv.buffer)
		toWrite := len(data)
		if toWrite > available {
			toWrite = available
		}

		// 写入缓冲区
		sv.buffer = append(sv.buffer, data[:toWrite]...)
		data = data[toWrite:]
		written += toWrite

		// 如果缓冲区满了，验证当前块
		if len(sv.buffer) == sv.bufferSize {
			if err := sv.verifyCurrentBlock(); err != nil {
				return written, err
			}
		}
	}

	return written, nil
}

// verifyCurrentBlock 验证当前缓冲区中的块
func (sv *StreamVerifier) verifyCurrentBlock() error {
	if len(sv.buffer) == 0 {
		return nil
	}

	// 验证块
	if err := sv.checker.VerifyBlock(sv.offset, sv.buffer); err != nil {
		return err
	}

	// 重置缓冲区和更新偏移量
	sv.offset += int64(len(sv.buffer))
	sv.buffer = sv.buffer[:0]

	return nil
}

// Flush 刷新剩余数据
func (sv *StreamVerifier) Flush() error {
	if len(sv.buffer) > 0 {
		return sv.verifyCurrentBlock()
	}
	return nil
}
