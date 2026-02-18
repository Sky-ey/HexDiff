package diff

import (
	"os"
	"time"
)

type FileStatus uint8

const (
	StatusUnchanged FileStatus = iota // 未改变
	StatusAdded                       // 新增
	StatusDeleted                     // 删除
	StatusModified                    // 修改
)

// String 返回文件状态的字符串表示
func (s FileStatus) String() string {
	switch s {
	case StatusUnchanged:
		return "unchanged"
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	case StatusModified:
		return "modified"
	default:
		return "unknown"
	}
}

// FileEntry 目录中的文件条目
type FileEntry struct {
	Path         string      // 相对于目录的路径
	RelativePath string      // 相对路径（用于匹配）
	AbsPath      string      // 绝对路径
	Size         int64       // 文件大小
	Mode         os.FileMode // 文件权限
	MTime        time.Time   // 修改时间
	IsDir        bool        // 是否是目录
	IsSymlink    bool        // 是否是符号链接
}

// DirDiffResult 目录差异结果
type DirDiffResult struct {
	OldDir         string               // 旧目录路径
	NewDir         string               // 新目录路径
	Files          map[string]*FileDiff // 文件差异映射（键为相对路径）
	AddedFiles     []*FileDiff          // 新增文件列表
	DeletedFiles   []*FileDiff          // 删除文件列表
	ModifiedFiles  []*FileDiff          // 修改文件列表
	UnchangedFiles []*FileDiff          // 未改变文件列表
	TotalFiles     int                  // 总文件数
	ChangedFiles   int                  // 改变的文件数
}

// FileDiff 单个文件的差异
type FileDiff struct {
	RelativePath string     // 相对路径
	Status       FileStatus // 文件状态
	OldEntry     *FileEntry // 旧文件信息（删除/修改时有值）
	NewEntry     *FileEntry // 新文件信息（新增/修改时有值）
	Delta        *Delta     // 二进制差异（仅修改时有值）
	PatchData    []byte     // 补丁数据（新增文件时为完整内容）
}

// DirDiffConfig 目录差异检测配置
type DirDiffConfig struct {
	Recursive      bool     // 是否递归遍历子目录
	IgnorePatterns []string // 忽略的文件模式
	FollowSymlinks bool     // 是否跟随符号链接
	IgnoreHidden   bool     // 是否忽略隐藏文件
	UseSignature   bool     // 是否使用签名加速
	Compress       bool     // 是否压缩补丁
	WorkerCount    int      // 并行工作协程数
	BlockSize      int      // 块大小
}

// DefaultDirDiffConfig 默认目录差异检测配置
func DefaultDirDiffConfig() *DirDiffConfig {
	return &DirDiffConfig{
		Recursive:      true,
		IgnorePatterns: []string{".git", "__pycache__", "node_modules", ".DS_Store", "*.swp"},
		FollowSymlinks: false,
		IgnoreHidden:   false,
		UseSignature:   true,
		Compress:       true,
		WorkerCount:    4,
		BlockSize:      DefaultBlockSize,
	}
}

// Validate 验证配置参数
func (c *DirDiffConfig) Validate() error {
	if c.WorkerCount < 1 || c.WorkerCount > 32 {
		return ErrInvalidWorkerCount
	}
	if c.BlockSize < MinBlockSize || c.BlockSize > MaxBlockSize {
		return ErrInvalidBlockSize
	}
	return nil
}

// DirPatch 目录补丁
type DirPatch struct {
	Version   uint16            // 版本号
	Timestamp int64             // 创建时间戳
	OldDir    string            // 旧目录名
	NewDir    string            // 新目录名
	Files     []*DirPatchFile   // 文件补丁列表
	Metadata  map[string]string // 元数据
}

// DirPatchFile 单个文件的补丁信息
type DirPatchFile struct {
	RelativePath  string      // 相对路径
	Status        FileStatus  // 文件状态
	Mode          os.FileMode // 文件权限
	MTime         int64       // 修改时间戳
	Size          int64       // 文件大小
	Checksum      [32]byte    // SHA-256校验和
	DeltaSize     int64       // 补丁数据大小
	Delta         []byte      // 补丁数据（修改/新增时使用）
	IsFullContent bool        // 是否为完整内容（新增文件）
}

// NewDirDiffResult 创建新的目录差异结果
func NewDirDiffResult(oldDir, newDir string) *DirDiffResult {
	return &DirDiffResult{
		OldDir:         oldDir,
		NewDir:         newDir,
		Files:          make(map[string]*FileDiff),
		AddedFiles:     make([]*FileDiff, 0),
		DeletedFiles:   make([]*FileDiff, 0),
		ModifiedFiles:  make([]*FileDiff, 0),
		UnchangedFiles: make([]*FileDiff, 0),
	}
}

// AddFileDiff 添加文件差异
func (r *DirDiffResult) AddFileDiff(diff *FileDiff) {
	r.Files[diff.RelativePath] = diff

	switch diff.Status {
	case StatusAdded:
		r.AddedFiles = append(r.AddedFiles, diff)
	case StatusDeleted:
		r.DeletedFiles = append(r.DeletedFiles, diff)
	case StatusModified:
		r.ModifiedFiles = append(r.ModifiedFiles, diff)
	case StatusUnchanged:
		r.UnchangedFiles = append(r.UnchangedFiles, diff)
	}

	r.TotalFiles++
	if diff.Status != StatusUnchanged {
		r.ChangedFiles++
	}
}

// NewDirPatch 创建新的目录补丁
func NewDirPatch(oldDir, newDir string) *DirPatch {
	return &DirPatch{
		Version:   1,
		Timestamp: time.Now().Unix(),
		OldDir:    oldDir,
		NewDir:    newDir,
		Files:     make([]*DirPatchFile, 0),
		Metadata:  make(map[string]string),
	}
}

// AddFile 添加文件补丁
func (p *DirPatch) AddFile(file *DirPatchFile) {
	p.Files = append(p.Files, file)
}

// GetFileCount 获取文件数量
func (p *DirPatch) GetFileCount() int {
	return len(p.Files)
}

func (fp *DirPatchFile) GetMTime() time.Time {
	return time.Unix(fp.MTime, 0)
}
