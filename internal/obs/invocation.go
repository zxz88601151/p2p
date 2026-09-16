// Package obs — invocation 观测能力（OBS-CHANGE）。
//
// 本文件为 PHASE AUTH-2-OBS-CHANGE 的交付物：在 **不修改任何既有观测行为**
// 的前提下，为后续 C-d IMPLEMENTATION 提供一组语义化、单位安全的
// invocation 观测 API，使 canonicalContains 调用点能够记录「调用次数与耗时」。
//
// 设计铁律（FINAL SPEC §11 / §14.3；本阶段授权 §2 / §3 / §4 / §5）：
//
//   - Event = one invocation：一次被观测的函数调用 = 一个 measurement unit；
//   - Counting = cumulative per-invocation：计数与 hit/miss、success/failure、
//     返回值、实现分支**无关**（计数器在函数入口自增）；
//   - Duration 单位 = 微秒，事件字段 `duration_us`（int64）——复用既有
//     REORG_DURATION_US 惯例，**不得改用其他时间单位**；
//   - measurement-only：不参与 consensus / validation / canonical selection /
//     rejection / persistence / network；观测失败（含丢弃）不改变任何业务结果；
//   - **零 schema 变更**：完全复用既有 JSONL 事件行形态与 normalizeValue 路径。
//
// 本文件不触碰 obs.go 中的任何既有声明或行为（向后兼容由「新增文件」保证）。
package obs

import "time"

// CountInvocation 计入一次 invocation（cumulative per-invocation）。
//
// counter 为累计计数器名，遵循既有命名惯例（形如 "canonical_contains_total"）。
// 本函数等价于 Inc，仅作为 invocation 观测语义的显式入口：计数**只**反映调用
// 次数，与调用结果、返回值或实现分支无关。
//
// 观测关闭（P2PCHAIN_OBS=off / Disable）时为纯空操作。
func CountInvocation(counter string) { Inc(counter) }

// InvocationCount 返回命名 invocation 计数器的当前累计值（未记录过返回 0）。
//
// 语义与 Counter 相同，作为「累计调用次数」这一冻结语义的显式读取入口。
func InvocationCount(counter string) uint64 { return Counter(counter) }

// EmitInvocationDuration 记录一次 invocation 的耗时事件。
//
// durationEvent 为耗时事件名，遵循既有命名惯例（形如
// "CANONICAL_CONTAINS_DURATION_US"）。耗时**固定**以微秒写入字段
// `duration_us`（int64），与既有 REORG_DURATION_US 惯例一致——调用方
// 不得改用毫秒、纳秒等其他时间单位。
//
// 本函数与 Emit 同语义：非阻塞、不 panic、不获取任何业务锁；观测关闭时为空操作。
func EmitInvocationDuration(durationEvent string, d time.Duration) {
	Emit(durationEvent, "duration_us", d.Microseconds())
}

// ObserveInvocation 开始观测一次 invocation，返回其结束回调。
//
//	done := obs.ObserveInvocation("canonical_contains_total", "CANONICAL_CONTAINS_DURATION_US")
//	defer done()
//
// 调用本函数即计入一次累计计数（counter）；随后调用返回的 done 记录本次
// invocation 的耗时（durationEvent，字段 duration_us，微秒）。
//
// 契约（不得弱化）：
//   - Event = one invocation：每次调用记且仅记一次；
//   - Counting = cumulative per-invocation：与调用结果无关；
//   - Duration = duration_us（微秒），复用 REORG_DURATION_US 惯例；
//   - measurement-only：done 绝不改变调用方的返回值、错误处理或锁顺序。
//
// 观测关闭（P2PCHAIN_OBS=off / Disable）时，返回的回调为纯空操作，
// 且不采集时间戳——OFF 态保持零开销。返回的回调**恒非 nil**，
// 调用方可直接 defer，无需判空。
func ObserveInvocation(counter, durationEvent string) func() {
	if !enabled.Load() {
		return func() {}
	}
	Inc(counter)
	start := time.Now()
	return func() {
		Emit(durationEvent, "duration_us", time.Since(start).Microseconds())
	}
}
