// Package obs 提供运行时可观测性（observability-only）基础设施。
//
// PHASE MINING-2-I0-OBSERVABILITY-1 授权引入。设计铁律：OBSERVE ≠ CHANGE。
//
//   - Emit 绝不阻塞调用方：内部为有界通道 + select/default 丢弃（丢弃计数可查）；
//   - Emit 绝不 panic、绝不获取任何业务锁、绝不改变任何返回值/错误处理/锁顺序；
//   - 计数器全部为 atomic，等待方无需持锁；
//   - 序列化与磁盘写入全部在单一后台 goroutine 完成（批攒 + 周期 flush）；
//   - P2PCHAIN_OBS=off 可整体关闭（关闭后 Emit 仅剩一次原子读开销）；
//   - P2PCHAIN_OBS_FILE=<path> 把事件写入专用 JSONL 文件（默认写 stderr）。
//
// 事件行格式（JSONL，每行一个对象）：
//
//	{"ts":"RFC3339Nano","ts_us":<unix micros>,"event":"<NAME>","fields":{...}}
package obs

import (
	"encoding/json"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// chanCapacity 事件通道容量。满时新事件被丢弃（不阻塞、不等待）。
	// R2 实测峰值负载 ~600 events/s，16k 容量足以吸收任何观测窗口内的突发。
	chanCapacity = 16384
	// flushInterval 后台写出周期。
	flushInterval = 1 * time.Second
)

type rawEvent struct {
	tsUs int64
	name string
	kv   []any
}

var (
	enabled  atomic.Bool
	outFile  atomic.Value // writerBox（统一具体类型，避免 atomic.Value 混型 panic）
	dropped  atomic.Uint64
	emitted  atomic.Uint64
	reqSeq   atomic.Uint64
	events   chan rawEvent
	stopOnce sync.Once
	doneCh   chan struct{}
)

// writerBox 让 atomic.Value 持有一致的赋值兼容具体类型。
type writerBox struct{ w io.Writer }

// counters 动态注册的命名计数器（sync.Map 并发安全）。
var counters sync.Map // map[string]*atomic.Uint64

func init() {
	enabled.Store(true)
	// 环境开关：P2PCHAIN_OBS=off 整体关闭（测试/对比运行用）。
	if v := os.Getenv("P2PCHAIN_OBS"); v == "off" || v == "0" || v == "false" {
		enabled.Store(false)
	}
	outFile.Store(writerBox{w: os.Stderr})
	if p := os.Getenv("P2PCHAIN_OBS_FILE"); p != "" {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			outFile.Store(writerBox{w: f})
		}
		// 打开失败时保持 stderr（降级，不报错不打扰主流程）。
	}
	if !enabled.Load() {
		return
	}
	events = make(chan rawEvent, chanCapacity)
	doneCh = make(chan struct{})
	go writerLoop()
}

// Enabled 报告观测是否处于启用状态。
func Enabled() bool { return enabled.Load() }

// Disable 关闭事件流（后台 goroutine 排空后退出）。仅用于测试与对照运行。
func Disable() {
	if enabled.Swap(false) {
		stopOnce.Do(func() { close(events) })
		<-doneCh
	}
}

// EnableForTest 重建事件流并启用（仅测试使用；与 Disable 配对，非并发安全）。
// 用于 off→on 双态对照测试：不依赖进程环境变量，保证测试可自恢复。
func EnableForTest() {
	stopOnce = sync.Once{}
	events = make(chan rawEvent, chanCapacity)
	doneCh = make(chan struct{})
	enabled.Store(true)
	go writerLoop()
}

// SetOutputForTest 替换事件输出目标（仅测试使用；调用方须已 Disable 或在单线程上下文）。
func SetOutputForTest(w io.Writer) { outFile.Store(writerBox{w: w}) }

// NextRequestID 返回单调递增的请求标识（观测专用，与业务逻辑无关）。
func NextRequestID() uint64 { return reqSeq.Add(1) }

