// Package HexDiff provides a simple and powerful API for binary diff and patch operations.
// It can be used as a library or via CLI.
package HexDiff

import (
	"fmt"
	"os"
	"time"

	"github.com/Sky-ey/HexDiff/pkg/cli"
	"github.com/Sky-ey/HexDiff/pkg/compression"
	"github.com/Sky-ey/HexDiff/pkg/diff"
	"github.com/Sky-ey/HexDiff/pkg/patch"
)

// Error represents a HexDiff-specific error
type Error struct {
	Op   string
	File string
	Err  error
}

func (e *Error) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s %s: %v", e.Op, e.File, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Common errors
var (
	ErrFileNotFound     = os.ErrNotExist
	ErrFileRead         = fmt.Errorf("file read error")
	ErrFileWrite        = fmt.Errorf("file write error")
	ErrPatchGeneration  = fmt.Errorf("patch generation failed")
	ErrPatchApplication = fmt.Errorf("patch application failed")
	ErrPatchValidation  = fmt.Errorf("patch validation failed")
	ErrInvalidArgument  = fmt.Errorf("invalid argument")
	ErrInvalidConfig    = fmt.Errorf("invalid configuration")
)

// CompressionType represents the compression algorithm
type CompressionType int

const (
	CompressionNone CompressionType = iota // No compression
	CompressionGzip                        // Gzip compression
	CompressionLZ4                         // LZ4 compression
	CompressionZstd                        // Zstandard compression
)

// String returns the compression type as a string
func (c CompressionType) String() string {
	switch c {
	case CompressionNone:
		return "none"
	case CompressionGzip:
		return "gzip"
	case CompressionLZ4:
		return "lz4"
	case CompressionZstd:
		return "zstd"
	default:
		return "unknown"
	}
}

// ============================================================================
// Progress Reporter
// ============================================================================

// ProgressFunc is a callback function for reporting progress
type ProgressFunc func(current, total int64, message string)

// noOpProgress is a no-op progress reporter
var noOpProgress ProgressFunc = func(current, total int64, message string) {}

// cliProgressAdapter converts ProgressFunc to cli.ProgressReporter and diff.ProgressReporter
type cliProgressAdapter struct {
	progress ProgressFunc
	current  int64
	total    int64
	finished bool
}

func (p *cliProgressAdapter) SetCurrent(value int64) {
	p.current = value
	if p.progress != nil {
		p.progress(p.current, p.total, "")
	}
}

func (p *cliProgressAdapter) Increment(delta int64) {
	p.current += delta
	if p.progress != nil {
		p.progress(p.current, p.total, "")
	}
}

func (p *cliProgressAdapter) IncProgress(delta int) {
	p.Increment(int64(delta))
}

func (p *cliProgressAdapter) SetTotal(value int64) {
	p.total = value
}

func (p *cliProgressAdapter) SetMessage(msg string) {
	if p.progress != nil {
		p.progress(p.current, p.total, msg)
	}
}

func (p *cliProgressAdapter) Message(msg string) {
	p.SetMessage(msg)
}

func (p *cliProgressAdapter) Finish() {
	p.finished = true
}

func (p *cliProgressAdapter) IsFinished() bool {
	return p.finished
}

func (p *cliProgressAdapter) SetProgress(percent int) {
	p.current = int64(percent) * p.total / 100
	if p.progress != nil {
		p.progress(p.current, p.total, "")
	}
}

// ============================================================================
// Configuration Types
// ============================================================================

