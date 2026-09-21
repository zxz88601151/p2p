package p2p_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"p2pchain/internal/p2p"
)

// runtimeNumGoroutine 当前存活 goroutine 数（Stop 泄漏检测用）。
func runtimeNumGoroutine() int { return runtime.NumGoroutine() }

// dialStalledPeer 以原始 TCP 拨入节点，完成双向握手后**不再读取任何数据**——
// 复现 LTM-2 慢对端（接收缓冲塞满后，同步写将对端 writeTimeout 卡死）。
// 返回的连接由调用方关闭。
func dialStalledPeer(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("拨入失败: %v", err)
	}
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	r := bufio.NewReader(conn)
	// 节点在注册连接后立即主动发送握手（先于出站 writer 启动）
	if _, err := r.ReadString('\n'); err != nil {
		t.Fatalf("未收到节点握手: %v", err)
	}
	hs := p2p.Message{Type: p2p.MsgHandshake,
		Payload: json.RawMessage(`{"node_id":"stalled-peer","chain_height":0,"listen_addr":""}`)}
	line, err := json.Marshal(hs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		t.Fatalf("发送握手失败: %v", err)
	}
	_ = conn.SetDeadline(time.Time{}) // 清除 deadline：此后永不读取，模拟 socket 缓冲塞满
	return conn
}

// TestBroadcastNonBlockingUnderStalledPeer R1-A 判别性回归（LTM-2 单元级）：
// 慢对端（握手后拒读）不得阻塞 Broadcast 调用方；慢对端存在期间健康对端投递通道保持活性。
//
// 判别性（本机实测）：3000 条 × ~1KB ≈ 3MB，远超本地回环 send/recv 内核缓冲——
// R1-A 前同步写在慢对端上缓冲塞满即卡 writeTimeout(10s)，3 次失败断开前累计 ~30s
// → Broadcast 耗时远超 1s → FAIL（实测 30.19s）；
// R1-A 后广播仅非阻塞入队（队列满即对该对端丢帧，OUTQ_OVERFLOW 观测）
// → 耗时 <1s → PASS。健康对端活性用「突发后标记消息必达」证明。
func TestBroadcastNonBlockingUnderStalledPeer(t *testing.T) {
	h1 := &recordHandler{}
	h2 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	defer n1.Stop()
	n2, _ := startTestNode(t, h2)
	defer n2.Stop()

	if err := n2.ConnectToPeer(addr1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return n1.PeerCount() == 1 }, 5*time.Second, "n2 未连上 n1")

	stalled := dialStalledPeer(t, addr1)
	defer stalled.Close()
	waitFor(t, func() bool { return n1.PeerCount() == 2 }, 5*time.Second, "慢对端未注册")

	payload, _ := json.Marshal(p2p.TxPayload{Encoded: strings.Repeat("ab", 500)})
	const total = 3000
	start := time.Now()
	for i := 0; i < total; i++ {
		n1.Broadcast(p2p.Message{Type: p2p.MsgNewTx, Payload: payload})
	}
	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("Broadcast 被慢对端阻塞 %v（R1-A 语义要求非阻塞入队）", elapsed)
	}

	// 健康对端活性：慢对端持续滞塞期间，后续消息仍按时到达 n2
	marker, _ := json.Marshal(p2p.TxPayload{Encoded: "marker-after-burst"})
	n1.Broadcast(p2p.Message{Type: p2p.MsgNewTx, Payload: marker})
	waitFor(t, func() bool {
		h2.mu.Lock()
		defer h2.mu.Unlock()
		for _, raw := range h2.txs {
			if strings.Contains(raw, "marker-after-burst") {
				return true
			}
		}
		return false
	}, 10*time.Second, "慢对端滞塞期间健康对端投递通道失效")
}

// TestBroadcastPerPeerFIFOOrder R1-A 顺序保持：同一对端上广播按发送顺序到达
// （单 writer 串行写出 ⇒ per-peer FIFO；本测试同时覆盖小流量下全量必达）。
func TestBroadcastPerPeerFIFOOrder(t *testing.T) {
	h1 := &recordHandler{}
	h2 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	defer n1.Stop()
	n2, _ := startTestNode(t, h2)
	defer n2.Stop()

	if err := n2.ConnectToPeer(addr1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return n1.PeerCount() == 1 && n2.PeerCount() == 1 }, 5*time.Second, "未连接")

	const total = 50
	for i := 0; i < total; i++ {
		payload, _ := json.Marshal(p2p.TxPayload{Encoded: fmt.Sprintf("seq-%03d", i)})
		n1.Broadcast(p2p.Message{Type: p2p.MsgNewTx, Payload: payload})
	}

	waitFor(t, func() bool {
		_, _, txs, _, _ := h2.counts()
		return txs == total
	}, 5*time.Second, "对端未收全广播")

	h2.mu.Lock()
	defer h2.mu.Unlock()
	for i, raw := range h2.txs {
		var tp p2p.TxPayload
		if err := json.Unmarshal([]byte(raw), &tp); err != nil {
			t.Fatalf("解析第 %d 条失败: %v", i, err)
		}
		if want := fmt.Sprintf("seq-%03d", i); tp.Encoded != want {
			t.Fatalf("per-peer FIFO 乱序: 位置 %d = %q, want %q", i, tp.Encoded, want)
		}
	}
}

// TestStopReleasesWriterGoroutines R1-A 生命周期：Stop 后各连接的出站 writer 必须退出
// （经 p.closed 终结信号），不残留阻塞在队列上的 goroutine。
func TestStopReleasesWriterGoroutines(t *testing.T) {
	h1 := &recordHandler{}
	h2 := &recordHandler{}
	n1, addr1 := startTestNode(t, h1)
	n2, _ := startTestNode(t, h2)
	defer n2.Stop()

	if err := n2.ConnectToPeer(addr1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return n1.PeerCount() == 1 }, 5*time.Second, "未连接")

	before := runtimeNumGoroutine()
	n1.Broadcast(p2p.Message{Type: p2p.MsgNewTx,
		Payload: json.RawMessage(`{"encoded":"aa"}`)})
	n1.Stop()

	// writer 退出路径：handleConn 清理（连接关闭 → 读循环退出 → defer signalClosed）。
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if runtimeNumGoroutine() <= before {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("Stop 后 goroutine 未回落: before=%d after=%d", before, runtimeNumGoroutine())
}
