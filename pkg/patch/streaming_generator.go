package patch

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/Sky-ey/HexDiff/pkg/diff"
)

// StreamingPatchGenerator 流式补丁生成器（适用于大文件）
type StreamingPatchGenerator struct {
	engine       *diff.Engine
	compression  CompressionType
	patchFile    *os.File
	writer       *bufio.Writer
	dataWriter   io.Writer
	dataFilePath string
	dataFile     *os.File
	header       *PatchHeader
	operations   []PatchOperation
	dataOffset   uint32
}

// NewStreamingPatchGenerator 创建新的流式补丁生成器
func NewStreamingPatchGenerator(engine *diff.Engine, compression CompressionType) *StreamingPatchGenerator {
	return &StreamingPatchGenerator{
		engine:      engine,
		compression: compression,
		operations:  make([]PatchOperation, 0),
		dataOffset:  0,
	}
}

// GeneratePatchStreaming 流式生成补丁文件（适用于大文件）
func (spg *StreamingPatchGenerator) GeneratePatchStreaming(oldFilePath, newFilePath, patchPath string) (*PatchInfo, error) {
	var err error

	// 创建补丁文件
	spg.patchFile, err = os.Create(patchPath)
	if err != nil {
		return nil, fmt.Errorf("create patch file: %w", err)
	}
	spg.writer = bufio.NewWriter(spg.patchFile)

	// 创建数据文件（用于存储插入数据）
	spg.dataFilePath = patchPath + ".tmpdata"
	spg.dataFile, err = os.Create(spg.dataFilePath)
	if err != nil {
		spg.patchFile.Close()
		return nil, fmt.Errorf("create data file: %w", err)
	}

	// 设置数据写入器
	if spg.compression == CompressionGzip {
		gzipWriter := gzip.NewWriter(spg.dataFile)
		spg.dataWriter = gzipWriter
	} else {
		spg.dataWriter = spg.dataFile
	}

	// 初始化补丁头
	spg.header = NewPatchHeader()
	spg.header.Compression = spg.compression

	// 获取文件信息
	oldStat, err := os.Stat(oldFilePath)
	if err != nil {
		spg.cleanup()
		return nil, fmt.Errorf("stat old file: %w", err)
	}

	newStat, err := os.Stat(newFilePath)
	if err != nil {
		spg.cleanup()
		return nil, fmt.Errorf("stat new file: %w", err)
	}

	spg.header.SourceSize = oldStat.Size()
	spg.header.TargetSize = newStat.Size()

	// 生成差异
	delta, err := spg.engine.GenerateDelta(oldFilePath, newFilePath)
	if err != nil {
		spg.cleanup()
		return nil, fmt.Errorf("generate delta: %w", err)
	}

	spg.header.TargetChecksum = delta.Checksum

	// 计算源文件校验和
	sourceChecksum, err := spg.calculateFileChecksumStreaming(oldFilePath)
	if err != nil {
		spg.cleanup()
		return nil, fmt.Errorf("calculate source checksum: %w", err)
	}

	spg.header.SourceChecksum = sourceChecksum

	// 转换操作并流式写入插入数据
	for _, op := range delta.Operations {
		patchOp := PatchOperation{
			Offset: uint64(op.Offset),
			Size:   uint32(op.Size),
		}

		switch op.Type {
		case diff.OpCopy:
			patchOp.Type = 0
			patchOp.SrcOffset = uint64(op.SrcOffset)
		case diff.OpInsert:
			patchOp.Type = 1
			dataOffset, err := spg.writeInsertDataStreaming(op.Data)
			if err != nil {
				spg.cleanup()
				return nil, fmt.Errorf("write insert data: %w", err)
			}
			patchOp.DataOffset = dataOffset
		case diff.OpDelete:
			patchOp.Type = 2
		default:
			spg.cleanup()
			return nil, fmt.Errorf("unknown operation type: %v", op.Type)
		}

		spg.operations = append(spg.operations, patchOp)
	}

	// 关闭数据写入器
	if err := spg.closeDataWriter(); err != nil {
		spg.cleanup()
		return nil, fmt.Errorf("close data writer: %w", err)
	}

	// 更新补丁头
	spg.header.OperationCount = uint32(len(spg.operations))
	spg.header.DataOffset = uint32(HeaderSize + len(spg.operations)*OperationSize)

	// 写入补丁文件
	if err := spg.writePatchFile(); err != nil {
		spg.cleanup()
		return nil, fmt.Errorf("write patch file: %w", err)
	}

	// 删除临时数据文件
	if err := os.Remove(spg.dataFilePath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("警告: 无法删除临时数据文件 %s: %v\n", spg.dataFilePath, err)
	}

	// 获取补丁文件信息
	patchInfo, err := spg.getPatchFileInfo(patchPath, oldFilePath, newFilePath)
	if err != nil {
		return nil, fmt.Errorf("get patch info: %w", err)
	}

	return patchInfo, nil
}