// Config holds the configuration for HexDiff operations
type Config struct {
	// BlockSize is the block size for diff operations (default: 4096)
	BlockSize int
	// WindowSize is the rolling hash window size (default: 64)
	WindowSize int
	// EnableCRC32 enables CRC32 checksum (default: true)
	EnableCRC32 bool
	// EnableSHA256 enables SHA256 checksum (default: true)
	EnableSHA256 bool
	// MaxMemory is the maximum memory usage in bytes (default: 100MB)
	MaxMemory int64
	// Compression is the compression type (default: CompressionGzip)
	Compression CompressionType
	// Verify enables verification after patch application (default: true)
	Verify bool
	// Backup creates backup before applying patch (default: false)
	Backup bool
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		BlockSize:    4096,
		WindowSize:   64,
		EnableCRC32:  true,
		EnableSHA256: true,
		MaxMemory:    100 * 1024 * 1024,
		Compression:  CompressionGzip,
		Verify:       true,
		Backup:       false,
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.BlockSize < diff.MinBlockSize || c.BlockSize > diff.MaxBlockSize {
		return &Error{
			Op:  "validate config",
			Err: fmt.Errorf("block size must be between %d and %d", diff.MinBlockSize, diff.MaxBlockSize),
		}
	}
	if c.WindowSize < 8 || c.WindowSize > c.BlockSize {
		return &Error{
			Op:  "validate config",
			Err: fmt.Errorf("window size must be between 8 and block size"),
		}
	}
	if c.MaxMemory < 1024*1024 {
		return &Error{
			Op:  "validate config",
			Err: fmt.Errorf("max memory must be at least 1MB"),
		}
	}
	return nil
}

// DiffConfig converts Config to diff.DiffConfig
func (c *Config) DiffConfig() *diff.DiffConfig {
	return &diff.DiffConfig{
		BlockSize:    c.BlockSize,
		WindowSize:   c.WindowSize,
		EnableCRC32:  c.EnableCRC32,
		EnableSHA256: c.EnableSHA256,
		MaxMemory:    c.MaxMemory,
	}
}

// CompressionConfig converts CompressionType to compression config
func (c *CompressionType) CompressionConfig() compression.CompressionConfig {
	switch *c {
	case CompressionGzip:
		return compression.CompressionConfig{Type: compression.CompressionGzip}
	case CompressionLZ4:
		return compression.CompressionConfig{Type: compression.CompressionLZ4}
	default:
		return compression.CompressionConfig{Type: compression.CompressionNone}
	}
}

// ============================================================================
// Options Pattern (for chainable API)
// ============================================================================

// Option is a function that modifies a HexDiff instance
type Option func(h *HexDiff) error

// HexDiff is the main type for chainable API
type HexDiff struct {
	config      *Config
	progress    ProgressFunc
	engine      *cli.EngineAdapter
	initialized bool
}

// New creates a new HexDiff instance with default configuration
func New() *HexDiff {
	return &HexDiff{
		config:   DefaultConfig(),
		progress: noOpProgress,
	}
}

// WithBlockSize sets the block size for diff operations
func WithBlockSize(size int) Option {
	return func(h *HexDiff) error {
		if size < diff.MinBlockSize || size > diff.MaxBlockSize {
			return &Error{
				Op:  "option",
				Err: fmt.Errorf("block size must be between %d and %d", diff.MinBlockSize, diff.MaxBlockSize),
			}
		}
		h.config.BlockSize = size
		return nil
	}
}

// WithWindowSize sets the rolling hash window size
func WithWindowSize(size int) Option {
	return func(h *HexDiff) error {
		if size < 8 || size > h.config.BlockSize {
			return &Error{
				Op:  "option",
				Err: fmt.Errorf("window size must be between 8 and block size"),
			}
		}
		h.config.WindowSize = size
		return nil
	}
}

// WithCompression sets the compression type
func WithCompression(ct CompressionType) Option {
	return func(h *HexDiff) error {
		h.config.Compression = ct
		return nil
	}
}

// WithChecksum enables or disables checksum verification
func WithChecksum(enableCRC32, enableSHA256 bool) Option {
	return func(h *HexDiff) error {
		h.config.EnableCRC32 = enableCRC32
		h.config.EnableSHA256 = enableSHA256
		return nil
	}
}

// WithMaxMemory sets the maximum memory usage
func WithMaxMemory(bytes int64) Option {
	return func(h *HexDiff) error {
		if bytes < 1024*1024 {
			return &Error{
				Op:  "option",
				Err: fmt.Errorf("max memory must be at least 1MB"),
			}
		}
		h.config.MaxMemory = bytes
		return nil
	}
}

// WithProgress sets the progress callback function
func WithProgress(pf ProgressFunc) Option {
	return func(h *HexDiff) error {
		if pf == nil {
			h.progress = noOpProgress
		} else {
			h.progress = pf
		}
		return nil
	}
}

