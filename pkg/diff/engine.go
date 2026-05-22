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

	// 优化操作：合并连续的相同类型操作
	optimizer := NewOptimizer(nil)
	delta = optimizer.OptimizeDelta(delta)

	return delta, nil
}

// generateDeltaWithRollingHash 使用滚动哈希生成差异
func (e *Engine) generateDeltaWithRollingHash(newFile *os.File, signature *Signature, delta *Delta) error {
	window := make([]byte, e.config.BlockSize)
	n, err := io.ReadFull(newFile, window)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return NewDiffError("read new file", "", err)
	}

	var unmatchedStart int64
	var unmatchedData []byte

	// 创建文件哈希器
	var fileHasher hash.Hash
	if e.config.EnableSHA256 {
		fileHasher = sha256.New()
	}
	if fileHasher != nil && n > 0 {
		fileHasher.Write(window[:n])
	}

	if n < e.config.BlockSize {
		e.processTailData(delta, signature, window[:n], 0, &unmatchedStart, &unmatchedData)
		e.flushInsert(delta, unmatchedStart, unmatchedData)
		e.setDeltaChecksum(delta, fileHasher)
		return nil
	}

	basePow := calculateBasePow(e.config.BlockSize)
	windowHash := hexhash.FastHash(window)
	windowStart := int64(0)
	windowIndex := 0
	oneByte := make([]byte, 1)

	for {
		var matchedBlock *Block
		if _, exists := signature.Blocks[windowHash]; exists {
			matchedBlock = signature.FindBlock(windowHash, orderedWindow(window, windowIndex))
		}

		if matchedBlock != nil {
			e.flushInsert(delta, unmatchedStart, unmatchedData)
			unmatchedData = unmatchedData[:0]
			delta.AddOperation(Operation{
				Type:      OpCopy,
				Offset:    windowStart,
				Size:      matchedBlock.Size,
				SrcOffset: matchedBlock.Offset,
			})

			windowStart += int64(matchedBlock.Size)
			n, err = io.ReadFull(newFile, window)
			windowIndex = 0
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				return NewDiffError("read new file", "", err)
			}
			if fileHasher != nil && n > 0 {
				fileHasher.Write(window[:n])
			}
			if n < e.config.BlockSize {
				e.processTailData(delta, signature, window[:n], windowStart, &unmatchedStart, &unmatchedData)
				break
			}
			windowHash = hexhash.FastHash(window)
			continue
		}

		oldByte := window[windowIndex]
		e.appendUnmatchedByte(oldByte, windowStart, &unmatchedStart, &unmatchedData)

		n, err = newFile.Read(oneByte)
		if err != nil && err != io.EOF {
			return NewDiffError("read new file", "", err)
		}
		if n == 0 {
			e.appendRemainingWindow(window, windowIndex, windowStart+1, &unmatchedStart, &unmatchedData)
			break
		}
		if fileHasher != nil {
			fileHasher.Write(oneByte[:n])
		}

		windowHash = rollBlockHash(windowHash, oldByte, oneByte[0], basePow)
		window[windowIndex] = oneByte[0]
		windowIndex = (windowIndex + 1) % e.config.BlockSize
		windowStart++
	}

	e.flushInsert(delta, unmatchedStart, unmatchedData)
	e.setDeltaChecksum(delta, fileHasher)
	return nil
}

func calculateBasePow(blockSize int) uint64 {
	basePow := uint64(1)
	for i := 0; i < blockSize-1; i++ {
		basePow = (basePow * hexhash.RollingHashBase) % hexhash.RollingHashMod
	}
	return basePow
}

func rollBlockHash(current uint64, oldByte, newByte byte, basePow uint64) uint64 {
	removed := (uint64(oldByte) * basePow) % hexhash.RollingHashMod
	next := (current + hexhash.RollingHashMod - removed) % hexhash.RollingHashMod
	next = (next*hexhash.RollingHashBase + uint64(newByte)) % hexhash.RollingHashMod
	return next
}

func orderedWindow(window []byte, startIndex int) []byte {
	if startIndex == 0 {
		return window
	}
	ordered := make([]byte, 0, len(window))
	ordered = append(ordered, window[startIndex:]...)
	ordered = append(ordered, window[:startIndex]...)
	return ordered
}

func (e *Engine) processTailData(delta *Delta, signature *Signature, data []byte, offset int64, unmatchedStart *int64, unmatchedData *[]byte) {
	if len(data) == 0 {
		return
	}
	blockHash := hexhash.FastHash(data)
	if _, exists := signature.Blocks[blockHash]; exists {
		if matchedBlock := signature.FindBlock(blockHash, data); matchedBlock != nil {
			e.flushInsert(delta, *unmatchedStart, *unmatchedData)
			*unmatchedData = (*unmatchedData)[:0]
			delta.AddOperation(Operation{
				Type:      OpCopy,
				Offset:    offset,
				Size:      matchedBlock.Size,
				SrcOffset: matchedBlock.Offset,
			})
			return
		}
	}
	e.appendUnmatchedBytes(data, offset, unmatchedStart, unmatchedData)
}

func (e *Engine) appendUnmatchedByte(b byte, offset int64, unmatchedStart *int64, unmatchedData *[]byte) {
	if len(*unmatchedData) == 0 {
		*unmatchedStart = offset
	}
	*unmatchedData = append(*unmatchedData, b)
}

func (e *Engine) appendUnmatchedBytes(data []byte, offset int64, unmatchedStart *int64, unmatchedData *[]byte) {
	if len(data) == 0 {
		return
	}
	if len(*unmatchedData) == 0 {
		*unmatchedStart = offset
	}
	*unmatchedData = append(*unmatchedData, data...)
}

func (e *Engine) appendRemainingWindow(window []byte, startIndex int, offset int64, unmatchedStart *int64, unmatchedData *[]byte) {
	if startIndex+1 < len(window) {
		e.appendUnmatchedBytes(window[startIndex+1:], offset, unmatchedStart, unmatchedData)
	}
	if startIndex > 0 {
		nextOffset := offset + int64(len(window)-startIndex-1)
		e.appendUnmatchedBytes(window[:startIndex], nextOffset, unmatchedStart, unmatchedData)
	}
}

func (e *Engine) flushInsert(delta *Delta, unmatchedStart int64, unmatchedData []byte) {
	if len(unmatchedData) == 0 {
		return
	}
	dataCopy := make([]byte, len(unmatchedData))
	copy(dataCopy, unmatchedData)
	delta.AddOperation(Operation{
		Type:   OpInsert,
		Offset: unmatchedStart,
		Size:   len(unmatchedData),
		Data:   dataCopy,
	})
}

func (e *Engine) setDeltaChecksum(delta *Delta, fileHasher hash.Hash) {
	if fileHasher != nil {
		checksumSlice := fileHasher.Sum(nil)
		copy(delta.Checksum[:], checksumSlice)
	}
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
