package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ProgressReporter 进度报告接口
type ProgressReporter interface {
	SetTotal(total int64)
	SetCurrent(current int64)
	Increment(delta int64)
	SetMessage(message string)
	Finish()
	IsFinished() bool
}

// ProgressManager 进度管理器
type ProgressManager struct {
	enabled bool
	output  io.Writer
	tasks   map[string]*ProgressTask
	mutex   sync.RWMutex
}

// NewProgressManager 创建进度管理器
func NewProgressManager(enabled bool) *ProgressManager {
	return &ProgressManager{
		enabled: enabled,
		output:  os.Stdout,
		tasks:   make(map[string]*ProgressTask),
	}
}

// NewTask 创建新的进度任务
func (pm *ProgressManager) NewTask(name string, total int64) ProgressReporter {
	if !pm.enabled {
		return &NoOpProgress{}
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	task := &ProgressTask{
		name:      name,
		total:     total,
		current:   0,
		startTime: time.Now(),
		output:    pm.output,
		finished:  false,
	}

	pm.tasks[name] = task
	task.render()
	return task
}

// RemoveTask 移除进度任务
func (pm *ProgressManager) RemoveTask(name string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	delete(pm.tasks, name)
}

// SetOutput 设置输出流
func (pm *ProgressManager) SetOutput(output io.Writer) {
	pm.output = output
}

// ProgressTask 进度任务
type ProgressTask struct {
	name      string
	total     int64
	current   int64
	message   string
	startTime time.Time
	output    io.Writer
	finished  bool
	mutex     sync.RWMutex
}

// SetTotal 设置总量
func (pt *ProgressTask) SetTotal(total int64) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()
	pt.total = total
	pt.render()
}

// SetCurrent 设置当前值
func (pt *ProgressTask) SetCurrent(current int64) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()
	pt.current = current
	pt.render()
}

// Increment 增加进度
func (pt *ProgressTask) Increment(delta int64) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()
	pt.current += delta
	if pt.current > pt.total {
		pt.current = pt.total
	}
	pt.render()
}

// SetMessage 设置消息
func (pt *ProgressTask) SetMessage(message string) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()
	pt.message = message
	pt.render()
}

// Finish 完成进度
func (pt *ProgressTask) Finish() {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()
	pt.current = pt.total
	pt.finished = true
	pt.render()
	fmt.Fprintln(pt.output) // 换行
}

// IsFinished 检查是否完成
func (pt *ProgressTask) IsFinished() bool {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()
	return pt.finished
}