// WithVerify enables or disables verification after patch application
func WithVerify(verify bool) Option {
	return func(h *HexDiff) error {
		h.config.Verify = verify
		return nil
	}
}

// WithBackup enables or disables backup before patch application
func WithBackup(backup bool) Option {
	return func(h *HexDiff) error {
		h.config.Backup = backup
		return nil
	}
}

// WithConfig sets a complete configuration
func WithConfig(cfg *Config) Option {
	return func(h *HexDiff) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		h.config = cfg
		return nil
	}
}

// init initializes the engine if not already initialized
func (h *HexDiff) init() error {
	if h.initialized {
		return nil
	}

	if err := h.config.Validate(); err != nil {
		return err
	}

	// Use CLI's engine adapter with defaults
	// Note: Custom config support can be added later if needed
	engine, err := cli.NewEngineAdapter()
	if err != nil {
		return &Error{
			Op:  "initialize engine",
			Err: err,
		}
	}

	h.engine = engine
	h.initialized = true
	return nil
}

// Diff generates a patch from oldFile to newFile and writes it to outputFile
// Simple API: hexdiff.Diff("old.txt", "new.txt", "patch.patch")
func Diff(oldFile, newFile, outputFile string) error {
	return DiffWithProgress(oldFile, newFile, outputFile, nil)
}

// DiffWithProgress generates a patch with progress callback
func DiffWithProgress(oldFile, newFile, outputFile string, progress ProgressFunc) error {
	cfg := DefaultConfig()
	opts := []Option{}
	if progress != nil {
		opts = append(opts, WithProgress(progress))
	}
	opts = append(opts, WithConfig(cfg))

	h := New()
	for _, opt := range opts {
		if err := opt(h); err != nil {
			return err
		}
	}

	return h.DiffTo(oldFile, newFile, outputFile)
}

// DiffDir generates a directory patch
// Simple API: hexdiff.DiffDir("old_dir", "new_dir", "patch.dir.patch")
func DiffDir(oldDir, newDir, outputFile string) error {
	return DiffDirWithOptions(oldDir, newDir, outputFile, nil)
}

// DiffDirWithOptions generates a directory patch with options
func DiffDirWithOptions(oldDir, newDir, outputFile string, opts []Option) error {
	h := New()
	for _, opt := range opts {
		if err := opt(h); err != nil {
			return err
		}
	}

	return h.DiffDirTo(oldDir, newDir, outputFile)
}

// Apply applies a patch to targetFile and writes the result to outputFile
// Simple API: hexdiff.Apply("patch.patch", "old.txt", "new.txt")
func Apply(patchFile, targetFile, outputFile string) error {
	return ApplyWithProgress(patchFile, targetFile, outputFile, nil, false)
}

// ApplyWithProgress applies a patch with progress callback
func ApplyWithProgress(patchFile, targetFile, outputFile string, progress ProgressFunc, verify bool) error {
	cfg := DefaultConfig()
	cfg.Verify = verify
	opts := []Option{WithConfig(cfg)}
	if progress != nil {
		opts = append(opts, WithProgress(progress))
	}

	h := New()
	for _, opt := range opts {
		if err := opt(h); err != nil {
			return err
		}
	}

	return h.ApplyTo(patchFile, targetFile, outputFile)
}

// ApplyDir applies a directory patch
func ApplyDir(patchFile, targetDir string) error {
	return ApplyDirWithOptions(patchFile, targetDir, nil)
}

// ApplyDirWithOptions applies a directory patch with options
func ApplyDirWithOptions(patchFile, targetDir string, opts []Option) error {
	h := New()
	for _, opt := range opts {
		if err := opt(h); err != nil {
			return err
		}
	}

	return h.ApplyDirTo(patchFile, targetDir)
}

// Validate validates a patch file
// Simple API: hexdiff.Validate("patch.patch")
func Validate(patchFile string) (*ValidationResult, error) {
	return ValidateWithProgress(patchFile, nil)
}

