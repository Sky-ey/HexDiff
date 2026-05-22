package diff

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateDeltaMatchesShiftedBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := filepath.Join(tmpDir, "old.bin")
	newPath := filepath.Join(tmpDir, "new.bin")

	oldData := buildPatternData(64 * 12)
	newData := make([]byte, 0, len(oldData)+1)
	newData = append(newData, 0xff)
	newData = append(newData, oldData...)

	if err := os.WriteFile(oldPath, oldData, 0644); err != nil {
		t.Fatalf("write old file: %v", err)
	}
	if err := os.WriteFile(newPath, newData, 0644); err != nil {
		t.Fatalf("write new file: %v", err)
	}

	engine, err := NewEngine(&DiffConfig{
		BlockSize:    64,
		WindowSize:   8,
		EnableCRC32:  true,
		EnableSHA256: true,
		MaxMemory:    100 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	delta, err := engine.GenerateDelta(oldPath, newPath)
	if err != nil {
		t.Fatalf("GenerateDelta() error = %v", err)
	}

	copyBytes, insertBytes := countOperationBytes(delta)
	if copyBytes != len(oldData) {
		t.Fatalf("copy bytes = %d, want %d", copyBytes, len(oldData))
	}
	if insertBytes != 1 {
		t.Fatalf("insert bytes = %d, want 1", insertBytes)
	}
	assertDeltaRebuildsTarget(t, oldData, newData, delta)
}

func TestGenerateDeltaMatchesAfterMiddleInsert(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := filepath.Join(tmpDir, "old.bin")
	newPath := filepath.Join(tmpDir, "new.bin")

	oldData := buildPatternData(64 * 16)
	insertData := []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02, 0x03}
	newData := make([]byte, 0, len(oldData)+len(insertData))
	newData = append(newData, oldData[:64*4]...)
	newData = append(newData, insertData...)
	newData = append(newData, oldData[64*4:]...)

	if err := os.WriteFile(oldPath, oldData, 0644); err != nil {
		t.Fatalf("write old file: %v", err)
	}
	if err := os.WriteFile(newPath, newData, 0644); err != nil {
		t.Fatalf("write new file: %v", err)
	}

	engine, err := NewEngine(&DiffConfig{
		BlockSize:    64,
		WindowSize:   8,
		EnableCRC32:  true,
		EnableSHA256: true,
		MaxMemory:    100 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	delta, err := engine.GenerateDelta(oldPath, newPath)
	if err != nil {
		t.Fatalf("GenerateDelta() error = %v", err)
	}

	copyBytes, insertBytes := countOperationBytes(delta)
	if copyBytes != len(oldData) {
		t.Fatalf("copy bytes = %d, want %d", copyBytes, len(oldData))
	}
	if insertBytes != len(insertData) {
		t.Fatalf("insert bytes = %d, want %d", insertBytes, len(insertData))
	}
	assertDeltaRebuildsTarget(t, oldData, newData, delta)
}

func buildPatternData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte((i*31 + i/7) % 251)
	}
	return data
}

func countOperationBytes(delta *Delta) (int, int) {
	copyBytes := 0
	insertBytes := 0
	for _, op := range delta.Operations {
		switch op.Type {
		case OpCopy:
			copyBytes += op.Size
		case OpInsert:
			insertBytes += op.Size
		}
	}
	return copyBytes, insertBytes
}

func assertDeltaRebuildsTarget(t *testing.T, oldData, newData []byte, delta *Delta) {
	t.Helper()
	rebuilt := make([]byte, delta.TargetSize)
	for _, op := range delta.Operations {
		start := int(op.Offset)
		end := start + op.Size
		switch op.Type {
		case OpCopy:
			copy(rebuilt[start:end], oldData[op.SrcOffset:op.SrcOffset+int64(op.Size)])
		case OpInsert:
			copy(rebuilt[start:end], op.Data)
		}
	}
	if !bytes.Equal(rebuilt, newData) {
		t.Fatal("delta operations did not rebuild target data")
	}
}
