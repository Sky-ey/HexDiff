package compression

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"HexDiff/pkg/metadata"
)

// EnhancedPatchHeader 增强补丁文件头
type EnhancedPatchHeader struct {
	Magic            uint32           // 魔数 "HXDF"
	Version          uint16           // 版本号
	CompressionType  CompressionType  // 压缩类型
	CompressionLevel CompressionLevel // 压缩级别
	Timestamp        int64            // 创建时间戳
	SourceSize       int64            // 源文件大小
	TargetSize       int64            // 目标文件大小
	SourceChecksum   [32]byte         // 源文件SHA-256校验和
	TargetChecksum   [32]byte         // 目标文件SHA-256校验和
	OperationCount   uint32           // 操作数量
	DataOffset       uint32           // 数据区偏移量
	MetadataOffset   uint32           // 元数据偏移量
	MetadataSize     uint32           // 元数据大小
	Reserved         [16]byte         // 保留字段
}

const (
	EnhancedMagicNumber = 0x48584446 // "HXDF"
	EnhancedHeaderSize  = 128        // 增强头部大小
)

// EnhancedPatchFile 增强补丁文件
type EnhancedPatchFile struct {
	Header   *EnhancedPatchHeader
	Metadata *metadata.PatchMetadata
	Data     []byte
}

// EnhancedPatchManager 增强补丁管理器
type EnhancedPatchManager struct {
	compressionManager *CompressionManager
	metadataManager    *metadata.MetadataManager
}

// NewEnhancedPatchManager 创建增强补丁管理器
func NewEnhancedPatchManager(compressionManager *CompressionManager, metadataManager *metadata.MetadataManager) *EnhancedPatchManager {
	return &EnhancedPatchManager{
		compressionManager: compressionManager,
		metadataManager:    metadataManager,
	}
}

// CreateEnhancedPatch 创建增强补丁
func (epm *EnhancedPatchManager) CreateEnhancedPatch(
	sourceFile, targetFile, patchFile string,
	compressionType CompressionType,
	compressionLevel CompressionLevel,
) error {
	startTime := time.Now()

	// 创建补丁元数据
	patchMetadata := epm.metadataManager.CreateMetadata(patchFile)

	// 读取源文件和目标文件信息
	sourceInfo, err := os.Stat(sourceFile)
	if err != nil {
		return fmt.Errorf("获取源文件信息失败: %w", err)
	}

	targetInfo, err := os.Stat(targetFile)
	if err != nil {
		return fmt.Errorf("获取目标文件信息失败: %w", err)
	}

	// 设置文件信息到元数据
	patchMetadata.SetSourceFileInfo(
		sourceInfo.Name(),
		sourceFile,
		sourceInfo.Size(),
		"", // 校验和稍后计算
	)

	patchMetadata.SetTargetFileInfo(
		targetInfo.Name(),
		targetFile,
		targetInfo.Size(),
		"", // 校验和稍后计算
	)

	// 生成差异数据（这里简化处理，实际应该调用差异引擎）
	diffData := []byte("placeholder diff data")

	// 压缩差异数据
	compressor, err := epm.compressionManager.GetCompressor(compressionType)
	if err != nil {
		return fmt.Errorf("获取压缩器失败: %w", err)
	}

	compressedData, err := compressor.Compress(diffData)
	if err != nil {
		return fmt.Errorf("压缩数据失败: %w", err)
	}

	// 创建增强补丁头部
	header := &EnhancedPatchHeader{
		Magic:            EnhancedMagicNumber,
		Version:          1,
		CompressionType:  compressionType,
		CompressionLevel: compressionLevel,
		Timestamp:        time.Now().Unix(),
		SourceSize:       sourceInfo.Size(),
		TargetSize:       targetInfo.Size(),
		OperationCount:   1, // 简化处理
		DataOffset:       EnhancedHeaderSize,
	}

	// 序列化元数据
	metadataBytes, err := epm.serializeMetadata(patchMetadata)
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %w", err)
	}

	// 设置元数据偏移和大小
	header.MetadataOffset = uint32(EnhancedHeaderSize + len(compressedData))
	header.MetadataSize = uint32(len(metadataBytes))

	// 写入补丁文件
	err = epm.writeEnhancedPatchFile(patchFile, header, compressedData, metadataBytes)
	if err != nil {
		return fmt.Errorf("写入补丁文件失败: %w", err)
	}

	// 更新性能信息
	duration := time.Since(startTime)
	patchMetadata.SetPerformanceInfo(
		duration.Milliseconds(),
		0, // 压缩时间单独计算
		0, // 内存使用量
		float64(len(compressedData))/duration.Seconds()/(1024*1024), // MB/s
	)

	// 设置补丁信息
	compressionRatio := float64(len(compressedData)) / float64(len(diffData))
	patchMetadata.SetPatchInfo(
		int64(len(compressedData)),
		compressionType.String(),
		compressionRatio,
		1, // 操作数量
		"rolling_hash",
	)

	// 保存元数据到单独文件
	return epm.metadataManager.SaveMetadata(patchFile, patchMetadata)
}

