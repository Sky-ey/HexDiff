package patch

import (
	"fmt"
	"os"
	"strings"
)

// Validator 补丁验证器
type Validator struct{}

// NewValidator 创建新的验证器
func NewValidator() *Validator {
	return &Validator{}
}

// ValidatePatchFile 验证补丁文件的完整性
func (v *Validator) ValidatePatchFile(patchFilePath string) (*ValidationResult, error) {
	result := &ValidationResult{
		PatchFilePath: patchFilePath,
		Valid:         false,
		Issues:        make([]string, 0),
	}

	// 检查文件是否存在
	if _, err := os.Stat(patchFilePath); os.IsNotExist(err) {
		result.Issues = append(result.Issues, "补丁文件不存在")
		return result, nil
	}

	// 读取补丁文件
	serializer := NewSerializer(CompressionNone)
	patchFile, err := serializer.DeserializePatch(patchFilePath)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("无法解析补丁文件: %v", err))
		return result, nil
	}

	// 验证文件头
	if err := v.validateHeader(patchFile.Header, result); err != nil {
		return result, err
	}

	// 验证操作列表
	if err := v.validateOperations(patchFile.Operations, patchFile.Data, result); err != nil {
		return result, err
	}

	// 验证数据完整性
	if err := v.validateData(patchFile.Data, result); err != nil {
		return result, err
	}

	// 如果没有问题，标记为有效
	if len(result.Issues) == 0 {
		result.Valid = true
	}

	return result, nil
}

// validateHeader 验证文件头
func (v *Validator) validateHeader(header *PatchHeader, result *ValidationResult) error {
	// 验证魔数
	if header.Magic != MagicNumber {
		result.Issues = append(result.Issues, fmt.Sprintf("无效的魔数: %x", header.Magic))
	}

	// 验证版本
	if header.Version != Version {
		result.Issues = append(result.Issues, fmt.Sprintf("不支持的版本: %d", header.Version))
	}

	// 验证文件大小
	if header.SourceSize < 0 {
		result.Issues = append(result.Issues, fmt.Sprintf("无效的源文件大小: %d", header.SourceSize))
	}

	if header.TargetSize < 0 {
		result.Issues = append(result.Issues, fmt.Sprintf("无效的目标文件大小: %d", header.TargetSize))
	}

	// 验证操作数量
	if header.OperationCount == 0 {
		result.Issues = append(result.Issues, "操作数量为零")
	}

	return nil
}

// validateOperations 验证操作列表
func (v *Validator) validateOperations(operations []PatchOperation, data []byte, result *ValidationResult) error {
	for i, op := range operations {
		// 验证操作类型
		if op.Type > 2 {
			result.Issues = append(result.Issues, fmt.Sprintf("操作 %d: 无效的操作类型 %d", i, op.Type))
		}

		// 验证操作大小
		if op.Size == 0 {
			result.Issues = append(result.Issues, fmt.Sprintf("操作 %d: 操作大小为零", i))
		}

		// 对于插入操作，验证数据偏移量
		if op.Type == 1 { // Insert操作
			if op.DataOffset+op.Size > uint32(len(data)) {
				result.Issues = append(result.Issues, fmt.Sprintf("操作 %d: 插入数据超出范围", i))
			}
		}

		// 验证偏移量的合理性
		if op.Type == 0 { // Copy操作
			result.Issues = append(result.Issues, fmt.Sprintf("操作 %d: 无效的源偏移量", i))
		}
	}

	return nil
}

// validateData 验证数据完整性
func (v *Validator) validateData(data []byte, result *ValidationResult) error {
	// 这里可以添加更多的数据完整性检查
	// 例如：检查数据是否符合预期的格式、是否有损坏等

	if len(data) == 0 {
		result.Issues = append(result.Issues, "补丁数据为空")
	}

	return nil
}

// ValidateSourceFile 验证源文件与补丁的兼容性
func (v *Validator) ValidateSourceFile(sourceFilePath, patchFilePath string) (*ValidationResult, error) {
	result := &ValidationResult{
		PatchFilePath: patchFilePath,
		Valid:         false,
		Issues:        make([]string, 0),
	}

	// 读取补丁文件头
	header, err := GetPatchInfo(patchFilePath)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("无法读取补丁信息: %v", err))
		return result, nil
	}

	// 检查源文件是否存在
	if _, err := os.Stat(sourceFilePath); os.IsNotExist(err) {
		result.Issues = append(result.Issues, "源文件不存在")
		return result, nil
	}

	// 验证源文件大小
	fileInfo, err := os.Stat(sourceFilePath)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("无法获取源文件信息: %v", err))
		return result, nil
	}

	if fileInfo.Size() != header.SourceSize {
		result.Issues = append(result.Issues, fmt.Sprintf("源文件大小不匹配: 期望 %d 字节，实际 %d 字节",
			header.SourceSize, fileInfo.Size()))
	}

	// 验证源文件校验和
	actualChecksum, err := calculateFileChecksum(sourceFilePath)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("无法计算源文件校验和: %v", err))
		return result, nil
	}

	if actualChecksum != header.SourceChecksum {
		result.Issues = append(result.Issues, "源文件校验和不匹配")
	}

	// 如果没有问题，标记为有效
	if len(result.Issues) == 0 {
		result.Valid = true
	}

	return result, nil
}

// ValidationResult 验证结果
type ValidationResult struct {
	PatchFilePath string   // 补丁文件路径
	Valid         bool     // 是否有效
	Issues        []string // 问题列表
}

// String 返回验证结果的字符串表示
func (r *ValidationResult) String() string {
	if r.Valid {
		return fmt.Sprintf("补丁文件 %s 验证通过 ✅", r.PatchFilePath)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("补丁文件 %s 验证失败 ❌\n问题:\n", r.PatchFilePath))
	for i, issue := range r.Issues {
		result.WriteString(fmt.Sprintf("  %d. %s\n", i+1, issue))
	}

	return result.String()
}

// HasIssues 检查是否有问题
func (r *ValidationResult) HasIssues() bool {
	return len(r.Issues) > 0
}

// GetIssueCount 获取问题数量
func (r *ValidationResult) GetIssueCount() int {
	return len(r.Issues)
}
