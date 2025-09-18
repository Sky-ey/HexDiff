package cli

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ErrorCode 错误代码
type ErrorCode int

const (
	// 通用错误
	ErrUnknown ErrorCode = iota
	ErrInvalidArgument
	ErrFileNotFound
	ErrPermissionDenied
	ErrInsufficientSpace
	ErrTimeout

	// 文件操作错误
	ErrFileRead
	ErrFileWrite
	ErrFileCreate
	ErrFileDelete
	ErrFileCopy
	ErrFileMove

	// 补丁相关错误
	ErrPatchGeneration
	ErrPatchApplication
	ErrPatchValidation
	ErrPatchCorrupted
	ErrPatchIncompatible

	// 完整性错误
	ErrChecksumMismatch
	ErrIntegrityCheck
	ErrBackupFailed
	ErrRecoveryFailed

	// 性能相关错误
	ErrMemoryExhausted
	ErrConcurrencyLimit
	ErrCacheError
	ErrIOError

	// 配置错误
	ErrConfigInvalid
	ErrConfigNotFound
	ErrConfigPermission
)

// CLIError CLI错误类型
type CLIError struct {
	Code      ErrorCode
	Message   string
	Cause     error
	Context   map[string]interface{}
	Stack     []string
	Timestamp string
}

// NewCLIError 创建新的CLI错误
func NewCLIError(code ErrorCode, message string) *CLIError {
	return &CLIError{
		Code:      code,
		Message:   message,
		Context:   make(map[string]interface{}),
		Stack:     captureStack(),
		Timestamp: getCurrentTimestamp(),
	}
}

// NewCLIErrorWithCause 创建带原因的CLI错误
func NewCLIErrorWithCause(code ErrorCode, message string, cause error) *CLIError {
	return &CLIError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Context:   make(map[string]interface{}),
		Stack:     captureStack(),
		Timestamp: getCurrentTimestamp(),
	}
}

// Error 实现error接口
func (e *CLIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// WithContext 添加上下文信息
func (e *CLIError) WithContext(key string, value interface{}) *CLIError {
	e.Context[key] = value
	return e
}

// GetCode 获取错误代码
func (e *CLIError) GetCode() ErrorCode {
	return e.Code
}

// GetMessage 获取错误消息
func (e *CLIError) GetMessage() string {
	return e.Message
}

// GetCause 获取原始错误
func (e *CLIError) GetCause() error {
	return e.Cause
}

// GetContext 获取上下文信息
func (e *CLIError) GetContext() map[string]interface{} {
	return e.Context
}

// GetStack 获取调用栈
func (e *CLIError) GetStack() []string {
	return e.Stack
}

// String 返回详细的错误信息
func (e *CLIError) String() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("错误代码: %s\n", e.Code.String()))
	builder.WriteString(fmt.Sprintf("错误消息: %s\n", e.Message))
	builder.WriteString(fmt.Sprintf("发生时间: %s\n", e.Timestamp))

	if e.Cause != nil {
		builder.WriteString(fmt.Sprintf("原始错误: %v\n", e.Cause))
	}

	if len(e.Context) > 0 {
		builder.WriteString("上下文信息:\n")
		for key, value := range e.Context {
			builder.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
		}
	}

	if len(e.Stack) > 0 {
		builder.WriteString("调用栈:\n")
		for _, frame := range e.Stack {
			builder.WriteString(fmt.Sprintf("  %s\n", frame))
		}
	}

	return builder.String()
}

