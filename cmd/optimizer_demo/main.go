package main

import (
	"fmt"
	"os"

	"github.com/Sky-ey/HexDiff/pkg/diff"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("使用方法: optimizer_demo <旧文件> <新文件>")
		os.Exit(1)
	}

	oldFilePath := os.Args[1]
	newFilePath := os.Args[2]

	// 创建差异检测引擎
	config := diff.DefaultDiffConfig()
	config.BlockSize = 4096
	engine, err := diff.NewEngine(config)
	if err != nil {
		fmt.Printf("创建引擎失败: %v\n", err)
		os.Exit(1)
	}

	// 生成差异（自动应用优化器）
	delta, err := engine.GenerateDelta(oldFilePath, newFilePath)
	if err != nil {
		fmt.Printf("生成差异失败: %v\n", err)
		os.Exit(1)
	}

	// 统计操作类型
	copyCount := 0
	insertCount := 0
	deleteCount := 0
	totalSize := 0

	for _, op := range delta.Operations {
		totalSize += op.Size
		switch op.Type {
		case diff.OpCopy:
			copyCount++
		case diff.OpInsert:
			insertCount++
		case diff.OpDelete:
			deleteCount++
		}
	}

	fmt.Println("=== 差异优化结果 ===")
	fmt.Printf("文件信息:\n")
	fmt.Printf("  旧文件: %s (%d 字节)\n", oldFilePath, delta.SourceSize)
	fmt.Printf("  新文件: %s (%d 字节)\n", newFilePath, delta.TargetSize)
	fmt.Printf("\n操作统计:\n")
	fmt.Printf("  总操作数: %d\n", len(delta.Operations))
	fmt.Printf("  Copy 操作: %d\n", copyCount)
	fmt.Printf("  Insert 操作: %d\n", insertCount)
	fmt.Printf("  Delete 操作: %d\n", deleteCount)
	fmt.Printf("\n优化效果:\n")
	fmt.Printf("  操作总数已优化合并\n")
	fmt.Printf("  补丁大小估算: %d 字节 (操作元数据: %d 字节, 数据: %d 字节)\n",
		len(delta.Operations)*26+totalSize, len(delta.Operations)*26, totalSize)

	// 计算压缩比
	compressionRatio := float64(len(delta.Operations)*26+totalSize) / float64(delta.TargetSize) * 100
	fmt.Printf("  压缩比: %.2f%% (相对于新文件)\n", compressionRatio)

	// 显示前 10 个操作（如果有的话）
	fmt.Printf("\n前 10 个操作:\n")
	maxOps := 10
	if len(delta.Operations) < maxOps {
		maxOps = len(delta.Operations)
	}

	for i := 0; i < maxOps; i++ {
		op := delta.Operations[i]
		var details string
		switch op.Type {
		case diff.OpCopy:
			details = fmt.Sprintf("SrcOffset=%d", op.SrcOffset)
		case diff.OpInsert:
			details = fmt.Sprintf("DataLen=%d", len(op.Data))
		case diff.OpDelete:
			details = ""
		}
		fmt.Printf("  [%d] %s Offset=%d Size=%d %s\n",
			i+1, op.Type.String(), op.Offset, op.Size, details)
	}
}
