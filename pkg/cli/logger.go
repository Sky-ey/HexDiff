package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// Logger 日志器
type Logger struct {
	level      LogLevel
	output     io.Writer
	file       *os.File
	prefix     string
	timeFormat string
	colors     bool
}

// NewLogger 创建新的日志器
func NewLogger(levelStr, filename string) *Logger {
	logger := &Logger{
		level:      parseLogLevel(levelStr),
		output:     os.Stdout,
		prefix:     "[HexDiff]",
		timeFormat: "2006-01-02 15:04:05",
		colors:     isTerminal(),
	}

	// 如果指定了日志文件，创建文件输出
	if filename != "" {
		if err := logger.setOutputFile(filename); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 无法创建日志文件 %s: %v\n", filename, err)
		} else {
			logger.colors = false // 文件输出不使用颜色
		}
	}

	return logger
}

// setOutputFile 设置输出文件
func (l *Logger) setOutputFile(filename string) error {
	// 确保目录存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 打开文件
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// 关闭之前的文件
	if l.file != nil {
		l.file.Close()
	}

	l.file = file
	l.output = file
	return nil
}

// Close 关闭日志器
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Debug 输出调试信息
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LogLevelDebug, "DEBUG", format, args...)
}

// Info 输出信息
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LogLevelInfo, "INFO", format, args...)
}

// Warn 输出警告
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LogLevelWarn, "WARN", format, args...)
}

// Warning 输出警告（别名）
func (l *Logger) Warning(format string, args ...interface{}) {
	l.Warn(format, args...)
}

// Error 输出错误
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LogLevelError, "ERROR", format, args...)
}

// Success 输出成功信息
func (l *Logger) Success(format string, args ...interface{}) {
	if l.colors {
		l.logWithColor(LogLevelInfo, "SUCCESS", "\033[32m", format, args...)
	} else {
		l.log(LogLevelInfo, "SUCCESS", format, args...)
	}
}

// Fatal 输出致命错误并退出
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.Error(format, args...)
	os.Exit(1)
}

// log 内部日志方法
func (l *Logger) log(level LogLevel, levelStr, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format(l.timeFormat)
	message := fmt.Sprintf(format, args...)
	
	var output string
	if l.colors {
		color := getColorCode(level)
		output = fmt.Sprintf("%s %s %s%s\033[0m %s\n", 
			timestamp, l.prefix, color, levelStr, message)
	} else {
		output = fmt.Sprintf("%s %s [%s] %s\n", 
			timestamp, l.prefix, levelStr, message)
	}

	fmt.Fprint(l.output, output)
}

// logWithColor 带颜色的日志输出
func (l *Logger) logWithColor(level LogLevel, levelStr, color, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format(l.timeFormat)
	message := fmt.Sprintf(format, args...)
	
	var output string
	if l.colors {
		output = fmt.Sprintf("%s %s %s%s\033[0m %s\n", 
			timestamp, l.prefix, color, levelStr, message)
	} else {
		output = fmt.Sprintf("%s %s [%s] %s\n", 
			timestamp, l.prefix, levelStr, message)
	}

	fmt.Fprint(l.output, output)
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(levelStr string) {
	l.level = parseLogLevel(levelStr)
}

// GetLevel 获取当前日志级别
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// parseLogLevel 解析日志级别字符串
func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToLower(levelStr) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

// getColorCode 获取日志级别对应的颜色代码
func getColorCode(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "\033[36m" // 青色
	case LogLevelInfo:
		return "\033[37m" // 白色
	case LogLevelWarn:
		return "\033[33m" // 黄色
	case LogLevelError:
		return "\033[31m" // 红色
	default:
		return "\033[37m" // 白色
	}
}

// isTerminal 检查是否为终端输出
func isTerminal() bool {
	// 简单检查，实际实现可能需要更复杂的逻辑
	if os.Getenv("TERM") == "" {
		return false
	}
	
	// 检查stdout是否为终端
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// MultiLogger 多输出日志器
type MultiLogger struct {
	loggers []*Logger
}

// NewMultiLogger 创建多输出日志器
func NewMultiLogger(loggers ...*Logger) *MultiLogger {
	return &MultiLogger{
		loggers: loggers,
	}
}

// Debug 输出调试信息到所有日志器
func (ml *MultiLogger) Debug(format string, args ...interface{}) {
	for _, logger := range ml.loggers {
		logger.Debug(format, args...)
	}
}

// Info 输出信息到所有日志器
func (ml *MultiLogger) Info(format string, args ...interface{}) {
	for _, logger := range ml.loggers {
		logger.Info(format, args...)
	}
}

// Warn 输出警告到所有日志器
func (ml *MultiLogger) Warn(format string, args ...interface{}) {
	for _, logger := range ml.loggers {
		logger.Warn(format, args...)
	}
}

// Warning 输出警告到所有日志器（别名）
func (ml *MultiLogger) Warning(format string, args ...interface{}) {
	ml.Warn(format, args...)
}

// Error 输出错误到所有日志器
func (ml *MultiLogger) Error(format string, args ...interface{}) {
	for _, logger := range ml.loggers {
		logger.Error(format, args...)
	}
}

// Success 输出成功信息到所有日志器
func (ml *MultiLogger) Success(format string, args ...interface{}) {
	for _, logger := range ml.loggers {
		logger.Success(format, args...)
	}
}

// Fatal 输出致命错误到所有日志器并退出
func (ml *MultiLogger) Fatal(format string, args ...interface{}) {
	for _, logger := range ml.loggers {
		logger.Error(format, args...)
	}
	os.Exit(1)
}

// Close 关闭所有日志器
func (ml *MultiLogger) Close() error {
	var lastErr error
	for _, logger := range ml.loggers {
		if err := logger.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}