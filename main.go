package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Sky-ey/HexDiff/pkg/diff"
	"github.com/Sky-ey/HexDiff/pkg/integrity"
	"github.com/Sky-ey/HexDiff/pkg/patch"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	switch command {
	case "signature":
		handleSignature()
	case "diff":
		handleDiff()
	case "patch":
		handlePatch()
	case "apply":
		handleApply()
	case "validate":
		handleValidate()
	case "info":
		handleInfo()
	case "verify":
		handleVerify()
	case "backup":
		handleBackup()
	case "recover":
		handleRecover()
	case "test-integrity":
		handleTestIntegrity()
	case "help":
		printUsage()
	default:
		fmt.Printf("未知命令: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("HexDiff - 高效二进制补丁工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  hexdiff signature <文件路径>                         - 生成文件签名")
	fmt.Println("  hexdiff diff <旧文件> <新文件>                       - 生成差异分析")
	fmt.Println("  hexdiff patch <旧文件> <新文件> <补丁文件>            - 生成补丁文件")
	fmt.Println("  hexdiff apply <源文件> <补丁文件> <目标文件>          - 应用补丁")
	fmt.Println("  hexdiff validate <补丁文件> [源文件]                 - 验证补丁文件")
	fmt.Println("  hexdiff info <补丁文件>                             - 显示补丁信息")
	fmt.Println("  hexdiff verify <文件路径>                           - 验证文件完整性")
	fmt.Println("  hexdiff backup <文件路径>                           - 创建文件备份")
	fmt.Println("  hexdiff recover <文件路径>                          - 恢复文件")
	fmt.Println("  hexdiff test-integrity                              - 测试完整性系统")
	fmt.Println("  hexdiff help                                        - 显示帮助信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  hexdiff signature old_file.bin")
	fmt.Println("  hexdiff diff old_file.bin new_file.bin")
	fmt.Println("  hexdiff patch old_file.bin new_file.bin update.patch")
	fmt.Println("  hexdiff apply old_file.bin update.patch new_file.bin")
	fmt.Println("  hexdiff validate update.patch old_file.bin")
	fmt.Println("  hexdiff info update.patch")
	fmt.Println("  hexdiff verify myfile.bin")
	fmt.Println("  hexdiff backup important.dat")
	fmt.Println("  hexdiff recover important.dat")
	fmt.Println("  hexdiff test-integrity")
}

func handleSignature() {
	if len(os.Args) < 3 {
		fmt.Println("错误: 请指定文件路径")
		fmt.Println("用法: hexdiff signature <文件路径>")
		return
	}

	filePath := os.Args[2]

	// 创建差异检测引擎
	engine, err := diff.NewEngine(nil) // 使用默认配置
	if err != nil {
		log.Fatalf("创建引擎失败: %v", err)
	}

	// 生成文件签名
	fmt.Printf("正在为文件 '%s' 生成签名...\n", filePath)
	signature, err := engine.GenerateSignature(filePath)
	if err != nil {
		log.Fatalf("生成签名失败: %v", err)
	}

	// 显示签名信息
	fmt.Printf("签名生成完成!\n")
	fmt.Printf("文件大小: %d 字节\n", signature.FileSize)
	fmt.Printf("块大小: %d 字节\n", signature.BlockSize)
	fmt.Printf("块数量: %d\n", getTotalBlocks(signature))
	fmt.Printf("SHA-256: %x\n", signature.Checksum)
}

func handleDiff() {
	if len(os.Args) < 4 {
		fmt.Println("错误: 请指定旧文件和新文件路径")
		fmt.Println("用法: hexdiff diff <旧文件> <新文件>")
		return
	}

	oldFile := os.Args[2]
	newFile := os.Args[3]

	// 创建差异检测引擎
	engine, err := diff.NewEngine(nil) // 使用默认配置
	if err != nil {
		log.Fatalf("创建引擎失败: %v", err)
	}

	// 生成差异
	fmt.Printf("正在生成 '%s' 和 '%s' 之间的差异...\n", oldFile, newFile)
	delta, err := engine.GenerateDelta(oldFile, newFile)
	if err != nil {
		log.Fatalf("生成差异失败: %v", err)
	}

	// 显示差异信息
	fmt.Printf("差异生成完成!\n")
	fmt.Printf("源文件大小: %d 字节\n", delta.SourceSize)
	fmt.Printf("目标文件大小: %d 字节\n", delta.TargetSize)
	fmt.Printf("操作数量: %d\n", len(delta.Operations))
	fmt.Printf("目标文件SHA-256: %x\n", delta.Checksum)

	// 统计操作类型
	var copyOps, insertOps, deleteOps int
	var totalInsertSize, totalCopySize int64

	for _, op := range delta.Operations {
		switch op.Type {
		case diff.OpCopy:
			copyOps++
			totalCopySize += int64(op.Size)
		case diff.OpInsert:
			insertOps++
			totalInsertSize += int64(op.Size)
		case diff.OpDelete:
			deleteOps++
		}
	}

	fmt.Printf("\n操作统计:\n")
	fmt.Printf("  复制操作: %d (总大小: %d 字节)\n", copyOps, totalCopySize)
	fmt.Printf("  插入操作: %d (总大小: %d 字节)\n", insertOps, totalInsertSize)
	fmt.Printf("  删除操作: %d\n", deleteOps)

	// 计算压缩比
	if delta.TargetSize > 0 {
		compressionRatio := float64(totalInsertSize) / float64(delta.TargetSize) * 100
		fmt.Printf("  补丁效率: %.2f%% (需要传输的新数据比例)\n", compressionRatio)
	}
}

func handlePatch() {
	if len(os.Args) < 5 {
		fmt.Println("错误: 请指定旧文件、新文件和补丁文件路径")
		fmt.Println("用法: hexdiff patch <旧文件> <新文件> <补丁文件>")
		return
	}

	oldFile := os.Args[2]
	newFile := os.Args[3]
	patchFile := os.Args[4]

	// 检查输入文件是否存在
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		log.Fatalf("旧文件不存在: %s", oldFile)
	}
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		log.Fatalf("新文件不存在: %s", newFile)
	}

	// 创建差异检测引擎
	engine, err := diff.NewEngine(nil) // 使用默认配置
	if err != nil {
		log.Fatalf("创建引擎失败: %v", err)
	}

	// 检查文件大小，决定使用哪种补丁生成器
	oldStat, err := os.Stat(oldFile)
	if err != nil {
		log.Fatalf("获取旧文件信息失败: %v", err)
	}

	newStat, err := os.Stat(newFile)
	if err != nil {
		log.Fatalf("获取新文件信息失败: %v", err)
	}

	fileSize := oldStat.Size()
	if newStat.Size() > fileSize {
		fileSize = newStat.Size()
	}

	// 如果文件大于500MB，使用流式补丁生成器
	const largeFileThreshold = 500 * 1024 * 1024 // 500MB
	var patchInfo *patch.PatchInfo

	fmt.Printf("正在生成补丁文件 '%s'...\n", patchFile)
	fmt.Printf("源文件: %s (%.2f GB)\n", oldFile, float64(oldStat.Size())/(1024*1024*1024))
	fmt.Printf("目标文件: %s (%.2f GB)\n", newFile, float64(newStat.Size())/(1024*1024*1024))

	if fileSize > largeFileThreshold {
		fmt.Println("检测到大文件，使用流式补丁生成器...")
		streamingGenerator := patch.NewStreamingPatchGenerator(engine, patch.CompressionNone)
		patchInfo, err = streamingGenerator.GeneratePatchStreaming(oldFile, newFile, patchFile)
	} else {
		generator := patch.NewGenerator(engine, patch.CompressionNone)
		patchInfo, err = generator.GeneratePatch(oldFile, newFile, patchFile)
	}

	if err != nil {
		log.Fatalf("生成补丁失败: %v", err)
	}

	// 显示补丁信息
	fmt.Println("\n补丁生成成功! ✅")
	fmt.Println(patchInfo.String())
}

