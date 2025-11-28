package patch

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sky-ey/HexDiff/pkg/diff"
)

// Generator 补丁生成器
type Generator struct {
	engine     *diff.Engine
	serializer *Serializer
}

// NewGenerator 创建新的补丁生成器
func NewGenerator(engine *diff.Engine, compression CompressionType) *Generator {
	return &Generator{
		engine:     engine,
		serializer: NewSerializer(compression),
	}
}

// GeneratePatch 生成补丁文件
func (g *Generator) GeneratePatch(oldFilePath, newFilePath, patchPath string) (*PatchInfo, error) {
	// 生成差异
	delta, err := g.engine.GenerateDelta(oldFilePath, newFilePath)
	if err != nil {
		return nil, fmt.Errorf("generate delta: %w", err)
	}

	// 计算源文件校验和
	sourceChecksum, err := g.calculateFileChecksum(oldFilePath)
	if err != nil {
		return nil, fmt.Errorf("calculate source checksum: %w", err)
	}

	// 序列化补丁
	if err := g.serializer.SerializeDelta(delta, sourceChecksum, patchPath); err != nil {
		return nil, fmt.Errorf("serialize patch: %w", err)
	}

	// 获取补丁文件信息
	patchInfo, err := g.getPatchFileInfo(patchPath, oldFilePath, newFilePath)
	if err != nil {
		return nil, fmt.Errorf("get patch info: %w", err)
	}

	return patchInfo, nil
}

// GeneratePatchWithMmap 使用内存映射生成补丁（适用于大文件）
func (g *Generator) GeneratePatchWithMmap(oldFilePath, newFilePath, patchPath string) (*PatchInfo, error) {
	// 使用内存映射打开文件
	oldFile, err := NewMappedFile(oldFilePath, true)
	if err != nil {
		return nil, fmt.Errorf("map old file: %w", err)
	}
	defer oldFile.Close()

	newFile, err := NewMappedFile(newFilePath, true)
	if err != nil {
		return nil, fmt.Errorf("map new file: %w", err)
	}
	defer newFile.Close()

	// 建议顺序访问
	oldFile.AdviseSequential()
	newFile.AdviseSequential()

	// 生成差异（这里可以进一步优化使用内存映射的数据）
	delta, err := g.engine.GenerateDelta(oldFilePath, newFilePath)
	if err != nil {
		return nil, fmt.Errorf("generate delta: %w", err)
	}

	// 计算源文件校验和
	sourceChecksum := sha256.Sum256(oldFile.Data())

	// 序列化补丁
	if err := g.serializer.SerializeDelta(delta, sourceChecksum, patchPath); err != nil {
		return nil, fmt.Errorf("serialize patch: %w", err)
	}

	// 获取补丁文件信息
	patchInfo, err := g.getPatchFileInfo(patchPath, oldFilePath, newFilePath)
	if err != nil {
		return nil, fmt.Errorf("get patch info: %w", err)
	}

	return patchInfo, nil
}

// calculateFileChecksum 计算文件校验和
func (g *Generator) calculateFileChecksum(filePath string) ([32]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return [32]byte{}, err
	}
	defer file.Close()

	hasher := sha256.New()
	buffer := make([]byte, 64*1024) // 64KB缓冲区

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			hasher.Write(buffer[:n])
		}
		if err != nil {
			break
		}
	}

	var checksum [32]byte
	copy(checksum[:], hasher.Sum(nil))
	return checksum, nil
}

