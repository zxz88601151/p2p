// logring.go 为 Developer Console 提供「真实日志」来源。
//
// 为什么需要它：控制台页面运行在浏览器里，无法读取节点进程的 stdout，
// 因此必须由节点侧把最近的日志经控制接口暴露出来（GET /logs）。
//
// 设计原则（本阶段规格 §19 / §20）：
//   - 不改变既有日志行为：仍原样写 stderr，只是额外复制一份进环形缓冲；
//   - 不改写日志原文：Component 与 Message 均为原文；
//   - 等级是**推断值**：本项目日志用标准库 log，原生不带等级，
//     因此 Level 由组件与关键词推断，仅用于界面分组着色，不是权威判定。
package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"p2pchain/internal/control"
)

// consoleLogCapacity 环形缓冲保留的日志行数上限（防止长期运行无界增长）。
const consoleLogCapacity = 600

// logRing 是一个定长的日志环形缓冲，同时实现 io.Writer 与 control.LogProvider。
type logRing struct {
	mu    sync.Mutex
	lines []control.LogEntry
	max   int
	buf   []byte // 未凑满一行的残留字节
}

func newLogRing(max int) *logRing {
	if max <= 0 {
		max = consoleLogCapacity
	}
	return &logRing{max: max, lines: make([]control.LogEntry, 0, max)}
}

// Write 实现 io.Writer：按行切分并解析。返回值恒为 len(p)，
// 保证 MultiWriter 不会因为本缓冲的问题影响 stderr 输出。
func (r *logRing) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.buf = append(r.buf, p...)
	for {
		i := bytes.IndexByte(r.buf, '\n')
		if i < 0 {
			break
		}
		line := string(r.buf[:i])
		r.buf = r.buf[i+1:]
		if strings.TrimSpace(line) == "" {
			continue
		}
		r.push(parseLogLine(line))
	}
	// 防御：出现超长且始终不带换行的一行时，丢弃残留，避免内存无界增长。
	if len(r.buf) > 64*1024 {
		r.buf = r.buf[:0]
	}
	return len(p), nil
}

func (r *logRing) push(e control.LogEntry) {
	if len(r.lines) >= r.max {
		copy(r.lines, r.lines[1:])
		r.lines = r.lines[:len(r.lines)-1]
	}
	r.lines = append(r.lines, e)
}

// RecentLogs 返回最近至多 n 条日志，按时间从旧到新排列。
func (r *logRing) RecentLogs(n int) []control.LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || n > len(r.lines) {
		n = len(r.lines)
	}
	out := make([]control.LogEntry, n)
	copy(out, r.lines[len(r.lines)-n:])
	return out
}

// 编译期断言：环形缓冲必须满足控制接口的日志提供者契约。
var _ control.LogProvider = (*logRing)(nil)

// parseLogLine 解析标准库 log 的默认输出格式：
//
//	2006/01/02 15:04:05 [component] message
//
// 解析失败时保留原文，绝不丢日志。
func parseLogLine(s string) control.LogEntry {
	s = strings.TrimRight(s, "\r\n")
	e := control.LogEntry{}
	rest := s

	// 时间前缀：Go 默认 LstdFlags 输出为 "YYYY/MM/DD HH:MM:SS "（20 字节）
	if len(rest) >= 20 && rest[4] == '/' && rest[7] == '/' && rest[10] == ' ' && rest[13] == ':' {
		e.Time = rest[11:19]
		rest = strings.TrimSpace(rest[20:])
	}

	// 组件前缀：[node] / [p2p] / [miner] / [ui]
	if strings.HasPrefix(rest, "[") {
		if i := strings.IndexByte(rest, ']'); i > 1 {
			e.Component = rest[1:i]
			rest = strings.TrimSpace(rest[i+1:])
		}
	}

	e.Message = rest
	e.Level = inferLevel(e.Message)
	return e
}

// 推断等级使用的关键词集合（保守、可读、可复核）。
var (
	levelErrWords  = []string{"失败", "错误", "error", "ERROR", "panic", "fatal", "拒绝", "非法", "无法", "占用"}
	levelWarnWords = []string{"警告", "warn", "WARN", "上限", "放弃", "作废", "未启用", "重试"}
)

// inferLevel 由日志正文推断等级（原生日志无等级字段，故只看正文，不猜组件）。
func inferLevel(message string) string {
	for _, w := range levelErrWords {
		if strings.Contains(message, w) {
			return "ERROR"
		}
	}
	for _, w := range levelWarnWords {
		if strings.Contains(message, w) {
			return "WARN"
		}
	}
	return "INFO"
}

// installLogRing 把标准库日志同时写到 stderr 与环形缓冲。
//
// 这样终端输出与（日志内容、顺序、格式）控制台看到的完全一致，
// 不改变任何既有日志行为——只是多了一路观察者。
func installLogRing() *logRing {
	ring := newLogRing(consoleLogCapacity)
	log.SetOutput(io.MultiWriter(os.Stderr, ring))
	return ring
}