func handleApply() {
	if len(os.Args) < 5 {
		fmt.Println("错误: 请指定源文件、补丁文件和目标文件路径")
		fmt.Println("用法: hexdiff apply <源文件> <补丁文件> <目标文件>")
		return
	}

	sourceFile := os.Args[2]
	patchFile := os.Args[3]
	targetFile := os.Args[4]

	// 检查输入文件是否存在
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		log.Fatalf("源文件不存在: %s", sourceFile)
	}
	if _, err := os.Stat(patchFile); os.IsNotExist(err) {
		log.Fatalf("补丁文件不存在: %s", patchFile)
	}

	// 创建补丁应用器
	applier := patch.NewApplier(nil) // 使用默认配置

	fmt.Printf("正在应用补丁...\n")
	fmt.Printf("源文件: %s\n", sourceFile)
	fmt.Printf("补丁文件: %s\n", patchFile)
	fmt.Printf("目标文件: %s\n", targetFile)

	// 应用补丁
	result, err := applier.ApplyPatch(sourceFile, patchFile, targetFile)
	if err != nil {
		log.Fatalf("应用补丁失败: %v", err)
	}

	// 显示结果
	fmt.Println("\n补丁应用完成! ✅")
	fmt.Println(result.String())
}

func handleValidate() {
	if len(os.Args) < 3 {
		fmt.Println("错误: 请指定补丁文件路径")
		fmt.Println("用法: hexdiff validate <补丁文件> [源文件]")
		return
	}

	patchFile := os.Args[2]
	var sourceFile string
	if len(os.Args) >= 4 {
		sourceFile = os.Args[3]
	}

	// 创建验证器
	validator := patch.NewValidator()

	// 验证补丁文件
	fmt.Printf("正在验证补丁文件: %s\n", patchFile)
	result, err := validator.ValidatePatchFile(patchFile)
	if err != nil {
		log.Fatalf("验证失败: %v", err)
	}

	fmt.Println(result.String())

	// 如果提供了源文件，验证兼容性
	if sourceFile != "" {
		fmt.Printf("\n正在验证源文件兼容性: %s\n", sourceFile)
		compatResult, err := validator.ValidateSourceFile(sourceFile, patchFile)
		if err != nil {
			log.Fatalf("兼容性验证失败: %v", err)
		}
		fmt.Println(compatResult.String())
	}
}

