package diff

import (
	"sort"
)

type Optimizer struct {
	config *OptimizerConfig
}

type OptimizerConfig struct {
	EnableMergeCopy   bool // 启用合并连续的Copy操作
	EnableMergeInsert bool // 启用合并连续的Insert操作
	EnableMergeDelete bool // 启用合并连续的Delete操作
	MinMergedSize     int  // 最小合并大小（字节）
}

func DefaultOptimizerConfig() *OptimizerConfig {
	return &OptimizerConfig{
		EnableMergeCopy:   true,
		EnableMergeInsert: true,
		EnableMergeDelete: true,
		MinMergedSize:     1024, // 1KB
	}
}

func NewOptimizer(config *OptimizerConfig) *Optimizer {
	if config == nil {
		config = DefaultOptimizerConfig()
	}
	return &Optimizer{config: config}
}

func (o *Optimizer) OptimizeDelta(delta *Delta) *Delta {
	if delta == nil || len(delta.Operations) == 0 {
		return delta
	}

	// 第一阶段：移除空操作
	o.RemoveEmptyOperations(delta)

	optimized := &Delta{
		Operations: make([]Operation, 0, len(delta.Operations)),
		SourceSize: delta.SourceSize,
		TargetSize: delta.TargetSize,
		Checksum:   delta.Checksum,
	}

	i := 0
	for i < len(delta.Operations) {
		op := delta.Operations[i]

		switch op.Type {
		case OpCopy:
			if o.config.EnableMergeCopy {
				merged, nextIdx := o.mergeCopyOperations(delta.Operations, i)
				optimized.AddOperation(merged)
				i = nextIdx
			} else {
				optimized.AddOperation(op)
				i++
			}

		case OpInsert:
			if o.config.EnableMergeInsert {
				merged, nextIdx := o.mergeInsertOperations(delta.Operations, i)
				optimized.AddOperation(merged)
				i = nextIdx
			} else {
				optimized.AddOperation(op)
				i++
			}

		case OpDelete:
			if o.config.EnableMergeDelete {
				merged, nextIdx := o.mergeDeleteOperations(delta.Operations, i)
				optimized.AddOperation(merged)
				i = nextIdx
			} else {
				optimized.AddOperation(op)
				i++
			}

		default:
			optimized.AddOperation(op)
			i++
		}
	}

	// 执行第二阶段优化：移除冗余的 Delete 操作
	o.optimizeRedundantDeletes(optimized)

	return optimized
}

func (o *Optimizer) optimizeRedundantDeletes(delta *Delta) {
	filtered := make([]Operation, 0, len(delta.Operations))

	for i := 0; i < len(delta.Operations); i++ {
		op := delta.Operations[i]

		// 检查是否有相邻的 Delete 操作
		if op.Type == OpDelete {
			// 检查后面是否有 Copy 操作，如果有，这个 Delete 可能是多余的
			if i+1 < len(delta.Operations) && delta.Operations[i+1].Type == OpCopy {
				copyOp := delta.Operations[i+1]
				// 如果 Delete 和 Copy 的位置相同，说明删除后立即覆盖，Delete 操作可以移除
				if op.Offset == copyOp.Offset && op.Size <= copyOp.Size {
					continue
				}
			}
		}

		filtered = append(filtered, op)
	}

	delta.Operations = filtered
}

func (o *Optimizer) mergeCopyOperations(operations []Operation, startIdx int) (Operation, int) {
	mergedOp := operations[startIdx]
	i := startIdx + 1

	for i < len(operations) && operations[i].Type == OpCopy {
		currentOp := operations[i]
		prevOp := operations[i-1]

		// 检查目标位置是否连续
		targetContinuous := prevOp.Offset+int64(prevOp.Size) == currentOp.Offset
		// 检查源位置是否连续
		sourceContinuous := prevOp.SrcOffset+int64(prevOp.Size) == currentOp.SrcOffset

		if targetContinuous && sourceContinuous {
			mergedOp.Size += currentOp.Size
			i++
		} else {
			break
		}
	}

	return mergedOp, i
}

func (o *Optimizer) mergeInsertOperations(operations []Operation, startIdx int) (Operation, int) {
	mergedOp := operations[startIdx]
	mergedData := make([]byte, 0, mergedOp.Size)
	mergedData = append(mergedData, mergedOp.Data...)
	i := startIdx + 1

	for i < len(operations) && operations[i].Type == OpInsert {
		currentOp := operations[i]
		prevOp := operations[i-1]

		if prevOp.Offset+int64(prevOp.Size) == currentOp.Offset {
			mergedOp.Size += currentOp.Size
			mergedData = append(mergedData, currentOp.Data...)
			i++
		} else {
			break
		}
	}

	mergedOp.Data = mergedData
	return mergedOp, i
}

func (o *Optimizer) mergeDeleteOperations(operations []Operation, startIdx int) (Operation, int) {
	mergedOp := operations[startIdx]
	i := startIdx + 1

	for i < len(operations) && operations[i].Type == OpDelete {
		currentOp := operations[i]
		prevOp := operations[i-1]

		if prevOp.Offset+int64(prevOp.Size) == currentOp.Offset {
			mergedOp.Size += currentOp.Size
			i++
		} else {
			break
		}
	}

	return mergedOp, i
}

func (o *Optimizer) SortOperations(delta *Delta) {
	sort.Slice(delta.Operations, func(i, j int) bool {
		if delta.Operations[i].Offset != delta.Operations[j].Offset {
			return delta.Operations[i].Offset < delta.Operations[j].Offset
		}
		return delta.Operations[i].Type < delta.Operations[j].Type
	})
}

func (o *Optimizer) RemoveEmptyOperations(delta *Delta) {
	filtered := make([]Operation, 0, len(delta.Operations))

	for _, op := range delta.Operations {
		if op.Size > 0 {
			filtered = append(filtered, op)
		}
	}

	delta.Operations = filtered
}
