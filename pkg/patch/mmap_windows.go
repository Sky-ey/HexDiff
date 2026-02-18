//go:build windows

package patch

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// MappedFile 内存映射文件
type MappedFile struct {
	file     *os.File
	data     []byte
	size     int64
	mapped   bool
	filePath string
}

// NewMappedFile 创建内存映射文件
func NewMappedFile(filePath string, readOnly bool) (*MappedFile, error) {
	var flag int

	if readOnly {
		flag = os.O_RDONLY
	} else {
		flag = os.O_RDWR
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

	data, err := mmapFile(file, int(size), readOnly)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("mmap file: %w", err)
	}

	return &MappedFile{
		file:     file,
		data:     data,
		size:     size,
		mapped:   true,
		filePath: filePath,
	}, nil
}

// mmapFile 跨平台内存映射
func mmapFile(file *os.File, size int, readOnly bool) ([]byte, error) {
	if readOnly {
		prot := uint32(windows.PAGE_READONLY)
		access := uint32(windows.FILE_MAP_READ)
		return windowsMapFile(file, prot, access, size)
	}
	prot := uint32(windows.PAGE_READWRITE)
	access := uint32(windows.FILE_MAP_WRITE)
	return windowsMapFile(file, prot, access, size)
}

// windowsMapFile Windows内存映射实现
func windowsMapFile(file *os.File, prot uint32, access uint32, size int) ([]byte, error) {
	handle, err := windows.CreateFileMapping(
		windows.Handle(file.Fd()),
		nil,
		prot,
		0,
		0,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateFileMapping: %w", err)
	}
	defer windows.CloseHandle(handle)

	addr, err := windows.MapViewOfFile(
		handle,
		access,
		0,
		0,
		uintptr(size),
	)
	if err != nil {
		return nil, fmt.Errorf("MapViewOfFile: %w", err)
	}

	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)
	return data, nil
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

	if mf.filePath != "" {
		return mf.file.Sync()
	}

	return nil
}

// Close 关闭内存映射文件
func (mf *MappedFile) Close() error {
	var err error

	if mf.mapped && mf.data != nil {
		if err = windows.UnmapViewOfFile(uintptr(unsafe.Pointer(&mf.data[0]))); err != nil {
			err = fmt.Errorf("UnmapViewOfFile: %w", err)
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
	return nil
}

// AdviseRandom 建议操作系统进行随机访问优化
func (mf *MappedFile) AdviseRandom() error {
	if !mf.mapped {
		return nil
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

// Seek 跳转到指定位置
func (sr *StreamReader) Seek(offset int64) error {
	if offset < 0 || offset > sr.fileSize {
		return fmt.Errorf("seek offset out of range: %d", offset)
	}

	_, err := sr.file.Seek(offset, 0)
	if err != nil {
		return err
	}

	sr.offset = offset
	return nil
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