// String 返回错误代码的字符串表示
func (code ErrorCode) String() string {
	switch code {
	case ErrUnknown:
		return "UNKNOWN"
	case ErrInvalidArgument:
		return "INVALID_ARGUMENT"
	case ErrFileNotFound:
		return "FILE_NOT_FOUND"
	case ErrPermissionDenied:
		return "PERMISSION_DENIED"
	case ErrInsufficientSpace:
		return "INSUFFICIENT_SPACE"
	case ErrTimeout:
		return "TIMEOUT"
	case ErrFileRead:
		return "FILE_READ"
	case ErrFileWrite:
		return "FILE_WRITE"
	case ErrFileCreate:
		return "FILE_CREATE"
	case ErrFileDelete:
		return "FILE_DELETE"
	case ErrFileCopy:
		return "FILE_COPY"
	case ErrFileMove:
		return "FILE_MOVE"
	case ErrPatchGeneration:
		return "PATCH_GENERATION"
	case ErrPatchApplication:
		return "PATCH_APPLICATION"
	case ErrPatchValidation:
		return "PATCH_VALIDATION"
	case ErrPatchCorrupted:
		return "PATCH_CORRUPTED"
	case ErrPatchIncompatible:
		return "PATCH_INCOMPATIBLE"
	case ErrChecksumMismatch:
		return "CHECKSUM_MISMATCH"
	case ErrIntegrityCheck:
		return "INTEGRITY_CHECK"
	case ErrBackupFailed:
		return "BACKUP_FAILED"
	case ErrRecoveryFailed:
		return "RECOVERY_FAILED"
	case ErrMemoryExhausted:
		return "MEMORY_EXHAUSTED"
	case ErrConcurrencyLimit:
		return "CONCURRENCY_LIMIT"
	case ErrCacheError:
		return "CACHE_ERROR"
	case ErrIOError:
		return "IO_ERROR"
	case ErrConfigInvalid:
		return "CONFIG_INVALID"
	case ErrConfigNotFound:
		return "CONFIG_NOT_FOUND"
	case ErrConfigPermission:
		return "CONFIG_PERMISSION"
	default:
		return "UNKNOWN"
	}
}

