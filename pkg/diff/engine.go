package diff

import (
	"crypto/sha256"
	"hash"
	"hash/crc32"
	"io"
	"os"

	hexhash "github.com/Sky-ey/HexDiff/pkg/hash"
)

// Engine 差异检测引擎
type Engine struct {
	config *DiffConfig
}

// NewEngine 创建新的差异检测引擎
func NewEngine(config *DiffConfig) (*Engine, error) {
	if config == nil {
		config = DefaultDiffConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Engine{
		config: config,
	}, nil
}

// GenerateSignature 为文件生成签名
func (e *Engine) GenerateSignature(filePath string) (*Signature, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, NewDiffError("open file", filePath, err)
	}
	defer file.Close()

	// 获取文件大小
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, NewDiffError("stat file", filePath, err)
	}

	fileSize := fileInfo.Size()
	signature := NewSignature(e.config.BlockSize, fileSize)

	// 创建SHA-256哈希器用于整个文件
	var fileHasher hash.Hash
	if e.config.EnableSHA256 {
		fileHasher = sha256.New()
	}

	buffer := make([]byte, e.config.BlockSize)
	var offset int64 = 0

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, NewDiffError("read file", filePath, err)
		}

		if n == 0 {
			break
		}

		blockData := buffer[:n]

		// 更新文件哈希
		if fileHasher != nil {
			fileHasher.Write(blockData)
		}

		// 计算块的滚动哈希
		blockHash := hexhash.FastHash(blockData)

		// 计算块的CRC32校验和
		var checksum uint32
		if e.config.EnableCRC32 {
			checksum = crc32.ChecksumIEEE(blockData)
		}

		// 创建块
		block := Block{
			Offset:   offset,
			Size:     n,
			Hash:     blockHash,
			Checksum: checksum,
		}

		signature.AddBlock(block)
		offset += int64(n)

		if err == io.EOF {
			break
		}
	}

	// 设置文件校验和
	if fileHasher != nil {
		checksumSlice := fileHasher.Sum(nil)
		copy(signature.Checksum[:], checksumSlice)
	}

	return signature, nil
}

// GenerateDelta 生成两个文件之间的差异
func (e *Engine) GenerateDelta(oldFilePath, newFilePath string) (*Delta, error) {
	// 首先为旧文件生成签名
	signature, err := e.GenerateSignature(oldFilePath)
	if err != nil {
		return nil, err
	}

	// 打开新文件
	newFile, err := os.Open(newFilePath)
	if err != nil {
		return nil, NewDiffError("open new file", newFilePath, err)
	}
	defer newFile.Close()

	// 获取新文件大小
	newFileInfo, err := newFile.Stat()
	if err != nil {
		return nil, NewDiffError("stat new file", newFilePath, err)
	}

	delta := NewDelta(signature.FileSize, newFileInfo.Size())

	// 使用滚动哈希进行匹配
	err = e.generateDeltaWithRollingHash(newFile, signature, delta)
	if err != nil {
		return nil, err
	}

	return delta, nil
}

// generateDeltaWithRollingHash 使用滚动哈希生成差异
func (e *Engine) generateDeltaWithRollingHash(newFile *os.File, signature *Signature, delta *Delta) error {
	rollingHash := hexhash.NewRollingHash(e.config.WindowSize)
	buffer := make([]byte, e.config.BlockSize)
	var fileOffset int64 = 0
	var unmatchedStart int64 = 0
	var unmatchedData []byte

	// 创建文件哈希器
	var fileHasher hash.Hash
	if e.config.EnableSHA256 {
		fileHasher = sha256.New()
	}

	for {
		n, err := newFile.Read(buffer)
		if err != nil && err != io.EOF {
			return NewDiffError("read new file", "", err)
		}

		if n == 0 {
			break
		}

		blockData := buffer[:n]

		// 更新文件哈希
		if fileHasher != nil {
			fileHasher.Write(blockData)
		}

		// 处理当前块
		var _ *hexhash.RollingHash = rollingHash
		matched := e.processBlock(blockData, fileOffset, signature, delta, &unmatchedStart, &unmatchedData)

		if !matched {
			// 如果没有匹配，将数据添加到未匹配缓冲区
			unmatchedData = append(unmatchedData, blockData...)
		}

		fileOffset += int64(n)

		if err == io.EOF {
			break
		}
	}

	// 处理剩余的未匹配数据
	if len(unmatchedData) > 0 {
		delta.AddOperation(Operation{
			Type:   OpInsert,
			Offset: unmatchedStart,
			Size:   len(unmatchedData),
			Data:   unmatchedData,
		})
	}

	// 设置目标文件校验和
	if fileHasher != nil {
		checksumSlice := fileHasher.Sum(nil)
		copy(delta.Checksum[:], checksumSlice)
	}

	return nil
}

// processBlock 处理单个数据块
func (e *Engine) processBlock(blockData []byte, offset int64, signature *Signature, delta *Delta, unmatchedStart *int64, unmatchedData *[]byte) bool {
	// 计算块哈希
	blockHash := hexhash.FastHash(blockData)

	// 查找匹配的块
	matchedBlock := signature.FindBlock(blockHash, blockData)

	if matchedBlock != nil {
		// 找到匹配，先处理未匹配的数据
		if len(*unmatchedData) > 0 {
			delta.AddOperation(Operation{
				Type:   OpInsert,
				Offset: *unmatchedStart,
				Size:   len(*unmatchedData),
				Data:   *unmatchedData,
			})
			*unmatchedData = (*unmatchedData)[:0] // 清空缓冲区
		}

		// 添加复制操作
		delta.AddOperation(Operation{
			Type:      OpCopy,
			Offset:    offset,
			Size:      matchedBlock.Size,
			SrcOffset: matchedBlock.Offset,
		})

		// 更新未匹配数据的起始位置
		*unmatchedStart = offset + int64(matchedBlock.Size)

		return true
	}

	// 没有找到匹配
	if len(*unmatchedData) == 0 {
		*unmatchedStart = offset
	}

	return false
}

// GetConfig 获取引擎配置
func (e *Engine) GetConfig() *DiffConfig {
	return e.config
}

// SetConfig 设置引擎配置
func (e *Engine) SetConfig(config *DiffConfig) error {
	if err := config.Validate(); err != nil {
		return err
	}
	e.config = config
	return nil
}
