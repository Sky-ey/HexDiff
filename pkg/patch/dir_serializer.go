package patch

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	hexdiff "github.com/Sky-ey/HexDiff/pkg/diff"
)

type DirPatchSerializer struct {
	compression CompressionType
}

func NewDirPatchSerializer(compression CompressionType) *DirPatchSerializer {
	return &DirPatchSerializer{
		compression: compression,
	}
}

func (s *DirPatchSerializer) SerializeDirPatch(result *hexdiff.DirDiffResult, oldDir, newDir, outputPath string) error {
	dirPatch := hexdiff.NewDirPatch(oldDir, newDir)

	for _, diff := range result.AddedFiles {
		entry := &hexdiff.DirPatchFile{
			RelativePath:  diff.RelativePath,
			Status:        diff.Status,
			Mode:          diff.NewEntry.Mode,
			MTime:         diff.NewEntry.MTime.Unix(),
			Size:          diff.NewEntry.Size,
			Delta:         diff.PatchData,
			IsFullContent: true,
		}
		dirPatch.AddFile(entry)
	}

	for _, diff := range result.DeletedFiles {
		entry := &hexdiff.DirPatchFile{
			RelativePath: diff.RelativePath,
			Status:       diff.Status,
			Mode:         diff.OldEntry.Mode,
			MTime:        diff.OldEntry.MTime.Unix(),
			Size:         diff.OldEntry.Size,
		}
		dirPatch.AddFile(entry)
	}

	for _, diff := range result.ModifiedFiles {
		entry := &hexdiff.DirPatchFile{
			RelativePath:  diff.RelativePath,
			Status:        diff.Status,
			Mode:          diff.NewEntry.Mode,
			MTime:         diff.NewEntry.MTime.Unix(),
			Size:          diff.NewEntry.Size,
			IsFullContent: false,
		}

		if diff.Delta != nil {
			entry.Delta = s.serializeDelta(diff.Delta)
		}

		dirPatch.AddFile(entry)
	}

	return s.writeDirPatch(dirPatch, outputPath)
}

func (s *DirPatchSerializer) serializeDelta(delta *hexdiff.Delta) []byte {
	buf := &bytes.Buffer{}

	currentDataOffset := uint32(0)
	dataBuf := &bytes.Buffer{}

	operations := make([]PatchOperation, len(delta.Operations))
	for i, op := range delta.Operations {
		patchOp := PatchOperation{
			Type:      uint8(op.Type),
			Size:      uint32(op.Size),
			Offset:    uint64(op.Offset),
			SrcOffset: uint64(op.SrcOffset),
		}

		if op.Type == 1 {
			patchOp.DataOffset = currentDataOffset
			dataBuf.Write(op.Data)
			currentDataOffset += uint32(len(op.Data))
		}

		operations[i] = patchOp
	}

	dataOffset := uint32(HeaderSize + uint32(len(delta.Operations))*OperationSize)

	header := &PatchHeader{
		Magic:          0x48455844,
		Version:        1,
		Compression:    CompressionNone,
		SourceSize:     delta.SourceSize,
		TargetSize:     delta.TargetSize,
		TargetChecksum: delta.Checksum,
		OperationCount: uint32(len(delta.Operations)),
		DataOffset:     dataOffset,
		Timestamp:      time.Now().Unix(),
	}

	buf.Write(header.Marshal())

	for _, op := range operations {
		buf.Write(op.Marshal())
	}

	buf.Write(dataBuf.Bytes())

	return buf.Bytes()
}

func (s *DirPatchSerializer) writeDirPatch(dirPatch *hexdiff.DirPatch, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create patch file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	oldDirName := dirPatch.OldDir
	newDirName := dirPatch.NewDir

	header := DirPatchHeader{
		Magic:         DirPatchMagic,
		Version:       DirPatchVersion,
		Timestamp:     dirPatch.Timestamp,
		OldDirNameLen: uint32(len(oldDirName)),
		NewDirNameLen: uint32(len(newDirName)),
		FileCount:     uint32(dirPatch.GetFileCount()),
	}

	metadataJSON, _ := json.Marshal(dirPatch.Metadata)
	header.MetadataLen = uint32(len(metadataJSON))

	writer.Write(header.Marshal())
	writer.WriteString(oldDirName)
	writer.WriteString(newDirName)

	if len(metadataJSON) > 0 {
		writer.Write(metadataJSON)
	}

	for _, filePatch := range dirPatch.Files {
		entry := DirPatchEntry{
			PathLen:       uint32(len(filePatch.RelativePath)),
			Status:        uint8(filePatch.Status),
			Mode:          uint32(filePatch.Mode),
			MTime:         filePatch.MTime,
			Size:          filePatch.Size,
			DataLen:       uint32(len(filePatch.Delta)),
			IsFullContent: boolToUint8(filePatch.IsFullContent),
		}
		copy(entry.Checksum[:], filePatch.Checksum[:])

		writer.Write(entry.Marshal())
		writer.WriteString(filePatch.RelativePath)

		if len(filePatch.Delta) > 0 {
			writer.Write(filePatch.Delta)
		}
	}

	return nil
}

