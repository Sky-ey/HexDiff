//go:build !windows

package patch

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// MappedFile 内存映射文件
type MappedFile struct {
	file   *os.File
	data   []byte
	size   int64
	mapped bool
}

// NewMappedFile 创建内存映射文件
func NewMappedFile(filePath string, readOnly bool) (*MappedFile, error) {
	var flag int
	var prot int

	if readOnly {
		flag = os.O_RDONLY
		prot = syscall.PROT_READ
	} else {
		flag = os.O_RDWR
		prot = syscall.PROT_READ | syscall.PROT_WRITE
	}

	file, err := os.OpenFile(filePath, flag, 0)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("stat file: %w", err)
	}

	size := fileInfo.Size()
	if size == 0 {
		return &MappedFile{
			file:   file,
			data:   nil,
			size:   0,
			mapped: false,
		}, nil
	}

	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), prot, syscall.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("mmap file: %w", err)
	}

	return &MappedFile{
		file:   file,
		data:   data,
		size:   size,
		mapped: true,
	}, nil
}

// Data 获取映射的数据
func (mf *MappedFile) Data() []byte {
	return mf.data
}

// Size 获取文件大小
func (mf *MappedFile) Size() int64 {
	return mf.size
}

// ReadAt 从指定偏移量读取数据
func (mf *MappedFile) ReadAt(offset int64, size int) ([]byte, error) {
	if offset < 0 || offset >= mf.size {
		return nil, fmt.Errorf("offset out of range: %d", offset)
	}

	end := offset + int64(size)
	if end > mf.size {
		end = mf.size
	}

	if !mf.mapped {
		data := make([]byte, end-offset)
		n, err := mf.file.ReadAt(data, offset)
		if err != nil {
			return nil, err
		}
		return data[:n], nil
	}

	return mf.data[offset:end], nil
}

// WriteAt 向指定偏移量写入数据
func (mf *MappedFile) WriteAt(data []byte, offset int64) error {
	if !mf.mapped {
		_, err := mf.file.WriteAt(data, offset)
		return err
	}

	if offset < 0 || offset+int64(len(data)) > mf.size {
		return fmt.Errorf("write range out of bounds")
	}

	copy(mf.data[offset:], data)
	return nil
}

// Sync 同步内存映射到磁盘
func (mf *MappedFile) Sync() error {
	if !mf.mapped {
		return mf.file.Sync()
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_MSYNC,
		uintptr(unsafe.Pointer(&mf.data[0])),
		uintptr(len(mf.data)),
		uintptr(0x01),
	)

	if errno != 0 {
		return errno
	}

	return nil
}

// Close 关闭内存映射文件
func (mf *MappedFile) Close() error {
	var err error

	if mf.mapped && mf.data != nil {
		if unmapErr := syscall.Munmap(mf.data); unmapErr != nil {
			err = fmt.Errorf("munmap: %w", unmapErr)
		}
		mf.data = nil
		mf.mapped = false
	}

	if mf.file != nil {
		if closeErr := mf.file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close file: %w", closeErr)
		}
		mf.file = nil
	}

	return err
}

// AdviseSequential 建议操作系统进行顺序访问优化
func (mf *MappedFile) AdviseSequential() error {
	if !mf.mapped {
		return nil
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_MADVISE,
		uintptr(unsafe.Pointer(&mf.data[0])),
		uintptr(len(mf.data)),
		uintptr(2), // MADV_SEQUENTIAL
	)

	if errno != 0 {
		return errno
	}

	return nil
}

// AdviseRandom 建议操作系统进行随机访问优化
func (mf *MappedFile) AdviseRandom() error {
	if !mf.mapped {
		return nil
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_MADVISE,
		uintptr(unsafe.Pointer(&mf.data[0])),
		uintptr(len(mf.data)),
		uintptr(1), // MADV_RANDOM
	)

	if errno != 0 {
		return errno
	}

	return nil
}

// StreamReader 流式读取器，用于大文件处理
type StreamReader struct {
	file       *os.File
	bufferSize int
	buffer     []byte
	offset     int64
	fileSize   int64
}

// NewStreamReader 创建流式读取器
func NewStreamReader(filePath string, bufferSize int) (*StreamReader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("stat file: %w", err)
	}

	if bufferSize <= 0 {
		bufferSize = 64 * 1024
	}

	return &StreamReader{
		file:       file,
		bufferSize: bufferSize,
		buffer:     make([]byte, bufferSize),
		offset:     0,
		fileSize:   fileInfo.Size(),
	}, nil
}

// Read 读取下一块数据
func (sr *StreamReader) Read() ([]byte, int64, error) {
	if sr.offset >= sr.fileSize {
		return nil, sr.offset, fmt.Errorf("EOF")
	}

	n, err := sr.file.Read(sr.buffer)
	if err != nil && n == 0 {
		return nil, sr.offset, err
	}

	data := make([]byte, n)
	copy(data, sr.buffer[:n])

	currentOffset := sr.offset
	sr.offset += int64(n)

	return data, currentOffset, nil
}

func (sr *StreamReader) Seek(offset int64, whence int) (int64, error) {
	if offset < 0 || offset > sr.fileSize {
		return 0, fmt.Errorf("seek offset out of range: %d", offset)
	}

	newOffset, err := sr.file.Seek(offset, whence)
	if err != nil {
		return 0, err
	}

	sr.offset = newOffset

	return newOffset, nil
}

// Close 关闭流式读取器
func (sr *StreamReader) Close() error {
	if sr.file != nil {
		return sr.file.Close()
	}
	return nil
}

// Size 获取文件大小
func (sr *StreamReader) Size() int64 {
	return sr.fileSize
}

// Offset 获取当前偏移量
func (sr *StreamReader) Offset() int64 {
	return sr.offset
}