// ValidateWithProgress validates a patch file with progress callback
func ValidateWithProgress(patchFile string, progress ProgressFunc) (*ValidationResult, error) {
	h := New()
	if progress != nil {
		if err := h.SetProgress(progress); err != nil {
			return nil, err
		}
	}

	return h.ValidatePatch(patchFile)
}

// GetPatchInfo gets information about a patch file
// Simple API: info, err := hexdiff.GetPatchInfo("patch.patch")
func GetPatchInfo(patchFile string) (*PatchInfo, error) {
	h := New()
	return h.GetInfo(patchFile)
}

// GetDirPatchInfo gets information about a directory patch file
func GetDirPatchInfo(patchFile string) (*DirPatchInfo, error) {
	h := New()
	return h.GetDirInfo(patchFile)
}

// ============================================================================
// Chainable API Methods (on HexDiff instance)
// ============================================================================

// DiffTo generates a patch (chainable API)
func (h *HexDiff) DiffTo(oldFile, newFile, outputFile string) error {
	if err := h.init(); err != nil {
		return err
	}

	progressAdapter := &cliProgressAdapter{progress: h.progress}
	compress := h.config.Compression != CompressionNone
	return h.engine.GeneratePatch(oldFile, newFile, outputFile, "", compress, progressAdapter)
}

// DiffDirTo generates a directory patch (chainable API)
func (h *HexDiff) DiffDirTo(oldDir, newDir, outputFile string) error {
	if err := h.init(); err != nil {
		return err
	}

	progressAdapter := &cliProgressAdapter{progress: h.progress}
	compress := h.config.Compression != CompressionNone

	dirConfig := diff.DefaultDirDiffConfig()
	dirConfig.BlockSize = h.config.BlockSize
	dirConfig.Compress = compress

	dirEngine, err := diff.NewDirEngine(nil, dirConfig)
	if err != nil {
		return &Error{
			Op:  "create dir engine",
			Err: err,
		}
	}

	result, err := dirEngine.GenerateDirDiff(oldDir, newDir, progressAdapter)
	if err != nil {
		return &Error{
			Op:  "generate dir diff",
			Err: err,
		}
	}

	dirPatchSerializer := patch.NewDirPatchSerializer(patch.CompressionNone)
	oldBase := ""
	newBase := ""
	err = dirPatchSerializer.SerializeDirPatch(result, oldBase, newBase, outputFile)
	if err != nil {
		return &Error{
			Op:  "serialize dir patch",
			Err: err,
		}
	}

	return nil
}

// ApplyTo applies a patch (chainable API)
func (h *HexDiff) ApplyTo(patchFile, targetFile, outputFile string) error {
	if err := h.init(); err != nil {
		return err
	}

	progressAdapter := &cliProgressAdapter{progress: h.progress}
	return h.engine.ApplyPatch(patchFile, targetFile, outputFile, h.config.Verify, progressAdapter)
}

// ApplyDirTo applies a directory patch (chainable API)
func (h *HexDiff) ApplyDirTo(patchFile, targetDir string) error {
	if err := h.init(); err != nil {
		return err
	}

	progressAdapter := &cliProgressAdapter{progress: h.progress}
	_, err := h.engine.ApplyDirPatch(patchFile, targetDir, h.config.Verify, progressAdapter)
	if err != nil {
		return &Error{
			Op:  "apply dir patch",
			Err: err,
		}
	}
	return nil
}

// ValidatePatch validates a patch file (chainable API)
func (h *HexDiff) ValidatePatch(patchFile string) (*ValidationResult, error) {
	if err := h.init(); err != nil {
		return nil, err
	}

	progressAdapter := &cliProgressAdapter{progress: h.progress}
	result, err := h.engine.ValidatePatch(patchFile, progressAdapter)
	if err != nil {
		return nil, &Error{
			Op:  "validate patch",
			Err: err,
		}
	}

	return &ValidationResult{
		Valid:         result.Valid,
		ValidFormat:   result.ValidFormat,
		ValidChecksum: result.ValidChecksum,
		ValidData:     result.ValidData,
		Errors:        result.Errors,
	}, nil
}

