package p2p_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"p2pchain/internal/p2p"
)

// dialQuiet 拨入节点并读取节点主动发送的握手；**不回送本端握手**。
// 返回首次读取的错误：nil = 已被接受（收到握手）；EOF/错误 = 已被服务端拒绝关闭。
func dialQuiet(t *testing.T, addr string) (net.Conn, error) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("拨入失败: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	r := bufio.NewReader(conn)
	_, rerr := r.ReadString('\n')
	_ = conn.SetReadDeadline(time.Time{})
	return conn, rerr
}

// dialHandshaking 拨入并完成双向握手；若读节点握手即遭拒绝则原样返回该错误。
func dialHandshaking(t *testing.T, addr string) (net.Conn, error) {
	t.Helper()
	conn, err := dialQuiet(t, addr)
	if err != nil {
		return conn, err
	}
	hs := p2p.Message{Type: p2p.MsgHandshake,
		Payload: json.RawMessage(`{"node_id":"limits-test","chain_height":0,"listen_addr":""}`)}
	line, err := json.Marshal(hs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return conn, err
	}
	return conn, nil
}

// expectClosed 断言连接在 within 内被对端关闭（读到 EOF/错误；读超时 = 未关闭 = 失败）。
func expectClosed(t *testing.T, conn net.Conn, within time.Duration, label string) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(within))
	buf := make([]byte, 16)
	for {
		_, err := conn.Read(buf)
		if err == nil {
			continue // 残留数据，继续读到错误为止
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			t.Fatalf("%s: 连接未被对端关闭（读超时）", label)
		}
		return // EOF / connection reset = 已关闭
	}
}

// expectAlive 断言连接在 within 内未被对端关闭（读超时 = 仍存活）。
func expectAlive(t *testing.T, conn net.Conn, within time.Duration, label string) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(within))
	buf := make([]byte, 16)
	_, err := conn.Read(buf)
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return
	}
	t.Fatalf("%s: 连接意外被关闭: %v", label, err)
}

// TestHandshakeQuotaEnforced R1-B：同时处于「已注册未握手」状态的连接至多
// handshakeQuota(32) 个；第 33 个静默连接被立即拒绝（对端读到 EOF）。
// 顺序拨入保证判定确定性（第 i 个连接的裁决只取决于此前 i-1 个的状态）。
func TestHandshakeQuotaEnforced(t *testing.T) {
	h1 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	defer n1.Stop()

	const quota = 32 // 与生产常量一致
	var silent []net.Conn
	accepted, rejected := 0, 0
	for i := 0; i < quota+8; i++ {
		conn, rerr := dialQuiet(t, addr1)
		if rerr == nil {
			accepted++
			silent = append(silent, conn)
		} else {
			rejected++
			_ = conn.Close()
		}
		time.Sleep(5 * time.Millisecond)
	}
	if accepted != quota || rejected != 8 {
		t.Fatalf("握手配额判定不符: accepted=%d (want %d) rejected=%d (want 8)", accepted, quota, rejected)
	}
	if n1.PeerCount() != quota {
		t.Fatalf("未握手注册数 = %d, want %d", n1.PeerCount(), quota)
	}
	for _, c := range silent {
		_ = c.Close()
	}
	waitFor(t, func() bool { return n1.PeerCount() == 0 }, 5*time.Second, "静默连接未清理")
}

// TestInboundCapEnforced R1-B：入站连接至多 maxInbound(125) 个（含已完成握手者），
// 超额连接被立即拒绝；对端总数仍受 maxPeers(128) 约束。
// 顺序拨入 + 即时握手，使「未握手配额」不成为干扰因素。
func TestInboundCapEnforced(t *testing.T) {
	h1 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	defer n1.Stop()

	const capInbound = 125 // 与生产常量一致
	accepted, rejected := 0, 0
	// 必须持有全部已接受连接的引用：否则 conn 被循环覆盖失引后，GC 的
	// os.File finalizer 会提前关闭 fd，节点侧将其当作断开清理出注册表，
	// 注册计数永远涨不到上限，拒绝分支无从触发（实测 130 连接中途被
	// GC 关了 84 个——判读依据：关闭日志时间戳分散于测试运行中段）。
	var held []net.Conn
	for i := 0; i < capInbound+5; i++ {
		conn, rerr := dialHandshaking(t, addr1)
		if rerr == nil {
			accepted++
			held = append(held, conn)
		} else {
			rejected++
			_ = conn.Close()
		}
		time.Sleep(5 * time.Millisecond)
	}
	if accepted != capInbound || rejected != 5 {
		t.Fatalf("入站上限判定不符: accepted=%d (want %d) rejected=%d (want 5)", accepted, capInbound, rejected)
	}
	waitFor(t, func() bool { return n1.PeerCount() == capInbound }, 5*time.Second,
		"入站注册数未达上限")
	for _, c := range held {
		_ = c.Close()
	}
	waitFor(t, func() bool { return n1.PeerCount() == 0 }, 5*time.Second, "入站连接未清理")
}

// TestHandshakeDeadlineEnforced R1-B：连接后 handshakeTimeout(10s) 内未完成握手
// 即被断开（原实现声明该超时却从未执行）；已握手连接不受影响。
func TestHandshakeDeadlineEnforced(t *testing.T) {
	h1 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	defer n1.Stop()

	silent, serr := dialQuiet(t, addr1)
	if serr != nil {
		t.Fatalf("静默连接未收到节点握手: %v", serr)
	}
	good, gerr := dialHandshaking(t, addr1)
	if gerr != nil {
		t.Fatalf("正常连接被拒绝: %v", gerr)
	}
	waitFor(t, func() bool { return n1.PeerCount() == 2 }, 5*time.Second, "两连接未注册")

	aliveDone := make(chan struct{})
	go func() {
		defer close(aliveDone)
		expectAlive(t, good, 13*time.Second, "已握手连接")
	}()
	// 未握手连接：服务端读死线 = handshakeTimeout(10s)，容差到 15s
	expectClosed(t, silent, 15*time.Second, "未握手连接")
	<-aliveDone
	waitFor(t, func() bool { return n1.PeerCount() == 1 }, 5*time.Second,
		"超时连接未清理（已握手连接应保留）")
}