// getPatchFileInfo 获取补丁文件信息
func (g *Generator) getPatchFileInfo(patchPath, oldFilePath, newFilePath string) (*PatchInfo, error) {
	// 获取文件大小
	oldStat, err := os.Stat(oldFilePath)
	if err != nil {
		return nil, err
	}

	newStat, err := os.Stat(newFilePath)
	if err != nil {
		return nil, err
	}

	patchStat, err := os.Stat(patchPath)
	if err != nil {
		return nil, err
	}

	// 读取补丁头信息
	header, err := GetPatchInfo(patchPath)
	if err != nil {
		return nil, err
	}

	return &PatchInfo{
		PatchPath:      patchPath,
		OldFilePath:    oldFilePath,
		NewFilePath:    newFilePath,
		OldFileSize:    oldStat.Size(),
		NewFileSize:    newStat.Size(),
		PatchFileSize:  patchStat.Size(),
		OperationCount: int(header.OperationCount),
		Compression:    header.Compression,
		CreatedAt:      header.Timestamp,
		SourceChecksum: header.SourceChecksum,
		TargetChecksum: header.TargetChecksum,
	}, nil
}

// PatchInfo 补丁信息
type PatchInfo struct {
	PatchPath      string          // 补丁文件路径
	OldFilePath    string          // 原文件路径
	NewFilePath    string          // 新文件路径
	OldFileSize    int64           // 原文件大小
	NewFileSize    int64           // 新文件大小
	PatchFileSize  int64           // 补丁文件大小
	OperationCount int             // 操作数量
	Compression    CompressionType // 压缩类型
	CreatedAt      int64           // 创建时间
	SourceChecksum [32]byte        // 源文件校验和
	TargetChecksum [32]byte        // 目标文件校验和
}

// CompressionRatio 计算压缩比
func (pi *PatchInfo) CompressionRatio() float64 {
	if pi.NewFileSize == 0 {
		return 0
	}
	return float64(pi.PatchFileSize) / float64(pi.NewFileSize) * 100
}

// SizeReduction 计算大小减少百分比
func (pi *PatchInfo) SizeReduction() float64 {
	if pi.NewFileSize == 0 {
		return 0
	}
	saved := pi.NewFileSize - pi.PatchFileSize
	return float64(saved) / float64(pi.NewFileSize) * 100
}

// String 返回补丁信息的字符串表示
func (pi *PatchInfo) String() string {
	return fmt.Sprintf(`补丁信息:
  补丁文件: %s
  原文件: %s (%d 字节)
  新文件: %s (%d 字节)
  补丁大小: %d 字节
  操作数量: %d
  压缩类型: %s
  压缩比: %.2f%%
  大小减少: %.2f%%
  源文件校验和: %x
  目标文件校验和: %x`,
		filepath.Base(pi.PatchPath),
		filepath.Base(pi.OldFilePath), pi.OldFileSize,
		filepath.Base(pi.NewFilePath), pi.NewFileSize,
		pi.PatchFileSize,
		pi.OperationCount,
		pi.Compression.String(),
		pi.CompressionRatio(),
		pi.SizeReduction(),
		pi.SourceChecksum[:8], // 只显示前8字节
		pi.TargetChecksum[:8],
	)
}

// ValidateChecksums 验证校验和
func (pi *PatchInfo) ValidateChecksums(oldFilePath, newFilePath string) error {
	// 验证源文件校验和
	oldChecksum, err := calculateFileChecksum(oldFilePath)
	if err != nil {
		return fmt.Errorf("calculate old file checksum: %w", err)
	}

	if oldChecksum != pi.SourceChecksum {
		return fmt.Errorf("source file checksum mismatch")
	}

	// 验证目标文件校验和
	newChecksum, err := calculateFileChecksum(newFilePath)
	if err != nil {
		return fmt.Errorf("calculate new file checksum: %w", err)
	}

	if newChecksum != pi.TargetChecksum {
		return fmt.Errorf("target file checksum mismatch")
	}

	return nil
}

// calculateFileChecksum 计算文件校验和（独立函数）
func calculateFileChecksum(filePath string) ([32]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return [32]byte{}, err
	}
	defer file.Close()

	hasher := sha256.New()
	buffer := make([]byte, 64*1024)

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			hasher.Write(buffer[:n])
		}
		if err != nil {
			break
		}
	}

	var result [32]byte
	copy(result[:], hasher.Sum(nil))
	return result, nil
}
