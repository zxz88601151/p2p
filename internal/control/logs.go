package control

import (
	"fmt"
	"net/http"
	"strconv"
)

// LogEntry 是结构化之后的一条节点运行日志。
//
// Level 说明（重要）：本项目的节点日志（cmd/node）使用标准库 log，输出形如
// 「2006/01/02 15:04:05 [node] …」，**原生不带等级**。因此 LogEntry.Level 由日志
// 来源侧按关键词推断而得（见 cmd/node 的 logring），仅用于界面分组与着色，
// 不代表协议或机器的权威判定。Component 与 Message 均为日志原文，不做改写。
type LogEntry struct {
	Time      string `json:"time"`
	Level     string `json:"level"` // INFO / WARN / ERROR（推断值）
	Component string `json:"component"`
	Message   string `json:"message"`
}

// LogProvider 由节点侧实现，提供最近的运行日志。
//
// 这是**可选**能力：之所以用独立接口而不是给 Node 增加方法，是为了让
// 既有实现（含测试替身）无需改动即可继续编译——未注入时 /logs 返回空数组，
// 而不是报错，也不是伪造日志。
type LogProvider interface {
	// RecentLogs 返回最近至多 n 条日志，按时间从旧到新排列。
	RecentLogs(n int) []LogEntry
}

// SetLogProvider 注入日志来源（可选）。传 nil 表示清除。
func (s *Server) SetLogProvider(p LogProvider) {
	s.mu.Lock()
	s.logs = p
	s.mu.Unlock()
}

// MaxLogTail 单次 /logs 请求最多返回的日志行数。
const MaxLogTail = 500

// defaultLogTail /logs 未指定 tail 时的默认行数。
const defaultLogTail = 100

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	tail := defaultLogTail
	if raw := r.URL.Query().Get("tail"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("tail 参数非法: %q", raw))
			return
		}
		if n > MaxLogTail {
			n = MaxLogTail
		}
		tail = n
	}

	s.mu.Lock()
	p := s.logs
	s.mu.Unlock()

	if p == nil {
		// 诚实空态：没有日志来源时返回空数组，而不是编造内容。
		writeJSON(w, http.StatusOK, []LogEntry{})
		return
	}
	entries := p.RecentLogs(tail)
	if entries == nil {
		entries = []LogEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}