// ErrorHandler 错误处理器
type ErrorHandler struct {
	logger   *Logger
	verbose  bool
	exitCode map[ErrorCode]int
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler(logger *Logger, verbose bool) *ErrorHandler {
	handler := &ErrorHandler{
		logger:   logger,
		verbose:  verbose,
		exitCode: make(map[ErrorCode]int),
	}

	// 设置默认退出代码
	handler.setDefaultExitCodes()

	return handler
}

// setDefaultExitCodes 设置默认退出代码
func (eh *ErrorHandler) setDefaultExitCodes() {
	eh.exitCode[ErrUnknown] = 1
	eh.exitCode[ErrInvalidArgument] = 2
	eh.exitCode[ErrFileNotFound] = 3
	eh.exitCode[ErrPermissionDenied] = 4
	eh.exitCode[ErrInsufficientSpace] = 5
	eh.exitCode[ErrTimeout] = 6
	eh.exitCode[ErrFileRead] = 10
	eh.exitCode[ErrFileWrite] = 11
	eh.exitCode[ErrFileCreate] = 12
	eh.exitCode[ErrFileDelete] = 13
	eh.exitCode[ErrFileCopy] = 14
	eh.exitCode[ErrFileMove] = 15
	eh.exitCode[ErrPatchGeneration] = 20
	eh.exitCode[ErrPatchApplication] = 21
	eh.exitCode[ErrPatchValidation] = 22
	eh.exitCode[ErrPatchCorrupted] = 23
	eh.exitCode[ErrPatchIncompatible] = 24
	eh.exitCode[ErrChecksumMismatch] = 30
	eh.exitCode[ErrIntegrityCheck] = 31
	eh.exitCode[ErrBackupFailed] = 32
	eh.exitCode[ErrRecoveryFailed] = 33
	eh.exitCode[ErrMemoryExhausted] = 40
	eh.exitCode[ErrConcurrencyLimit] = 41
	eh.exitCode[ErrCacheError] = 42
	eh.exitCode[ErrIOError] = 43
	eh.exitCode[ErrConfigInvalid] = 50
	eh.exitCode[ErrConfigNotFound] = 51
	eh.exitCode[ErrConfigPermission] = 52
}

// Handle 处理错误
func (eh *ErrorHandler) Handle(err error) int {
	if err == nil {
		return 0
	}

	// 检查是否为CLI错误
	if cliErr, ok := err.(*CLIError); ok {
		return eh.handleCLIError(cliErr)
	}

	// 处理普通错误
	eh.logger.Error("发生错误: %v", err)
	return 1
}

// handleCLIError 处理CLI错误
func (eh *ErrorHandler) handleCLIError(err *CLIError) int {
	// 输出用户友好的错误消息
	eh.logger.Error("%s", err.GetMessage())

	// 如果启用详细模式，输出更多信息
	if eh.verbose {
		if err.GetCause() != nil {
			eh.logger.Debug("原始错误: %v", err.GetCause())
		}

		if len(err.GetContext()) > 0 {
			eh.logger.Debug("上下文信息:")
			for key, value := range err.GetContext() {
				eh.logger.Debug("  %s: %v", key, value)
			}
		}

		if len(err.GetStack()) > 0 {
			eh.logger.Debug("调用栈:")
			for _, frame := range err.GetStack() {
				eh.logger.Debug("  %s", frame)
			}
		}
	}

	// 返回对应的退出代码
	if code, exists := eh.exitCode[err.GetCode()]; exists {
		return code
	}

	return 1
}

// SetExitCode 设置错误代码对应的退出代码
func (eh *ErrorHandler) SetExitCode(errCode ErrorCode, exitCode int) {
	eh.exitCode[errCode] = exitCode
}

// GetExitCode 获取错误代码对应的退出代码
func (eh *ErrorHandler) GetExitCode(errCode ErrorCode) int {
	if code, exists := eh.exitCode[errCode]; exists {
		return code
	}
	return 1
}

// 辅助函数

// captureStack 捕获调用栈
func captureStack() []string {
	var stack []string

	// 跳过当前函数和NewCLIError函数
	for i := 2; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}

		// 只保留相关的调用栈信息
		funcName := fn.Name()
		if strings.Contains(funcName, "runtime.") {
			continue
		}

		frame := fmt.Sprintf("%s:%d %s", file, line, funcName)
		stack = append(stack, frame)

		// 限制调用栈深度
		if len(stack) >= 10 {
			break
		}
	}

	return stack
}

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() string {
	return fmt.Sprintf("%d", time.Now().Unix())
}

// 预定义的错误创建函数

// ErrInvalidArgumentf 创建无效参数错误
func ErrInvalidArgumentf(format string, args ...interface{}) *CLIError {
	return NewCLIError(ErrInvalidArgument, fmt.Sprintf(format, args...))
}

// ErrFileNotFoundf 创建文件未找到错误
func ErrFileNotFoundf(format string, args ...interface{}) *CLIError {
	return NewCLIError(ErrFileNotFound, fmt.Sprintf(format, args...))
}

// ErrPermissionDeniedf 创建权限拒绝错误
func ErrPermissionDeniedf(format string, args ...interface{}) *CLIError {
	return NewCLIError(ErrPermissionDenied, fmt.Sprintf(format, args...))
}

// ErrPatchGenerationf 创建补丁生成错误
func ErrPatchGenerationf(format string, args ...interface{}) *CLIError {
	return NewCLIError(ErrPatchGeneration, fmt.Sprintf(format, args...))
}

// ErrPatchApplicationf 创建补丁应用错误
func ErrPatchApplicationf(format string, args ...interface{}) *CLIError {
	return NewCLIError(ErrPatchApplication, fmt.Sprintf(format, args...))
}

// ErrChecksumMismatchf 创建校验和不匹配错误
func ErrChecksumMismatchf(format string, args ...interface{}) *CLIError {
	return NewCLIError(ErrChecksumMismatch, fmt.Sprintf(format, args...))
}

// WrapError 包装普通错误为CLI错误
func WrapError(code ErrorCode, message string, cause error) *CLIError {
	return NewCLIErrorWithCause(code, message, cause)
}