// LoadEnhancedPatch 加载增强补丁
func (epm *EnhancedPatchManager) LoadEnhancedPatch(patchFile string) (*EnhancedPatchFile, error) {
	file, err := os.Open(patchFile)
	if err != nil {
		return nil, fmt.Errorf("打开补丁文件失败: %w", err)
	}
	defer file.Close()

	// 读取头部
	header, err := epm.readEnhancedHeader(file)
	if err != nil {
		return nil, fmt.Errorf("读取补丁头部失败: %w", err)
	}

	// 验证魔数
	if header.Magic != EnhancedMagicNumber {
		return nil, fmt.Errorf("无效的补丁文件格式")
	}

	// 读取压缩数据
	dataSize := header.MetadataOffset - header.DataOffset
	compressedData := make([]byte, dataSize)
	_, err = file.ReadAt(compressedData, int64(header.DataOffset))
	if err != nil {
		return nil, fmt.Errorf("读取压缩数据失败: %w", err)
	}

	// 解压数据
	decompressor, err := epm.compressionManager.GetDecompressor(header.CompressionType)
	if err != nil {
		return nil, fmt.Errorf("获取解压器失败: %w", err)
	}

	data, err := decompressor.Decompress(compressedData)
	if err != nil {
		return nil, fmt.Errorf("解压数据失败: %w", err)
	}

	// 读取元数据
	metadataBytes := make([]byte, header.MetadataSize)
	_, err = file.ReadAt(metadataBytes, int64(header.MetadataOffset))
	if err != nil {
		return nil, fmt.Errorf("读取元数据失败: %w", err)
	}

	patchMetadata, err := epm.deserializeMetadata(metadataBytes)
	if err != nil {
		return nil, fmt.Errorf("反序列化元数据失败: %w", err)
	}

	return &EnhancedPatchFile{
		Header:   header,
		Metadata: patchMetadata,
		Data:     data,
	}, nil
}

// ValidateEnhancedPatch 验证增强补丁
func (epm *EnhancedPatchManager) ValidateEnhancedPatch(patchFile string) error {
	patch, err := epm.LoadEnhancedPatch(patchFile)
	if err != nil {
		return err
	}

	// 验证头部
	if patch.Header.Version == 0 {
		return fmt.Errorf("无效的补丁版本")
	}

	// 验证元数据
	issues := epm.metadataManager.ValidateMetadata(patch.Metadata)
	if len(issues) > 0 {
		return fmt.Errorf("元数据验证失败: %v", issues)
	}

	// 验证压缩数据
	err = epm.compressionManager.ValidateCompressedData(patch.Data, patch.Header.CompressionType)
	if err != nil {
		return fmt.Errorf("压缩数据验证失败: %w", err)
	}

	return nil
}

// GetPatchInfo 获取补丁信息
func (epm *EnhancedPatchManager) GetPatchInfo(patchFile string) (*EnhancedPatchInfo, error) {
	patch, err := epm.LoadEnhancedPatch(patchFile)
	if err != nil {
		return nil, err
	}

	info := &EnhancedPatchInfo{
		Version:          patch.Header.Version,
		CompressionType:  patch.Header.CompressionType,
		CompressionLevel: patch.Header.CompressionLevel,
		CreatedAt:        time.Unix(patch.Header.Timestamp, 0),
		SourceSize:       patch.Header.SourceSize,
		TargetSize:       patch.Header.TargetSize,
		PatchSize:        int64(len(patch.Data)),
		OperationCount:   patch.Header.OperationCount,
		Metadata:         patch.Metadata,
	}

	return info, nil
}

// EnhancedPatchInfo 增强补丁信息
type EnhancedPatchInfo struct {
	Version          uint16                  `json:"version"`
	CompressionType  CompressionType         `json:"compression_type"`
	CompressionLevel CompressionLevel        `json:"compression_level"`
	CreatedAt        time.Time               `json:"created_at"`
	SourceSize       int64                   `json:"source_size"`
	TargetSize       int64                   `json:"target_size"`
	PatchSize        int64                   `json:"patch_size"`
	OperationCount   uint32                  `json:"operation_count"`
	Metadata         *metadata.PatchMetadata `json:"metadata"`
}

// 辅助方法

func (epm *EnhancedPatchManager) readEnhancedHeader(file *os.File) (*EnhancedPatchHeader, error) {
	header := &EnhancedPatchHeader{}

	err := binary.Read(file, binary.LittleEndian, header)
	if err != nil {
		return nil, err
	}

	return header, nil
}

func (epm *EnhancedPatchManager) writeEnhancedPatchFile(patchFile string, header *EnhancedPatchHeader, data, metadata []byte) error {
	file, err := os.Create(patchFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入头部
	err = binary.Write(file, binary.LittleEndian, header)
	if err != nil {
		return err
	}

	// 写入压缩数据
	_, err = file.Write(data)
	if err != nil {
		return err
	}

	// 写入元数据
	_, err = file.Write(metadata)
	if err != nil {
		return err
	}

	return nil
}

func (epm *EnhancedPatchManager) serializeMetadata(metadata *metadata.PatchMetadata) ([]byte, error) {
	// 使用JSON序列化元数据
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("序列化元数据失败: %w", err)
	}
	return data, nil
}

func (epm *EnhancedPatchManager) deserializeMetadata(data []byte) (*metadata.PatchMetadata, error) {
	var metadata metadata.PatchMetadata
	err := json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, fmt.Errorf("反序列化元数据失败: %w", err)
	}
	return &metadata, nil
}