// Emit 非阻塞发送一条结构化事件。kv 为平铺的 key/value 对（key 必须是 string）。
//
// 通道满时事件被丢弃并递增丢弃计数；本函数任何路径都不阻塞、不 panic。
// value 仅支持 string / int / int64 / uint64 / bool / float64 / nil，
// 其他类型以 fmt.Sprintf 兜底字符串化（由后台 goroutine 完成）。
func Emit(name string, kv ...any) {
	if !enabled.Load() {
		return
	}
	// 通道在 Disable/未启用时可能为 nil：直接返回。
	ch := events
	if ch == nil {
		return
	}
	ev := rawEvent{tsUs: time.Now().UnixMicro(), name: name, kv: kv}
	select {
	case ch <- ev:
		emitted.Add(1)
	default:
		dropped.Add(1)
	}
}

// Inc 命名计数器原子自增（计数器仅观测，绝不参与任何业务判断）。
func Inc(name string) {
	if !enabled.Load() {
		return
	}
	v, _ := counters.LoadOrStore(name, &atomic.Uint64{})
	v.(*atomic.Uint64).Add(1)
}

// Counter 返回命名计数器当前值（未注册过返回 0）。
func Counter(name string) uint64 {
	v, ok := counters.Load(name)
	if !ok {
		return 0
	}
	return v.(*atomic.Uint64).Load()
}

// Stats 返回累计发出/丢弃的事件数（供测试与自检）。
func Stats() (emittedN, droppedN uint64) { return emitted.Load(), dropped.Load() }

// writerLoop 是唯一的序列化与落盘 goroutine：批攒事件 → 周期 flush。
func writerLoop() {
	defer close(doneCh)
	w := &flushWriter{w: outFile.Load().(writerBox).w}
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				w.flush()
				return
			}
			w.writeEvent(ev)
		case <-ticker.C:
			w.flush()
		}
	}
}

// flushWriter 聚合字节并按周期写出（bufio 简化版，仅本包使用）。
type flushWriter struct {
	w   io.Writer
	buf []byte
}

func (fw *flushWriter) writeEvent(ev rawEvent) {
	fw.buf = appendJSONLine(fw.buf[:0], ev)
	_, _ = fw.w.Write(fw.buf)
}

func (fw *flushWriter) flush() { /* 无缓冲句柄，writeEvent 已逐条写出；保留钩子便于扩展 bufio */ }

// appendJSONLine 手工拼一行 JSON（避免在后台为每条事件做反射序列化）。
func appendJSONLine(dst []byte, ev rawEvent) []byte {
	dst = append(dst, `{"ts":"`...)
	dst = append(dst, time.Now().Format(time.RFC3339Nano)...)
	dst = append(dst, `","ts_us":`...)
	dst = strconv.AppendInt(dst, ev.tsUs, 10)
	dst = append(dst, `,"event":`...)
	dst = strconv.AppendQuote(dst, ev.name)
	fields := make(map[string]any, len(ev.kv)/2)
	for i := 0; i+1 < len(ev.kv); i += 2 {
		k, ok := ev.kv[i].(string)
		if !ok {
			k = strconv.Itoa(i)
		}
		fields[k] = normalizeValue(ev.kv[i+1])
	}
	enc, err := json.Marshal(fields)
	if err != nil {
		enc = []byte(`{"_marshal_error":true}`)
	}
	dst = append(dst, `,"fields":`...)
	dst = append(dst, enc...)
	return append(dst, '}', '\n')
}

// normalizeValue 把观测值收敛为 JSON 可序列化类型。
func normalizeValue(v any) any {
	switch x := v.(type) {
	case nil, string, bool, int, int64, uint64, float64:
		return x
	case error:
		return x.Error()
	case time.Duration:
		return x.Microseconds()
	default:
		return strconv.Itoa(0) + ":" + jsonFallback(x)
	}
}

// jsonFallback 用标准库兜底字符串化任意类型。
func jsonFallback(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "<unserializable>"
	}
	return string(b)
}
