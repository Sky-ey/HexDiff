package patch

import (
	"encoding/binary"
	"fmt"
	"time"
)

// 补丁文件格式常量
const (
	// MagicNumber 补丁文件魔数
	MagicNumber = 0x48455844 // "HEXD"
	// Version 补丁文件版本
	Version = 1
	// HeaderSize 文件头大小 (4+2+1+1+8+8+8+32+32+4+4 = 104字节)
	HeaderSize = 104
)

// CompressionType 压缩类型
type CompressionType uint8

const (
	CompressionNone CompressionType = iota // 无压缩
	CompressionGzip                        // Gzip压缩
	CompressionLZ4                         // LZ4压缩
)

// String 返回压缩类型的字符串表示
func (c CompressionType) String() string {
	switch c {
	case CompressionNone:
		return "None"
	case CompressionGzip:
		return "Gzip"
	case CompressionLZ4:
		return "LZ4"
	default:
		return "Unknown"
	}
}

// PatchHeader 补丁文件头
type PatchHeader struct {
	Magic          uint32          // 魔数 "HEXD"
	Version        uint16          // 版本号
	Compression    CompressionType // 压缩类型
	Reserved       uint8           // 保留字段
	Timestamp      int64           // 创建时间戳
	SourceSize     int64           // 源文件大小
	TargetSize     int64           // 目标文件大小
	SourceChecksum [32]byte        // 源文件SHA-256校验和
	TargetChecksum [32]byte        // 目标文件SHA-256校验和
	OperationCount uint32          // 操作数量
	DataOffset     uint32          // 数据区偏移量
}

// NewPatchHeader 创建新的补丁文件头
func NewPatchHeader() *PatchHeader {
	return &PatchHeader{
		Magic:       MagicNumber,
		Version:     Version,
		Compression: CompressionNone,
		Timestamp:   time.Now().Unix(),
	}
}

// Validate 验证补丁文件头
func (h *PatchHeader) Validate() error {
	if h.Magic != MagicNumber {
		return fmt.Errorf("invalid magic number: expected %x, got %x", MagicNumber, h.Magic)
	}
	if h.Version != Version {
		return fmt.Errorf("unsupported version: %d", h.Version)
	}
	if h.SourceSize < 0 || h.TargetSize < 0 {
		return fmt.Errorf("invalid file size: source=%d, target=%d", h.SourceSize, h.TargetSize)
	}
	return nil
}

// Marshal 序列化补丁文件头
func (h *PatchHeader) Marshal() []byte {
	buf := make([]byte, HeaderSize)

	binary.LittleEndian.PutUint32(buf[0:4], h.Magic)
	binary.LittleEndian.PutUint16(buf[4:6], h.Version)
	buf[6] = uint8(h.Compression)
	buf[7] = h.Reserved
	binary.LittleEndian.PutUint64(buf[8:16], uint64(h.Timestamp))
	binary.LittleEndian.PutUint64(buf[16:24], uint64(h.SourceSize))
	binary.LittleEndian.PutUint64(buf[24:32], uint64(h.TargetSize))
	copy(buf[32:64], h.SourceChecksum[:])
	copy(buf[64:96], h.TargetChecksum[:])
	binary.LittleEndian.PutUint32(buf[96:100], h.OperationCount)
	binary.LittleEndian.PutUint32(buf[100:104], h.DataOffset)

	return buf
}

