package main

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/Sky-ey/HexDiff/pkg/patch"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: hexdiff-dbg <补丁文件>")
		return
	}

	patchFile := os.Args[1]

	file, err := os.Open(patchFile)
	if err != nil {
		fmt.Printf("打开文件失败: %v\n", err)
		return
	}
	defer file.Close()

	// 读取文件头
	headerData := make([]byte, patch.HeaderSize)
	if _, err := file.Read(headerData); err != nil {
		fmt.Printf("读取文件头失败: %v\n", err)
		return
	}

	magic := binary.LittleEndian.Uint32(headerData[0:4])
	version := binary.LittleEndian.Uint16(headerData[4:6])
	compression := headerData[6]
	sourceSize := binary.LittleEndian.Uint64(headerData[16:24])
	targetSize := binary.LittleEndian.Uint64(headerData[24:32])
	operationCount := binary.LittleEndian.Uint32(headerData[96:100])

	fmt.Printf("补丁文件信息:\n")
	fmt.Printf("  魔数: 0x%x\n", magic)
	fmt.Printf("  版本: %d\n", version)
	fmt.Printf("  压缩类型: %d\n", compression)
	fmt.Printf("  源文件大小: %d\n", sourceSize)
	fmt.Printf("  目标文件大小: %d\n", targetSize)
	fmt.Printf("  操作数量: %d\n", operationCount)
	fmt.Printf("\n")

	// 读取操作列表
	fmt.Println("操作列表:")
	fmt.Println("序号 | 类型   | 偏移量       | 大小    | 源偏移量     | 数据偏移")
	fmt.Println("-----|--------|--------------|---------|--------------|---------")

	for i := uint32(0); i < operationCount; i++ {
		opData := make([]byte, patch.OperationSize)
		if _, err := file.Read(opData); err != nil {
			fmt.Printf("读取操作 %d 失败: %v\n", i, err)
			return
		}

		opType := opData[0]
		size := binary.LittleEndian.Uint32(opData[2:6])
		offset := binary.LittleEndian.Uint64(opData[6:14])
		srcOffset := binary.LittleEndian.Uint64(opData[14:22])
		dataOffset := binary.LittleEndian.Uint32(opData[22:26])

		typeStr := "???"
		switch opType {
		case 0:
			typeStr = "COPY"
		case 1:
			typeStr = "INSERT"
		case 2:
			typeStr = "DELETE"
		}

		fmt.Printf("%4d | %-6s | 0x%012x | %-7d | 0x%012x | 0x%08x\n",
			i, typeStr, offset, size, srcOffset, dataOffset)
	}
}
