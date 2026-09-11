package p2p_test

import (
	"net"
	"testing"
	"time"
)

// netListenEphemeral 申请一个空闲的本地监听器，返回监听器与其地址。
// 监听器保持打开并交给被测节点使用（Serve 注入），因此「端口已就绪」是确定的事实，
// 不需要再靠拨号探测来判断，也不会因探测连接污染对等节点计数。
func netListenEphemeral() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

// waitFor 轮询等待条件成立。
func waitFor(t *testing.T, cond func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(msg)
}
