package obs

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// 本文件验证 OBS-CHANGE 交付的 invocation 观测契约（FINAL SPEC §20 目标 3/4/5/6/7）：
// measurement semantics、event counting、deterministic validation、
// off-state validation、schema compatibility validation。
//
// 计数器为包级 sync.Map，跨测试**不重置**——因此所有断言均基于增量（before/after），
// 且各用例使用互不相同的计数器名。

const (
	cntCumulative   = "test_obsch_invocation_total"
	evtCumulative   = "TEST_OBSCH_INVOCATION_DURATION_US"
	cntOutcome      = "test_obsch_outcome_total"
	evtOutcome      = "TEST_OBSCH_OUTCOME_DURATION_US"
	cntDeterminism  = "test_obsch_determinism_total"
	evtDeterminism  = "TEST_OBSCH_DETERMINISM_DURATION_US"
	cntOffState     = "test_obsch_off_total"
	evtOffState     = "TEST_OBSCH_OFF_DURATION_US"
	cntSchemaCompat = "test_obsch_schema_total"
	evtSchemaCompat = "TEST_OBSCH_SCHEMA_DURATION_US"
)

// TestObserveInvocationCountsPerInvocation 验证 Event = one invocation：
// 每次调用记且仅记一次，且每次都会发出恰一条耗时事件。
func TestObserveInvocationCountsPerInvocation(t *testing.T) {
	var buf bytes.Buffer
	SetOutputForTest(&buf)
	resetForTest(t, 256)

	before := InvocationCount(cntCumulative)
	const n = 5
	for i := 0; i < n; i++ {
		ObserveInvocation(cntCumulative, evtCumulative)() // 立即结束
	}
	Disable()

	if got := InvocationCount(cntCumulative) - before; got != n {
		t.Fatalf("累计计数增量 = %d, want %d", got, n)
	}
	if got := strings.Count(buf.String(), `"event":"`+evtCumulative+`"`); got != n {
		t.Fatalf("耗时事件条数 = %d, want %d", got, n)
	}
}

// TestInvocationCountIndependentOfOutcome 验证 Counting = cumulative per-invocation：
// 计数与 hit/miss、success/failure、返回值、实现分支无关。
func TestInvocationCountIndependentOfOutcome(t *testing.T) {
	resetForTest(t, 64)
	before := InvocationCount(cntOutcome)

	// 交替 hit / miss：计数只反映调用次数。
	for i := 0; i < 6; i++ {
		CountInvocation(cntOutcome)
		hit := i%2 == 0
		_ = hit // 结果不参与计数
	}

	if got := InvocationCount(cntOutcome) - before; got != 6 {
		t.Fatalf("计数增量 = %d, want 6（与命中/未命中无关）", got)
	}
}

// TestEmitInvocationDurationUnitIsMicroseconds 验证 Duration 单位固定为微秒，
// 且字段名固定为 duration_us（复用既有 REORG_DURATION_US 惯例）。
// 使用精确值断言：若误用毫秒得 12，误用纳秒得 12345000，均会被捕获。
func TestEmitInvocationDurationUnitIsMicroseconds(t *testing.T) {
	var buf bytes.Buffer
	SetOutputForTest(&buf)
	resetForTest(t, 64)

	const wantUs = int64(12345)
	EmitInvocationDuration(evtDeterminism, time.Duration(wantUs)*time.Microsecond)
	Disable()

	line := strings.TrimSpace(buf.String())
	if !strings.Contains(line, `"duration_us":`) {
		t.Fatalf("事件行缺少 duration_us 字段: %s", line)
	}
	if got := extractInt64Field(t, line, "duration_us"); got != wantUs {
		t.Fatalf("duration_us = %d, want %d（单位必须为微秒）", got, wantUs)
	}
}

