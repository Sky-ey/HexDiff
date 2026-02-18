package patch

import (
	"encoding/binary"
	"fmt"
)

const (
	DirPatchMagic      = 0x48455844 // "HEXD"
	DirPatchVersion    = 2          // 版本2表示目录补丁
	DirPatchHeaderSize = 64
)

type DirPatchHeader struct {
	Magic         uint32
	Version       uint16
	Reserved      uint16
	Timestamp     int64
	OldDirNameLen uint32
	NewDirNameLen uint32
	FileCount     uint32
	MetadataLen   uint32
	Reserved2     uint16
}

func (h *DirPatchHeader) Validate() error {
	if h.Magic != DirPatchMagic {
		return fmt.Errorf("invalid magic number: expected %x, got %x", DirPatchMagic, h.Magic)
	}
	if h.Version != DirPatchVersion {
		return fmt.Errorf("unsupported version: %d", h.Version)
	}
	return nil
}

func (h *DirPatchHeader) Marshal() []byte {
	buf := make([]byte, DirPatchHeaderSize)
	binary.LittleEndian.PutUint32(buf[0:4], h.Magic)
	binary.LittleEndian.PutUint16(buf[4:6], h.Version)
	binary.LittleEndian.PutUint16(buf[6:8], h.Reserved)
	binary.LittleEndian.PutUint64(buf[8:16], uint64(h.Timestamp))
	binary.LittleEndian.PutUint32(buf[16:20], h.OldDirNameLen)
	binary.LittleEndian.PutUint32(buf[20:24], h.NewDirNameLen)
	binary.LittleEndian.PutUint32(buf[24:28], h.FileCount)
	binary.LittleEndian.PutUint32(buf[28:32], h.MetadataLen)
	binary.LittleEndian.PutUint16(buf[60:62], h.Reserved2)
	return buf
}

func (h *DirPatchHeader) Unmarshal(data []byte) error {
	if len(data) < DirPatchHeaderSize {
		return fmt.Errorf("insufficient data for header")
	}
	h.Magic = binary.LittleEndian.Uint32(data[0:4])
	h.Version = binary.LittleEndian.Uint16(data[4:6])
	h.Reserved = binary.LittleEndian.Uint16(data[6:8])
	h.Timestamp = int64(binary.LittleEndian.Uint64(data[8:16]))
	h.OldDirNameLen = binary.LittleEndian.Uint32(data[16:20])
	h.NewDirNameLen = binary.LittleEndian.Uint32(data[20:24])
	h.FileCount = binary.LittleEndian.Uint32(data[24:28])
	h.MetadataLen = binary.LittleEndian.Uint32(data[28:32])
	h.Reserved2 = binary.LittleEndian.Uint16(data[60:62])
	return h.Validate()
}

type DirPatchEntry struct {
	PathLen       uint32
	Status        uint8
	Mode          uint32
	MTime         int64
	Size          int64
	Checksum      [32]byte
	DataLen       uint32
	IsFullContent uint8
	Reserved      uint16
}

func (e *DirPatchEntry) Marshal() []byte {
	buf := make([]byte, 64)
	binary.LittleEndian.PutUint32(buf[0:4], e.PathLen)
	buf[4] = e.Status
	binary.LittleEndian.PutUint32(buf[5:9], e.Mode)
	binary.LittleEndian.PutUint64(buf[9:17], uint64(e.MTime))
	binary.LittleEndian.PutUint64(buf[17:25], uint64(e.Size))
	copy(buf[25:57], e.Checksum[:])
	binary.LittleEndian.PutUint32(buf[57:61], e.DataLen)
	buf[61] = e.IsFullContent
	binary.LittleEndian.PutUint16(buf[62:64], e.Reserved)
	return buf
}

func (e *DirPatchEntry) Unmarshal(data []byte) error {
	if len(data) < 64 {
		return fmt.Errorf("insufficient data for entry")
	}
	e.PathLen = binary.LittleEndian.Uint32(data[0:4])
	e.Status = data[4]
	e.Mode = binary.LittleEndian.Uint32(data[5:9])
	e.MTime = int64(binary.LittleEndian.Uint64(data[9:17]))
	e.Size = int64(binary.LittleEndian.Uint64(data[17:25]))
	copy(e.Checksum[:], data[25:57])
	e.DataLen = binary.LittleEndian.Uint32(data[57:61])
	e.IsFullContent = data[61]
	e.Reserved = binary.LittleEndian.Uint16(data[62:64])
	return nil
}
