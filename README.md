# HexDiff 一个二进制补丁生成工具

## 核心功能

- 基于二进制的快速补丁生成，支持Cli工具和软件包调用

- 采用SHA-256和CRC32双重文件完整性验证，自动备份恢复

- 支持单文件、文件夹双模式

- 支持并发生成，性能针对大文件优化

- 多压缩算法支持，包括LZ4、Zstd、Gzip等

- 支持自定义进度显示


## 技术栈

-  Go 1.25

## 基础用法

### 安装

```shell

go get github.com/Sky-ey/HexDiff

```

### 基础使用

```go
import hexdiff "github.com/Sky-ey/HexDiff"

// 生成单文件补丁
err := hexdiff.Diff("old.txt", "new.txt", "diff.patch")

// 生成文件夹补丁
err := hexdiff.DiffDir("old_dir", "new_dir", "diff.patch")

// 应用补丁（通用）
err := hexdiff.Apply("diff.patch", "old.txt", "restored.txt")
```

### 进阶用法

```go
import hexdiff "github.com/Sky-ey/HexDiff"

patch, err := hexdiff.New().
	// 设置块大小
	WithBlockSize(8192).
	// 设置压缩算法
	WithCompression(hexdiff.CompressionLZ4).
	// 设置进度条
	WithProgress(func(current, total int64, message string) {}).
	// 生成补丁
	Diff("old.txt", "new.txt", "diff.patch")
```