func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

type DirPatchFileReader struct {
	RelativePath  string
	Status        hexdiff.FileStatus
	Mode          uint32
	MTime         int64
	Size          int64
	Checksum      [32]byte
	Delta         []byte
	IsFullContent bool
}

func (s *DirPatchSerializer) DeserializeDirPatch(inputPath string) (*hexdiff.DirPatch, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open patch file: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	headerData := make([]byte, DirPatchHeaderSize)
	if _, err := io.ReadFull(reader, headerData); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	header := &DirPatchHeader{}
	if err := header.Unmarshal(headerData); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}

	dirPatch := &hexdiff.DirPatch{
		Version:   header.Version,
		Timestamp: header.Timestamp,
	}

	oldDirName := make([]byte, header.OldDirNameLen)
	newDirName := make([]byte, header.NewDirNameLen)

	if _, err := io.ReadFull(reader, oldDirName); err != nil {
		return nil, fmt.Errorf("read old dir name: %w", err)
	}
	if _, err := io.ReadFull(reader, newDirName); err != nil {
		return nil, fmt.Errorf("read new dir name: %w", err)
	}

	dirPatch.OldDir = string(oldDirName)
	dirPatch.NewDir = string(newDirName)

	if header.MetadataLen > 0 {
		metadataJSON := make([]byte, header.MetadataLen)
		if _, err := io.ReadFull(reader, metadataJSON); err != nil {
			return nil, fmt.Errorf("read metadata: %w", err)
		}
		json.Unmarshal(metadataJSON, &dirPatch.Metadata)
	}

	dirPatch.Files = make([]*hexdiff.DirPatchFile, 0, header.FileCount)

	for i := uint32(0); i < header.FileCount; i++ {
		entryData := make([]byte, 64)
		if _, err := io.ReadFull(reader, entryData); err != nil {
			return nil, fmt.Errorf("read entry %d: %w", i, err)
		}

		entry := &DirPatchEntry{}
		if err := entry.Unmarshal(entryData); err != nil {
			return nil, fmt.Errorf("parse entry %d: %w", i, err)
		}

		pathBytes := make([]byte, entry.PathLen)
		if _, err := io.ReadFull(reader, pathBytes); err != nil {
			return nil, fmt.Errorf("read path %d: %w", i, err)
		}

		filePatch := &hexdiff.DirPatchFile{
			RelativePath:  string(pathBytes),
			Status:        hexdiff.FileStatus(entry.Status),
			Mode:          os.FileMode(entry.Mode),
			MTime:         entry.MTime,
			Size:          entry.Size,
			IsFullContent: entry.IsFullContent == 1,
		}
		copy(filePatch.Checksum[:], entry.Checksum[:])

		if entry.DataLen > 0 {
			delta := make([]byte, entry.DataLen)
			if _, err := io.ReadFull(reader, delta); err != nil {
				return nil, fmt.Errorf("read delta %d: %w", i, err)
			}
			filePatch.Delta = delta
		}

		dirPatch.Files = append(dirPatch.Files, filePatch)
	}

	return dirPatch, nil
}

func GetDirPatchInfo(patchPath string) (*DirPatchHeader, error) {
	file, err := os.Open(patchPath)
	if err != nil {
		return nil, fmt.Errorf("open patch file: %w", err)
	}
	defer file.Close()

	headerData := make([]byte, DirPatchHeaderSize)
	if _, err := io.ReadFull(file, headerData); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	header := &DirPatchHeader{}
	if err := header.Unmarshal(headerData); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}

	return header, nil
}

func IsDirPatch(patchPath string) (bool, error) {
	header, err := GetDirPatchInfo(patchPath)
	if err != nil {
		return false, err
	}
	return header.Magic == DirPatchMagic && header.Version == DirPatchVersion, nil
}