func handleInfo() {
	if len(os.Args) < 3 {
		fmt.Println("错误: 请指定补丁文件路径")
		fmt.Println("用法: hexdiff info <补丁文件>")
		return
	}

	patchFile := os.Args[2]

	// 检查补丁文件是否存在
	if _, err := os.Stat(patchFile); os.IsNotExist(err) {
		log.Fatalf("补丁文件不存在: %s", patchFile)
	}

	// 读取补丁信息
	header, err := patch.GetPatchInfo(patchFile)
	if err != nil {
		log.Fatalf("读取补丁信息失败: %v", err)
	}

	// 获取文件大小
	stat, err := os.Stat(patchFile)
	if err != nil {
		log.Fatalf("获取文件信息失败: %v", err)
	}

	// 显示详细信息
	fmt.Printf("补丁文件信息: %s\n", patchFile)
	fmt.Printf("==========================================\n")
	fmt.Printf("文件格式版本: %d\n", header.Version)
	fmt.Printf("压缩类型: %s\n", header.Compression.String())
	fmt.Printf("创建时间: %d\n", header.Timestamp)
	fmt.Printf("源文件大小: %d 字节\n", header.SourceSize)
	fmt.Printf("目标文件大小: %d 字节\n", header.TargetSize)
	fmt.Printf("补丁文件大小: %d 字节\n", stat.Size())
	fmt.Printf("操作数量: %d\n", header.OperationCount)
	fmt.Printf("压缩比: %.2f%%\n", float64(stat.Size())/float64(header.TargetSize)*100)
	fmt.Printf("大小减少: %.2f%%\n", float64(header.TargetSize-stat.Size())/float64(header.TargetSize)*100)
	fmt.Printf("源文件SHA-256: %x\n", header.SourceChecksum)
	fmt.Printf("目标文件SHA-256: %x\n", header.TargetChecksum)
}

// getTotalBlocks 计算签名中的总块数
func getTotalBlocks(signature *diff.Signature) int {
	total := 0
	for _, blocks := range signature.Blocks {
		total += len(blocks)
	}
	return total
}

// handleVerify 处理文件完整性验证
func handleVerify() {
	if len(os.Args) < 3 {
		fmt.Println("错误: 请指定文件路径")
		fmt.Println("用法: hexdiff verify <文件路径>")
		return
	}

	filePath := os.Args[2]

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("文件不存在: %s", filePath)
	}

	// 创建完整性检查器
	checker := integrity.NewIntegrityChecker(integrity.DefaultCheckerConfig())

	fmt.Printf("正在验证文件完整性: %s\n", filePath)

	// 生成文件校验和
	if err := checker.GenerateFileChecksums(filePath); err != nil {
		log.Fatalf("生成文件校验和失败: %v", err)
	}

	// 验证文件
	result, err := checker.VerifyFile(filePath)
	if err != nil {
		log.Fatalf("验证文件失败: %v", err)
	}

	// 显示结果
	fmt.Println("\n文件完整性验证完成! ✅")
	fmt.Println(result.String())
}

// handleBackup 处理文件备份
func handleBackup() {
	if len(os.Args) < 3 {
		fmt.Println("错误: 请指定文件路径")
		fmt.Println("用法: hexdiff backup <文件路径>")
		return
	}

	filePath := os.Args[2]

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("文件不存在: %s", filePath)
	}

	// 创建完整性检查器和恢复管理器
	checker := integrity.NewIntegrityChecker(integrity.DefaultCheckerConfig())
	recoveryManager := integrity.NewRecoveryManager(checker, integrity.DefaultRecoveryConfig())

	fmt.Printf("正在创建文件备份: %s\n", filePath)

	// 创建备份
	backupPath, err := recoveryManager.CreateBackup(filePath)
	if err != nil {
		log.Fatalf("创建备份失败: %v", err)
	}

	fmt.Printf("备份创建成功! ✅\n")
	fmt.Printf("备份文件: %s\n", backupPath)

	// 获取备份信息
	backupInfo, err := recoveryManager.GetBackupInfo()
	if err != nil {
		log.Fatalf("获取备份信息失败: %v", err)
	}

	fmt.Println(backupInfo.String())
}

