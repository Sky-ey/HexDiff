package diff

import "errors"

// 差异检测相关错误
var (
	ErrInvalidBlockSize    = errors.New("invalid block size: must be between 64 and 65536 bytes")
	ErrInvalidWindowSize   = errors.New("invalid window size: must be between 8 and block size")
	ErrInvalidMaxMemory    = errors.New("invalid max memory: must be at least 1MB")
	ErrInvalidWorkerCount  = errors.New("invalid worker count: must be between 1 and 32")
	ErrFileNotFound        = errors.New("file not found")
	ErrFileReadError       = errors.New("file read error")
	ErrFileWriteError      = errors.New("file write error")
	ErrInvalidSignature    = errors.New("invalid signature format")
	ErrChecksumMismatch    = errors.New("checksum mismatch")
	ErrInvalidOperation    = errors.New("invalid operation")
	ErrMemoryLimitExceeded = errors.New("memory limit exceeded")
	ErrCorruptedData       = errors.New("corrupted data detected")
	ErrDirectoryNotFound   = errors.New("directory not found")
	ErrInvalidDirectory    = errors.New("invalid directory")
)

// DiffError 差异检测错误类型
type DiffError struct {
	Op   string // 操作名称
	Path string // 文件路径
	Err  error  // 原始错误
}

// Error 实现error接口
func (e *DiffError) Error() string {
	if e.Path != "" {
		return e.Op + " " + e.Path + ": " + e.Err.Error()
	}
	return e.Op + ": " + e.Err.Error()
}

// Unwrap 返回原始错误
func (e *DiffError) Unwrap() error {
	return e.Err
}

// NewDiffError 创建新的差异检测错误
func NewDiffError(op, path string, err error) *DiffError {
	return &DiffError{
		Op:   op,
		Path: path,
		Err:  err,
	}
}
