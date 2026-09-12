package main

// 本文件覆盖 Developer Console 的「真实日志来源」与难度展示量的换算。
//
//   logRing：日志按行切分 → 解析 → 定长环形缓冲，供 GET /logs 使用；
//   relativeDifficulty：把共识真值 bits 换算成界面展示的难度倍数。
//
// 这两处的共同底线：日志原文不得被改写（组件、正文保持原样），
// 难度倍数只用于展示、不得反过来影响共识。

import (
	"fmt"
	"log"
	"math"
	"math/big"
	"strings"
	"testing"

	"p2pchain/internal/control"
	"p2pchain/internal/pow"
)

// ---- 日志解析 ----

// TestParseLogLine 日志解析：时间/组件可缺省，正文必须原样保留。
func TestParseLogLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want control.LogEntry
	}{
		{
			name: "标准格式（时间+组件）",
			in:   "2006/01/02 15:04:05 [node] 节点启动完成",
			want: control.LogEntry{Time: "15:04:05", Level: "INFO", Component: "node", Message: "节点启动完成"},
		},
		{
			name: "无时间前缀",
			in:   "[miner] 挖出区块 高度=3",
			want: control.LogEntry{Level: "INFO", Component: "miner", Message: "挖出区块 高度=3"},
		},
		{
			name: "无组件前缀",
			in:   "运行结束",
			want: control.LogEntry{Level: "INFO", Message: "运行结束"},
		},
		{
			name: "正文含错误关键词",
			in:   "2006/01/02 15:04:05 [node] 控制接口监听失败: 端口占用",
			want: control.LogEntry{Time: "15:04:05", Level: "ERROR", Component: "node", Message: "控制接口监听失败: 端口占用"},
		},
		{
			name: "正文含警告关键词",
			in:   "[p2p] 连接被拒，稍后重试",
			want: control.LogEntry{Level: "WARN", Component: "p2p", Message: "连接被拒，稍后重试"},
		},
		{
			name: "Windows 行尾",
			in:   "2006/01/02 15:04:05 [ui] 已打开控制台\r",
			want: control.LogEntry{Time: "15:04:05", Level: "INFO", Component: "ui", Message: "已打开控制台"},
		},
		{
			name: "正文以方括号开头但不是组件（无闭合）",
			in:   "数组下标 [3 越界",
			want: control.LogEntry{Level: "INFO", Message: "数组下标 [3 越界"},
		},
	}

	for _, tc := range cases {
		got := parseLogLine(tc.in)
		if got != tc.want {
			t.Errorf("%s: parseLogLine(%q) = %+v, want %+v", tc.name, tc.in, got, tc.want)
		}
	}
}

// TestInferLevel 等级推断：只在正文里找关键词，找不到就是 INFO。
func TestInferLevel(t *testing.T) {
	cases := map[string]string{
		"节点启动完成":                "INFO",
		"区块校验失败":                "ERROR",
		"peer connection error": "ERROR",
		"goroutine panic":       "ERROR",
		"连接数已达上限":               "WARN",
		"候选区块已作废":               "WARN",
		"重试中":                   "WARN",
		"":                      "INFO",
	}
	for msg, want := range cases {
		if got := inferLevel(msg); got != want {
			t.Errorf("inferLevel(%q) = %q, want %q", msg, got, want)
		}
	}
}

// ---- 环形缓冲 ----

// TestLogRingKeepsNewestLines 容量满后淘汰最旧的，RecentLogs 保持旧→新顺序。
func TestLogRingKeepsNewestLines(t *testing.T) {
	r := newLogRing(3)
	for i := 1; i <= 5; i++ {
		if _, err := fmt.Fprintf(r, "2006/01/02 15:04:05 [node] 第 %d 行\n", i); err != nil {
			t.Fatalf("写入第 %d 行失败: %v", i, err)
		}
	}

	got := r.RecentLogs(10)
	if len(got) != 3 {
		t.Fatalf("容量 3 应只保留 3 条，实际 %d 条: %+v", len(got), got)
	}
	if got[0].Message != "第 3 行" || got[2].Message != "第 5 行" {
		t.Fatalf("保留的不是最新 3 条: %q ... %q", got[0].Message, got[2].Message)
	}
	if got[0].Time != "15:04:05" || got[0].Component != "node" {
		t.Fatalf("时间/组件解析异常: %+v", got[0])
	}

	// 只要最后 n 条
	if tail := r.RecentLogs(1); len(tail) != 1 || tail[0].Message != "第 5 行" {
		t.Fatalf("RecentLogs(1) = %+v, want 仅第 5 行", tail)
	}
}

// TestLogRingBuffersPartialLines 一条日志被拆成多次 Write 时，先缓冲、凑齐换行才入库。
// 这是 MultiWriter 下的真实情形：标准库 log 通常一次写整行，但不能假设。
func TestLogRingBuffersPartialLines(t *testing.T) {
	r := newLogRing(10)

	n, err := r.Write([]byte("2006/01/02 15:04:05 [p2p] 已连接"))
	if err != nil || n != len("2006/01/02 15:04:05 [p2p] 已连接") {
		t.Fatalf("Write 返回值 = (%d, %v)，应为 (len(p), nil)", n, err)
	}
	if got := len(r.RecentLogs(0)); got != 0 {
		t.Fatalf("未出现换行前不应产生记录，实际 %d 条", got)
	}

	if _, err := r.Write([]byte("对端\n")); err != nil {
		t.Fatal(err)
	}
	got := r.RecentLogs(0)
	if len(got) != 1 {
		t.Fatalf("补齐换行后应有 1 条记录，实际 %d 条", len(got))
	}
	if got[0].Message != "已连接对端" || got[0].Component != "p2p" {
		t.Fatalf("跨 Write 拼接结果不符: %+v", got[0])
	}
}

