package patch

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/Sky-ey/HexDiff/pkg/diff"
)

// Serializer 补丁序列化器
type Serializer struct {
	compression CompressionType
}

// NewSerializer 创建新的序列化器
func NewSerializer(compression CompressionType) *Serializer {
	return &Serializer{
		compression: compression,
	}
}

// SerializeDelta 将差异结果序列化为补丁文件
func (s *Serializer) SerializeDelta(delta *diff.Delta, sourceChecksum [32]byte, outputPath string) error {
	// 创建补丁文件结构
	patchFile := NewPatchFile()
	patchFile.Header.Compression = s.compression
	patchFile.Header.SourceSize = delta.SourceSize
	patchFile.Header.TargetSize = delta.TargetSize
	patchFile.Header.SourceChecksum = sourceChecksum
	patchFile.Header.TargetChecksum = delta.Checksum

	// 转换操作并收集插入数据，过滤掉空操作
	for _, op := range delta.Operations {
		if op.Size == 0 {
			continue
		}

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
			patchOp.DataOffset = patchFile.AddInsertData(op.Data)
		case diff.OpDelete:
			patchOp.Type = 2
		default:
			return fmt.Errorf("unknown operation type: %v", op.Type)
		}

		patchFile.Operations = append(patchFile.Operations, patchOp)
	}

	// 更新文件头
	patchFile.UpdateHeader()

	// 写入文件
	return s.writePatchFile(patchFile, outputPath)
}

// writePatchFile 写入补丁文件
func (s *Serializer) writePatchFile(patchFile *PatchFile, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create patch file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// 写入文件头
	headerData := patchFile.Header.Marshal()
	if _, err := writer.Write(headerData); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// 写入操作列表
	for _, op := range patchFile.Operations {
		opData := op.Marshal()
		if _, err := writer.Write(opData); err != nil {
			return fmt.Errorf("write operation: %w", err)
		}
	}

	// 写入数据区（可能压缩）
	if err := s.writeData(writer, patchFile.Data); err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	return nil
}

// writeData 写入数据（支持压缩）
func (s *Serializer) writeData(writer io.Writer, data []byte) error {
	switch s.compression {
	case CompressionNone:
		_, err := writer.Write(data)
		return err
	case CompressionGzip:
		gzipWriter := gzip.NewWriter(writer)
		defer gzipWriter.Close()
		_, err := gzipWriter.Write(data)
		return err
	default:
		return fmt.Errorf("unsupported compression type: %v", s.compression)
	}
}

// DeserializePatch 反序列化补丁文件
func (s *Serializer) DeserializePatch(inputPath string) (*PatchFile, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open patch file: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	// 读取文件头
	headerData := make([]byte, HeaderSize)
	if _, err := io.ReadFull(reader, headerData); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	header := &PatchHeader{}
	if err := header.Unmarshal(headerData); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}

	patchFile := &PatchFile{
		Header:     header,
		Operations: make([]PatchOperation, header.OperationCount),
	}

	// 读取操作列表
	for i := uint32(0); i < header.OperationCount; i++ {
		opData := make([]byte, OperationSize)
		if _, err := io.ReadFull(reader, opData); err != nil {
			return nil, fmt.Errorf("read operation %d: %w", i, err)
		}

		if err := patchFile.Operations[i].Unmarshal(opData); err != nil {
			return nil, fmt.Errorf("parse operation %d: %w", i, err)
		}
	}

	// 读取剩余的数据区
	remainingData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read remaining data: %w", err)
	}

	if len(remainingData) > 0 {
		// 解压数据
		patchFile.Data, err = s.decompressData(remainingData, header.Compression)
		if err != nil {
			return nil, fmt.Errorf("decompress data: %w", err)
		}
	}

	return patchFile, nil
}

func (s *Serializer) DeserializeFromData(data []byte) (*PatchFile, error) {
	reader := bytes.NewReader(data)

	headerData := make([]byte, HeaderSize)
	if _, err := io.ReadFull(reader, headerData); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	header := &PatchHeader{}
	if err := header.Unmarshal(headerData); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}

	patchFile := &PatchFile{
		Header:     header,
		Operations: make([]PatchOperation, header.OperationCount),
	}

	for i := uint32(0); i < header.OperationCount; i++ {
		opData := make([]byte, OperationSize)
		if _, err := io.ReadFull(reader, opData); err != nil {
			return nil, fmt.Errorf("read operation %d: %w", i, err)
		}

		if err := patchFile.Operations[i].Unmarshal(opData); err != nil {
			return nil, fmt.Errorf("parse operation %d: %w", i, err)
		}
	}

	remainingData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read remaining data: %w", err)
	}

	if len(remainingData) > 0 {
		patchFile.Data, err = s.decompressData(remainingData, header.Compression)
		if err != nil {
			return nil, fmt.Errorf("decompress data: %w", err)
		}
	}

	return patchFile, nil
}

// decompressData 解压数据
func (s *Serializer) decompressData(compressedData []byte, compression CompressionType) ([]byte, error) {
	switch compression {
	case CompressionNone:
		return compressedData, nil
	case CompressionGzip:
		reader, err := gzip.NewReader(bytes.NewReader(compressedData))
		if err != nil {
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer reader.Close()

		return io.ReadAll(reader)
	default:
		return nil, fmt.Errorf("unsupported compression type: %v", compression)
	}
}

// GetPatchInfo 获取补丁文件信息
func GetPatchInfo(patchPath string) (*PatchHeader, error) {
	file, err := os.Open(patchPath)
	if err != nil {
		return nil, fmt.Errorf("open patch file: %w", err)
	}
	defer file.Close()

	headerData := make([]byte, HeaderSize)
	if _, err := io.ReadFull(file, headerData); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	header := &PatchHeader{}
	if err := header.Unmarshal(headerData); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}

	return header, nil
}