// GetInfo gets patch file information (chainable API)
func (h *HexDiff) GetInfo(patchFile string) (*PatchInfo, error) {
	if err := h.init(); err != nil {
		return nil, err
	}

	info, err := h.engine.GetPatchInfo(patchFile)
	if err != nil {
		return nil, &Error{
			Op:  "get patch info",
			Err: err,
		}
	}

	return &PatchInfo{
		Version:        info.Version,
		Compression:    CompressionType(info.Compression),
		SourceChecksum: info.SourceChecksum,
		TargetChecksum: info.TargetChecksum,
		OperationCount: info.OperationCount,
		PatchSize:      info.PatchSize,
		CreatedAt:      info.CreatedAt,
		Metadata:       info.Metadata,
	}, nil
}

// GetDirInfo gets directory patch information (chainable API)
func (h *HexDiff) GetDirInfo(patchFile string) (*DirPatchInfo, error) {
	if err := h.init(); err != nil {
		return nil, err
	}

	info, err := h.engine.GetDirPatchInfo(patchFile)
	if err != nil {
		return nil, &Error{
			Op:  "get dir patch info",
			Err: err,
		}
	}

	return &DirPatchInfo{
		Version:          info.Version,
		OldDir:           info.OldDir,
		NewDir:           info.NewDir,
		FileCount:        info.FileCount,
		AddedFiles:       info.AddedFiles,
		DeletedFiles:     info.DeletedFiles,
		ModifiedFiles:    info.ModifiedFiles,
		UnchangedFiles:   info.UnchangedFiles,
		PatchSize:        info.PatchSize,
		CreatedAt:        info.CreatedAt,
		AddedFileList:    info.AddedFileList,
		DeletedFileList:  info.DeletedFileList,
		ModifiedFileList: info.ModifiedFileList,
	}, nil
}

// SetProgress sets the progress callback (chainable API)
func (h *HexDiff) SetProgress(pf ProgressFunc) error {
	if pf == nil {
		h.progress = noOpProgress
	} else {
		h.progress = pf
	}
	return nil
}

// Config returns the current configuration (chainable API)
func (h *HexDiff) Config() *Config {
	return h.config
}

// ============================================================================
// Result Types
// ============================================================================

// ValidationResult represents the result of patch validation
type ValidationResult struct {
	Valid         bool
	ValidFormat   bool
	ValidChecksum bool
	ValidData     bool
	Errors        []string
}

// PatchInfo represents information about a patch file
type PatchInfo struct {
	Version        uint16
	Compression    CompressionType
	SourceChecksum []byte
	TargetChecksum []byte
	OperationCount int
	PatchSize      int64
	CreatedAt      time.Time
	Metadata       map[string]string
}

// DirPatchInfo represents information about a directory patch file
type DirPatchInfo struct {
	Version          uint16
	OldDir           string
	NewDir           string
	FileCount        int
	AddedFiles       int
	DeletedFiles     int
	ModifiedFiles    int
	UnchangedFiles   int
	PatchSize        int64
	CreatedAt        time.Time
	AddedFileList    []string
	DeletedFileList  []string
	ModifiedFileList []string
}

// ============================================================================
// Re-exported Types (for advanced users)
// ============================================================================

// Re-export diff types
type (
	// Block represents a data block in diff operations
	Block = diff.Block
	// Operation represents a diff operation
	Operation = diff.Operation
	// OperationType is the type of diff operation
	OperationType = diff.OperationType
	// Delta represents diff results
	Delta = diff.Delta
	// Signature represents a file signature
	Signature = diff.Signature
	// DiffConfig represents diff configuration
	DiffConfig = diff.DiffConfig
)

// Re-export patch types
type (
	// PatchHeader represents a patch file header
	PatchHeader = patch.PatchHeader
)

// Constants
const (
	// DefaultBlockSize is the default block size
	DefaultBlockSize = diff.DefaultBlockSize
	// MinBlockSize is the minimum allowed block size
	MinBlockSize = diff.MinBlockSize
	// MaxBlockSize is the maximum allowed block size
	MaxBlockSize = diff.MaxBlockSize

	// OpCopy is the copy operation type
	OpCopy = diff.OpCopy
	// OpInsert is the insert operation type
	OpInsert = diff.OpInsert
	// OpDelete is the delete operation type
	OpDelete = diff.OpDelete
)