// Unmarshal 反序列化补丁文件头
func (h *PatchHeader) Unmarshal(data []byte) error {
	if len(data) < HeaderSize {
		return fmt.Errorf("insufficient data for header: need %d bytes, got %d", HeaderSize, len(data))
	}

	h.Magic = binary.LittleEndian.Uint32(data[0:4])
	h.Version = binary.LittleEndian.Uint16(data[4:6])
	h.Compression = CompressionType(data[6])
	h.Reserved = data[7]
	h.Timestamp = int64(binary.LittleEndian.Uint64(data[8:16]))
	h.SourceSize = int64(binary.LittleEndian.Uint64(data[16:24]))
	h.TargetSize = int64(binary.LittleEndian.Uint64(data[24:32]))
	copy(h.SourceChecksum[:], data[32:64])
	copy(h.TargetChecksum[:], data[64:96])
	h.OperationCount = binary.LittleEndian.Uint32(data[96:100])
	h.DataOffset = binary.LittleEndian.Uint32(data[100:104])

	return h.Validate()
}

// PatchOperation 补丁操作（序列化格式）
type PatchOperation struct {
	Type       uint8  // 操作类型 (0=Copy, 1=Insert, 2=Delete)
	Reserved   uint8  // 保留字段
	Size       uint32 // 数据大小
	Offset     uint64 // 目标偏移量
	SrcOffset  uint64 // 源偏移量（仅Copy操作使用）
	DataOffset uint32 // 数据在补丁文件中的偏移量（仅Insert操作使用）
}

// OperationSize 单个操作的序列化大小
const OperationSize = 26

// Marshal 序列化补丁操作
func (op *PatchOperation) Marshal() []byte {
	buf := make([]byte, OperationSize)

	buf[0] = op.Type
	buf[1] = op.Reserved
	binary.LittleEndian.PutUint32(buf[2:6], op.Size)
	binary.LittleEndian.PutUint64(buf[6:14], op.Offset)
	binary.LittleEndian.PutUint64(buf[14:22], op.SrcOffset)
	binary.LittleEndian.PutUint32(buf[22:26], op.DataOffset)

	return buf
}

// Unmarshal 反序列化补丁操作
func (op *PatchOperation) Unmarshal(data []byte) error {
	if len(data) < OperationSize {
		return fmt.Errorf("insufficient data for operation: need %d bytes, got %d", OperationSize, len(data))
	}

	op.Type = data[0]
	op.Reserved = data[1]
	op.Size = binary.LittleEndian.Uint32(data[2:6])
	op.Offset = binary.LittleEndian.Uint64(data[6:14])
	op.SrcOffset = binary.LittleEndian.Uint64(data[14:22])
	op.DataOffset = binary.LittleEndian.Uint32(data[22:26])

	return nil
}

// PatchFile 补丁文件结构
type PatchFile struct {
	Header     *PatchHeader     // 文件头
	Operations []PatchOperation // 操作列表
	Data       []byte           // 插入数据
}

// NewPatchFile 创建新的补丁文件
func NewPatchFile() *PatchFile {
	return &PatchFile{
		Header:     NewPatchHeader(),
		Operations: make([]PatchOperation, 0),
		Data:       make([]byte, 0),
	}
}

// AddInsertData 添加插入数据
func (pf *PatchFile) AddInsertData(data []byte) uint32 {
	offset := uint32(len(pf.Data))
	pf.Data = append(pf.Data, data...)
	return offset
}

// GetInsertData 获取插入数据
func (pf *PatchFile) GetInsertData(offset, size uint32) ([]byte, error) {
	if offset+size > uint32(len(pf.Data)) {
		return nil, fmt.Errorf("data range out of bounds: offset=%d, size=%d, total=%d",
			offset, size, len(pf.Data))
	}
	return pf.Data[offset : offset+size], nil
}

// CalculateSize 计算补丁文件总大小
func (pf *PatchFile) CalculateSize() int64 {
	size := int64(HeaderSize)                                // 文件头
	size += int64(len(pf.Operations)) * int64(OperationSize) // 操作列表
	size += int64(len(pf.Data))                              // 数据区
	return size
}

// UpdateHeader 更新文件头信息
func (pf *PatchFile) UpdateHeader() {
	pf.Header.OperationCount = uint32(len(pf.Operations))
	pf.Header.DataOffset = uint32(HeaderSize + len(pf.Operations)*OperationSize)
}
