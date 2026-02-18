//go:build windows

package performance

import (
	"fmt"
	"io"
	"os"

	hexpatch "github.com/Sky-ey/HexDiff/pkg/patch"
)

// MmapAccessor 内存映射访问接口
type MmapAccessor interface {
	ReadAt(offset int64, size int) ([]byte, error)
	Close() error
}

// OptimizedReader 优化的读取器
type OptimizedReader struct {
	file         *os.File
	optimizer    *IOOptimizer
	buffer       []byte
	filePos      int64
	fileSize     int64
	mmapData     []byte
	mmapAccessor MmapAccessor
	useMmap      bool
}

// NewOptimizedReader 创建优化的读取器 (Windows版本)
func (io *IOOptimizer) NewOptimizedReader(filePath string) (*OptimizedReader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	reader := &OptimizedReader{
		file:      file,
		optimizer: io,
		fileSize:  stat.Size(),
		buffer:    io.bufferPool.Get(),
	}

	// 尝试使用内存映射
	if io.config.EnableMmap && stat.Size() > 0 {
		if mmapFile, err := hexpatch.NewMappedFile(filePath, true); err == nil {
			reader.mmapAccessor = mmapFile
			reader.useMmap = true
		}
	}

	return reader, nil
}

// Read 读取数据 (Windows版本)
func (r *OptimizedReader) Read(p []byte) (int, error) {
	if r.useMmap && r.mmapAccessor != nil {
		return r.readFromMmap(p)
	}
	return r.readFromFile(p)
}

// readFromMmap 从内存映射读取 (Windows版本)
func (r *OptimizedReader) readFromMmap(p []byte) (int, error) {
	if r.filePos >= r.fileSize {
		return 0, io.EOF
	}

	remaining := r.fileSize - r.filePos
	toRead := int64(len(p))
	if toRead > remaining {
		toRead = remaining
	}

	data, err := r.mmapAccessor.ReadAt(r.filePos, int(toRead))
	if err != nil {
		return 0, err
	}

	copy(p, data)
	r.filePos += toRead

	return int(toRead), nil
}

// readFromFile 从文件读取 (Windows版本)
func (r *OptimizedReader) readFromFile(p []byte) (int, error) {
	n, err := r.file.Read(p)
	r.filePos += int64(n)
	return n, err
}

// Seek 跳转到指定位置 (Windows版本)
func (r *OptimizedReader) Seek(offset int64) (int64, error) {
	r.filePos = offset
	return r.filePos, nil
}

// Close 关闭读取器 (Windows版本)
func (r *OptimizedReader) Close() error {
	var err error

	// 清理内存映射
	if r.mmapAccessor != nil {
		if unmapErr := r.mmapAccessor.Close(); unmapErr != nil {
			err = unmapErr
		}
	}

	// 归还缓冲区
	if r.buffer != nil {
		r.optimizer.bufferPool.Put(r.buffer)
	}

	// 关闭文件
	if closeErr := r.file.Close(); closeErr != nil && err == nil {
		err = closeErr
	}

	return err
}