// render 渲染进度条
func (pt *ProgressTask) render() {
	if pt.finished {
		return
	}

	// 计算百分比
	var percentage float64
	if pt.total > 0 {
		percentage = float64(pt.current) / float64(pt.total) * 100
	}

	// 计算速度和剩余时间
	elapsed := time.Since(pt.startTime)
	var speed float64
	var eta time.Duration

	if elapsed.Seconds() > 0 && pt.current > 0 {
		speed = float64(pt.current) / elapsed.Seconds()
		if speed > 0 && pt.total > pt.current {
			eta = time.Duration(float64(pt.total-pt.current)/speed) * time.Second
		}
	}

	// 构建进度条
	barWidth := 40
	filled := int(float64(barWidth) * percentage / 100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// 格式化输出
	var output string
	if pt.message != "" {
		output = fmt.Sprintf("\r%s: [%s] %.1f%% (%s) %s",
			pt.name, bar, percentage, formatBytes(pt.current), pt.message)
	} else {
		output = fmt.Sprintf("\r%s: [%s] %.1f%% (%s/%s)",
			pt.name, bar, percentage, formatBytes(pt.current), formatBytes(pt.total))
	}

	// 添加速度和ETA信息
	if speed > 0 {
		output += fmt.Sprintf(" | %s/s", formatBytes(int64(speed)))
	}
	if eta > 0 {
		output += fmt.Sprintf(" | ETA: %s", formatDuration(eta))
	}

	fmt.Fprint(pt.output, output)
}

// NoOpProgress 空操作进度报告器
type NoOpProgress struct{}

func (nop *NoOpProgress) SetTotal(total int64)      {}
func (nop *NoOpProgress) SetCurrent(current int64)  {}
func (nop *NoOpProgress) Increment(delta int64)     {}
func (nop *NoOpProgress) SetMessage(message string) {}
func (nop *NoOpProgress) Finish()                   {}
func (nop *NoOpProgress) IsFinished() bool          { return true }

// MultiProgress 多进度条管理器
type MultiProgress struct {
	tasks  []*ProgressTask
	output io.Writer
	mutex  sync.RWMutex
}

// NewMultiProgress 创建多进度条管理器
func NewMultiProgress() *MultiProgress {
	return &MultiProgress{
		tasks:  make([]*ProgressTask, 0),
		output: os.Stdout,
	}
}

// AddTask 添加进度任务
func (mp *MultiProgress) AddTask(name string, total int64) ProgressReporter {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	task := &ProgressTask{
		name:      name,
		total:     total,
		current:   0,
		startTime: time.Now(),
		output:    mp.output,
		finished:  false,
	}

	mp.tasks = append(mp.tasks, task)
	mp.render()
	return task
}

// render 渲染所有进度条
func (mp *MultiProgress) render() {
	// 清屏并移动到顶部
	fmt.Fprint(mp.output, "\033[2J\033[H")

	for _, task := range mp.tasks {
		if !task.finished {
			task.render()
			fmt.Fprintln(mp.output)
		}
	}
}

// Spinner 旋转进度指示器
type Spinner struct {
	message string
	chars   []string
	index   int
	active  bool
	output  io.Writer
	ticker  *time.Ticker
	done    chan bool
	mutex   sync.RWMutex
}

// NewSpinner 创建旋转指示器
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		chars:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		output:  os.Stdout,
		done:    make(chan bool),
	}
}

// Start 启动旋转指示器
func (s *Spinner) Start() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.active {
		return
	}

	s.active = true
	s.ticker = time.NewTicker(100 * time.Millisecond)

	go func() {
		for {
			select {
			case <-s.ticker.C:
				s.mutex.Lock()
				if s.active {
					fmt.Fprintf(s.output, "\r%s %s", s.chars[s.index], s.message)
					s.index = (s.index + 1) % len(s.chars)
				}
				s.mutex.Unlock()
			case <-s.done:
				return
			}
		}
	}()
}

// Stop 停止旋转指示器
func (s *Spinner) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.active {
		return
	}

	s.active = false
	s.ticker.Stop()
	s.done <- true
	fmt.Fprint(s.output, "\r") // 清除当前行
}

// SetMessage 设置消息
func (s *Spinner) SetMessage(message string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.message = message
}

// 辅助函数

// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

// ProgressBar 简单进度条
type ProgressBar struct {
	total   int64
	current int64
	width   int
	prefix  string
	suffix  string
	output  io.Writer
}

// NewProgressBar 创建简单进度条
func NewProgressBar(total int64, width int) *ProgressBar {
	return &ProgressBar{
		total:  total,
		width:  width,
		output: os.Stdout,
	}
}

// SetPrefix 设置前缀
func (pb *ProgressBar) SetPrefix(prefix string) {
	pb.prefix = prefix
}

// SetSuffix 设置后缀
func (pb *ProgressBar) SetSuffix(suffix string) {
	pb.suffix = suffix
}

// Update 更新进度
func (pb *ProgressBar) Update(current int64) {
	pb.current = current
	pb.render()
}

// Increment 增加进度
func (pb *ProgressBar) Increment(delta int64) {
	pb.current += delta
	if pb.current > pb.total {
		pb.current = pb.total
	}
	pb.render()
}

// render 渲染进度条
func (pb *ProgressBar) render() {
	percentage := float64(pb.current) / float64(pb.total)
	filled := int(float64(pb.width) * percentage)

	bar := strings.Repeat("█", filled) + strings.Repeat("░", pb.width-filled)

	output := fmt.Sprintf("\r%s[%s] %.1f%% %s",
		pb.prefix, bar, percentage*100, pb.suffix)

	fmt.Fprint(pb.output, output)
}

// Finish 完成进度条
func (pb *ProgressBar) Finish() {
	pb.current = pb.total
	pb.render()
	fmt.Fprintln(pb.output)
}