// writeInsertDataStreaming 流式写入插入数据
func (spg *StreamingPatchGenerator) writeInsertDataStreaming(data []byte) (uint32, error) {
	offset := spg.dataOffset

	if len(data) == 0 {
		return offset, nil
	}

	// 分块写入数据
	chunkSize := 64 * 1024 // 64KB块
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		_, err := spg.dataWriter.Write(data[i:end])
		if err != nil {
			return 0, err
		}
	}

	spg.dataOffset += uint32(len(data))
	return offset, nil
}

// closeDataWriter 关闭数据写入器
func (spg *StreamingPatchGenerator) closeDataWriter() error {
	var err error

	// 如果使用 Gzip 压缩，需要关闭 gzip writer
	if spg.compression == CompressionGzip {
		if gzipWriter, ok := spg.dataWriter.(*gzip.Writer); ok {
			err = gzipWriter.Close()
		}
	} else if spg.dataFile != nil {
		// 如果不使用压缩，需要 Sync 确保数据写入磁盘
		if syncErr := spg.dataFile.Sync(); syncErr != nil {
			if err == nil {
				err = syncErr
			}
		}
	}

	// 关闭数据文件
	if spg.dataFile != nil {
		closeErr := spg.dataFile.Close()
		if err == nil {
			err = closeErr
		}
		spg.dataFile = nil
	}

	return err
}

// writePatchFile 写入补丁文件
func (spg *StreamingPatchGenerator) writePatchFile() error {
	// 写入文件头
	headerData := spg.header.Marshal()
	if _, err := spg.writer.Write(headerData); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// 写入操作列表
	for _, op := range spg.operations {
		opData := op.Marshal()
		if _, err := spg.writer.Write(opData); err != nil {
			return fmt.Errorf("write operation: %w", err)
		}
	}

	// 将数据文件内容复制到补丁文件
	dataFile, err := os.Open(spg.dataFilePath)
	if err != nil {
		return fmt.Errorf("open data file: %w", err)
	}
	defer dataFile.Close()

	_, err = io.Copy(spg.writer, dataFile)
	if err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	// 刷新缓冲区
	if err := spg.writer.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	return nil
}

// calculateFileChecksumStreaming 流式计算文件校验和
func (spg *StreamingPatchGenerator) calculateFileChecksumStreaming(filePath string) ([32]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return [32]byte{}, err
	}
	defer file.Close()

	hasher := sha256.New()
	buffer := make([]byte, 64*1024)

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			hasher.Write(buffer[:n])
		}
		if err != nil {
			break
		}
	}

	var checksum [32]byte
	copy(checksum[:], hasher.Sum(nil))
	return checksum, nil
}

// getPatchFileInfo 获取补丁文件信息
func (spg *StreamingPatchGenerator) getPatchFileInfo(patchPath, oldFilePath, newFilePath string) (*PatchInfo, error) {
	oldStat, err := os.Stat(oldFilePath)
	if err != nil {
		return nil, err
	}

	newStat, err := os.Stat(newFilePath)
	if err != nil {
		return nil, err
	}

	patchStat, err := os.Stat(patchPath)
	if err != nil {
		return nil, err
	}

	header, err := GetPatchInfo(patchPath)
	if err != nil {
		return nil, err
	}

	return &PatchInfo{
		PatchPath:      patchPath,
		OldFilePath:    oldFilePath,
		NewFilePath:    newFilePath,
		OldFileSize:    oldStat.Size(),
		NewFileSize:    newStat.Size(),
		PatchFileSize:  patchStat.Size(),
		OperationCount: int(header.OperationCount),
		Compression:    header.Compression,
		CreatedAt:      header.Timestamp,
		SourceChecksum: header.SourceChecksum,
		TargetChecksum: header.TargetChecksum,
	}, nil
}

// cleanup 清理资源
func (spg *StreamingPatchGenerator) cleanup() {
	if spg.writer != nil {
		spg.writer.Flush()
	}
	if spg.patchFile != nil {
		spg.patchFile.Close()
	}
	if spg.dataFile != nil {
		spg.dataFile.Close()
	}
	if spg.dataFilePath != "" {
		os.Remove(spg.dataFilePath)
	}
}
