package obs

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// resetForTest 重建小容量事件通道（仅测试使用）。
func resetForTest(t *testing.T, capacity int) {
	t.Helper()
	Disable()
	stopOnce = sync.Once{}
	events = make(chan rawEvent, capacity)
	doneCh = make(chan struct{})
	enabled.Store(true)
	go writerLoop()
	t.Cleanup(Disable)
}

// TestEmitNonBlockingWhenFull 验证通道满时 Emit 不阻塞（§10 Test F）。
func TestEmitNonBlockingWhenFull(t *testing.T) {
	resetForTest(t, 2)
	for i := 0; i < 100; i++ {
		Emit("TEST_FILL", "i", i)
	}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			Emit("TEST_OVERFLOW", "i", i)
		}
		close(done)
	}()
	select {
	case <-done:
		// 期望路径：立即完成
	case <-time.After(2 * time.Second):
		t.Fatal("Emit 在通道满时阻塞了 —— 违反 OBSERVE≠CHANGE")
	}
	_, droppedN := Stats()
	if droppedN == 0 {
		t.Fatal("通道满时应有丢弃计数")
	}
}

// TestConcurrentEmitNoPanic 验证并发 Emit 不 panic、计数守恒。
func TestConcurrentEmitNoPanic(t *testing.T) {
	resetForTest(t, 1024)
	var wg sync.WaitGroup
	for g := 0; g < 64; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				Emit("TEST_CONCURRENT", "g", g, "i", i)
				Inc("test_counter")
			}
		}(g)
	}
	wg.Wait()
	if got := Counter("test_counter"); got != 64*200 {
		t.Fatalf("counter = %d, want %d", got, 64*200)
	}
}

// TestDisabledZeroOutput 验证 Disable 后 Emit 不产生任何输出（对照运行用）。
func TestDisabledZeroOutput(t *testing.T) {
	var buf bytes.Buffer
	SetOutputForTest(&buf)
	resetForTest(t, 64)
	Disable()
	buf.Reset()
	Emit("TEST_DISABLED", "k", "v")
	Inc("test_disabled_counter")
	if buf.Len() != 0 {
		t.Fatalf("Disable 后仍有输出: %q", buf.String())
	}
}

// TestEventJSONShape 验证事件行 JSON 形态（ts/ts_us/event/fields）。
func TestEventJSONShape(t *testing.T) {
	var buf bytes.Buffer
	SetOutputForTest(&buf)
	resetForTest(t, 64)
	Emit("TEST_SHAPE", "parent", "abcdef", "age_s", 1.5, "err", errors.New("boom"), "found", true, "n", uint64(7))
	Disable()
	line := strings.TrimSpace(buf.String())
	for _, want := range []string{`"event":"TEST_SHAPE"`, `"parent":"abcdef"`, `"found":true`, `"n":7`, `"ts_us":`} {
		if !strings.Contains(line, want) {
			t.Fatalf("事件行缺少 %s: %s", want, line)
		}
	}
}

// TestNextRequestIDMonotonic 验证 request_id 单调递增。
func TestNextRequestIDMonotonic(t *testing.T) {
	a := NextRequestID()
	b := NextRequestID()
	if b <= a {
		t.Fatalf("request_id 非单调: %d -> %d", a, b)
	}
}