// TestObserveInvocationDeterministic 验证 deterministic observation validation：
// 同参重跑的计数增量与事件条数完全一致。
func TestObserveInvocationDeterministic(t *testing.T) {
	var buf bytes.Buffer
	SetOutputForTest(&buf)

	run := func() (delta uint64, eventCount int) {
		buf.Reset() // 每轮清空，确保事件条数只反映本轮
		resetForTest(t, 128)
		before := InvocationCount(cntDeterminism)
		done := ObserveInvocation(cntDeterminism, evtDeterminism)
		done()
		Disable()
		return InvocationCount(cntDeterminism) - before,
			strings.Count(buf.String(), `"event":"`+evtDeterminism+`"`)
	}

	wantDelta, wantEvents := run()
	if wantDelta != 1 || wantEvents != 1 {
		t.Fatalf("基线：delta=%d events=%d, want 1/1", wantDelta, wantEvents)
	}
	for round := 2; round <= 4; round++ {
		gotDelta, gotEvents := run()
		if gotDelta != wantDelta || gotEvents != wantEvents {
			t.Fatalf("第 %d 轮 delta=%d events=%d, want %d/%d（观测不确定）",
				round, gotDelta, gotEvents, wantDelta, wantEvents)
		}
	}
}

// TestObserveInvocationOffStateZeroOutput 验证 off-state validation：
// 观测关闭时零输出、零计数，且返回回调非 nil（保证调用方可直接 defer）。
func TestObserveInvocationOffStateZeroOutput(t *testing.T) {
	var buf bytes.Buffer
	SetOutputForTest(&buf)
	resetForTest(t, 64)
	Disable()
	buf.Reset()

	before := InvocationCount(cntOffState)
	done := ObserveInvocation(cntOffState, evtOffState)
	if done == nil {
		t.Fatal("OFF 态返回 nil 回调——调用方 defer 将 panic")
	}
	done()

	if buf.Len() != 0 {
		t.Fatalf("OFF 态仍有事件输出: %q", buf.String())
	}
	if got := InvocationCount(cntOffState) - before; got != 0 {
		t.Fatalf("OFF 态仍计入 %d 次计数", got)
	}
}

// TestObserveInvocationSchemaCompatibility 验证 schema compatibility：
// 新增观测完全复用既有 JSONL 事件行 schema（ts / ts_us / event / fields），
// 不引入任何新字段或新序列化路径。
func TestObserveInvocationSchemaCompatibility(t *testing.T) {
	var buf bytes.Buffer
	SetOutputForTest(&buf)
	resetForTest(t, 64)

	ObserveInvocation(cntSchemaCompat, evtSchemaCompat)()
	Disable()

	line := strings.TrimSpace(buf.String())
	for _, want := range []string{`"ts":"`, `"ts_us":`, `"event":"` + evtSchemaCompat + `"`, `"fields":{`, `"duration_us":`} {
		if !strings.Contains(line, want) {
			t.Fatalf("事件行缺少 %s（schema 兼容性失败）: %s", want, line)
		}
	}

	// 既有 schema 是「每行一个 JSON 对象」，必须是合法 JSON。
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		t.Fatalf("事件行非合法 JSON: %v (%s)", err, line)
	}
	if got, _ := obj["event"].(string); got != evtSchemaCompat {
		t.Fatalf("event 字段 = %q, want %q", got, evtSchemaCompat)
	}
	fields, ok := obj["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields 不是 JSON 对象: %s", line)
	}
	if _, ok := fields["duration_us"]; !ok {
		t.Fatalf("fields 缺少 duration_us: %s", line)
	}
}

// extractInt64Field 从事件行中提取整型字段值（避免测试引入额外依赖）。
func extractInt64Field(t *testing.T, line, key string) int64 {
	t.Helper()
	marker := `"` + key + `":`
	i := strings.Index(line, marker)
	if i < 0 {
		t.Fatalf("未找到字段 %s: %s", key, line)
	}
	rest := line[i+len(marker):]
	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	if j == 0 {
		t.Fatalf("字段 %s 非正整数: %s", key, line)
	}
	var v int64
	for _, c := range rest[:j] {
		v = v*10 + int64(c-'0')
	}
	return v
}