// handleRecover 处理文件恢复
func handleRecover() {
	if len(os.Args) < 3 {
		fmt.Println("错误: 请指定文件路径")
		fmt.Println("用法: hexdiff recover <文件路径>")
		return
	}

	filePath := os.Args[2]

	// 创建完整性检查器和恢复管理器
	checker := integrity.NewIntegrityChecker(integrity.DefaultCheckerConfig())
	recoveryManager := integrity.NewRecoveryManager(checker, integrity.DefaultRecoveryConfig())

	fmt.Printf("正在尝试恢复文件: %s\n", filePath)

	// 尝试自动恢复
	if err := recoveryManager.AutoRecover(filePath); err != nil {
		log.Fatalf("文件恢复失败: %v", err)
	}

	fmt.Printf("文件恢复成功! ✅\n")

	// 验证恢复后的文件
	if err := checker.GenerateFileChecksums(filePath); err != nil {
		log.Printf("警告: 生成恢复文件校验和失败: %v", err)
		return
	}

	result, err := checker.VerifyFile(filePath)
	if err != nil {
		log.Printf("警告: 验证恢复文件失败: %v", err)
		return
	}

	if result.Success {
		fmt.Println("恢复文件完整性验证通过! ✅")
	} else {
		fmt.Printf("警告: 恢复文件完整性验证失败，失败块数: %d\n", result.FailedBlocks)
	}
}

// handleTestIntegrity 处理完整性系统测试
func handleTestIntegrity() {
	fmt.Println("开始完整性校验系统测试...")

	// 运行完整性系统测试
	if err := testIntegritySystem(); err != nil {
		log.Fatalf("完整性系统测试失败: %v", err)
	}

	fmt.Println("完整性校验系统测试全部通过! 🎉")
}

// testIntegritySystem 完整性校验系统测试（简化版本）
func testIntegritySystem() error {
	fmt.Println("=== 完整性校验系统测试 ===")

	// 创建测试目录
	testDir := "test_integrity"
	if err := os.MkdirAll(testDir, 0755); err != nil {
		return fmt.Errorf("创建测试目录失败: %w", err)
	}
	defer os.RemoveAll(testDir)

	// 创建测试文件
	testFile := testDir + "/test.txt"
	testData := []byte("这是一个用于测试完整性校验系统的示例文件。\n包含多行数据用于验证块级校验和功能。\n")

	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		return fmt.Errorf("创建测试文件失败: %w", err)
	}

	// 测试基本完整性检查器
	fmt.Println("\n1. 测试基本完整性检查器...")
	checker := integrity.NewIntegrityChecker(integrity.DefaultCheckerConfig())

	if err := checker.GenerateFileChecksums(testFile); err != nil {
		return fmt.Errorf("生成文件校验和失败: %w", err)
	}

	result, err := checker.VerifyFile(testFile)
	if err != nil {
		return fmt.Errorf("验证文件失败: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("文件验证失败")
	}

	fmt.Printf("✅ 基本完整性检查器测试通过 (验证了 %d 个块)\n", result.VerifiedBlocks)

	// 测试恢复管理器
	fmt.Println("\n2. 测试恢复管理器...")
	recoveryManager := integrity.NewRecoveryManager(checker, integrity.DefaultRecoveryConfig())

	backupPath, err := recoveryManager.CreateBackup(testFile)
	if err != nil {
		return fmt.Errorf("创建备份失败: %w", err)
	}

	fmt.Printf("✅ 恢复管理器测试通过 (备份文件: %s)\n", backupPath)

	// 测试增强应用器
	fmt.Println("\n3. 测试增强应用器...")
	applier := integrity.NewEnhancedApplier(integrity.DefaultEnhancedApplierConfig())

	targetFile := testDir + "/target.txt"
	applyResult, err := applier.ApplyPatchWithIntegrity(testFile, "", targetFile, nil)
	if err != nil {
		return fmt.Errorf("应用补丁失败: %w", err)
	}

	if !applyResult.Success {
		return fmt.Errorf("补丁应用失败")
	}

	fmt.Printf("✅ 增强应用器测试通过 (处理了 %d 字节)\n", applyResult.BytesProcessed)

	return nil
}