// TestLogRingSkipsBlankLines 空行不产生记录（日志里常见尾随空行）。
func TestLogRingSkipsBlankLines(t *testing.T) {
	r := newLogRing(10)
	if _, err := r.Write([]byte("\n   \n\t\n[ui] 有效行\n")); err != nil {
		t.Fatal(err)
	}
	got := r.RecentLogs(0)
	if len(got) != 1 || got[0].Message != "有效行" {
		t.Fatalf("空行未被跳过: %+v", got)
	}
}

// TestLogRingRecentLogsBounds n<=0 或 n 超长时返回全部，不越界。
func TestLogRingRecentLogsBounds(t *testing.T) {
	r := newLogRing(10)
	for i := 1; i <= 3; i++ {
		_, _ = fmt.Fprintf(r, "[node] 行 %d\n", i)
	}
	for _, n := range []int{-1, 0, 3, 99} {
		got := r.RecentLogs(n)
		if len(got) != 3 {
			t.Fatalf("RecentLogs(%d) 返回 %d 条, want 3", n, len(got))
		}
		if got[0].Message != "行 1" || got[2].Message != "行 3" {
			t.Fatalf("RecentLogs(%d) 顺序异常: %+v", n, got)
		}
	}

	// 空缓冲：返回空切片而不是 panic
	if got := newLogRing(1).RecentLogs(5); len(got) != 0 {
		t.Fatalf("空缓冲 RecentLogs = %+v, want 空", got)
	}
}

// TestNewLogRingDefaults 容量非法时回落到默认值，保证不会造出「写入即丢弃」的缓冲。
func TestNewLogRingDefaults(t *testing.T) {
	for _, max := range []int{0, -5} {
		if got := newLogRing(max).max; got != consoleLogCapacity {
			t.Fatalf("newLogRing(%d).max = %d, want %d", max, got, consoleLogCapacity)
		}
	}
}

// TestInstallLogRingMirrorsLog 安装后，标准库日志应被复制进环形缓冲，
// 且终端输出不受影响（只多一路观察者）。
func TestInstallLogRingMirrorsLog(t *testing.T) {
	oldOut, oldFlags, oldPrefix := log.Writer(), log.Flags(), log.Prefix()
	t.Cleanup(func() {
		log.SetOutput(oldOut)
		log.SetFlags(oldFlags)
		log.SetPrefix(oldPrefix)
	})

	ring := installLogRing()
	log.Printf("[console] 已注入日志来源")

	got := ring.RecentLogs(10)
	if len(got) == 0 {
		t.Fatal("安装日志环后未捕获到任何日志")
	}
	last := got[len(got)-1]
	if last.Component != "console" || last.Message != "已注入日志来源" {
		t.Fatalf("捕获到的日志内容不符: %+v", last)
	}
	if last.Time == "" {
		t.Fatalf("标准库日志带时间前缀，应能解析出时间: %+v", last)
	}
	// 原文不改写：消息里不应残留时间前缀、组件方括号或多余空白
	if strings.TrimSpace(last.Message) != last.Message || strings.Contains(last.Message, "[") {
		t.Fatalf("消息未剥离前缀，原文被破坏: %q", last.Message)
	}
}

// ---- 难度换算 ----

// TestRelativeDifficulty 展示难度必须与共识侧的 target 定义严格一致：
//
//	倍数 = 最低难度目标 / 当前目标 = 2^(bits - MaxTargetBits)
//
// 若这里和 pow 的定义脱钩，界面显示 "难度 16" 而实际链上是另一个难度，
// 属于对使用者的误导，因此用真实 big.Int 目标值反算做交叉验证。
func TestRelativeDifficulty(t *testing.T) {
	if got := relativeDifficulty(pow.MaxTargetBits - 1); got != 0 {
		t.Fatalf("低于最低难度的 bits 应返回 0，实际 %v", got)
	}
	if got := relativeDifficulty(pow.MaxTargetBits); got != 1 {
		t.Fatalf("最低难度倍数应为 1，实际 %v", got)
	}

	// 每提高 1 位，目标减半 → 倍数翻倍
	prev := relativeDifficulty(pow.MaxTargetBits)
	for bits := uint32(pow.MaxTargetBits) + 1; bits <= uint32(pow.MaxTargetBits)+8; bits++ {
		got := relativeDifficulty(bits)
		if got != prev*2 {
			t.Fatalf("bits=%d 倍数 = %v, want %v（应比上一档翻倍）", bits, got, prev*2)
		}
		prev = got
	}

	// 与 pow.BitsToTarget 交叉验证
	for _, delta := range []uint32{0, 1, 4, 8} {
		bits := pow.MaxTargetBits + delta
		lo := new(big.Float).SetInt(pow.BitsToTarget(pow.MaxTargetBits))
		hi := new(big.Float).SetInt(pow.BitsToTarget(bits))
		want, _ := new(big.Float).Quo(lo, hi).Float64()
		got := relativeDifficulty(bits)
		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("bits=%d 倍数 = %v, 按 target 定义应为 %v", bits, got, want)
		}
	}
}
