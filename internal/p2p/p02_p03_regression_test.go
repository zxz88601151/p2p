package p2p_test

// P0-2 / P0-3 判别性回归测试（随补丁附带）。
//
// P0-2（读循环被 Handler 同步阻塞）：TestDispatchDoesNotBlockReadLoop 用裸 TCP
// 向节点洪水式发送 40000 条小消息（约 20MB），Handler 侧每条睡眠 50ms 模拟
// 区块 PoW 校验/磁盘 IO。修复前读循环同步分发（20 条/秒），发送方被 TCP 背压
// 卡住（远超 5s，写 deadline 触发失败）；修复后读循环只做解析+入队（64 满即
// 丢），发送方数秒内写完。
//
// P0-3（SendTo 阻塞写）：TestSendToNonBlockingUnderStalledPeer 复用
// dialStalledPeer（握手后拒读的慢对端），循环 3000 次 SendTo 约 1KB 消息并
// 计时。修复前同步写在缓冲塞满后每次卡 writeTimeout（10s）；修复后仅非阻塞
// 入队（256 满即丢并返回错误），耗时 <3s。单次 SendTo 错误被忽略——队列满丢弃
// 是设计的非阻塞语义（调用方记日志，对端靠同步重试补齐）。

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"p2pchain/internal/p2p"
)

// slowHandler 模拟重业务：OnNewTx 每次睡眠，复现"Handler 拖住读循环"场景。
type slowHandler struct {
	recordHandler
	txSleep time.Duration
}

func (h *slowHandler) OnNewTx(addr string, raw json.RawMessage) {
	time.Sleep(h.txSleep)
	h.recordHandler.OnNewTx(addr, raw)
}

// dialFloodingPeer 以裸 TCP 拨入节点并完成握手，返回连接（调用方负责关闭）。
// 与 dialStalledPeer 不同：此连接用于持续写，不设读 deadline。
func dialFloodingPeer(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("拨入失败: %v", err)
	}
	r := bufio.NewReader(conn)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := r.ReadString('\n'); err != nil {
		t.Fatalf("未收到节点握手: %v", err)
	}
	hs := p2p.Message{Type: p2p.MsgHandshake,
		Payload: json.RawMessage(`{"node_id":"flooding-peer","chain_height":0,"listen_addr":""}`)}
	line, err := json.Marshal(hs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(append(line, '\n')); err != nil {
		t.Fatalf("发送握手失败: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{}) // 此后只写不读
	return conn
}

func TestDispatchDoesNotBlockReadLoop(t *testing.T) {
	h := &slowHandler{txSleep: 50 * time.Millisecond}
	n1, addr1 := startTestNode(t, h)
	defer n1.Stop()

	conn := dialFloodingPeer(t, addr1)
	defer conn.Close()
	waitFor(t, func() bool { return n1.PeerCount() == 1 }, 5*time.Second, "对端未注册")

	// 约 500B/条 × 40000 条 ≈ 20MB：超过 TCP 自适应缓冲上限，确保旧代码下
	// 必然触发发送方背压（新代码下读循环秒级排空）。
	payload, _ := json.Marshal(p2p.TxPayload{Encoded: strings.Repeat("ab", 150)})
	line, _ := json.Marshal(p2p.Message{Type: p2p.MsgNewTx, Payload: payload})
	line = append(line, '\n')
	if len(line) > 1024 {
		t.Fatalf("测试消息过大: %d", len(line))
	}

	start := time.Now()
	for i := 0; i < 40000; i++ {
		// 写 deadline 防止旧代码下无限挂起：触发即判失败（信息明确）。
		_ = conn.SetWriteDeadline(time.Now().Add(8 * time.Second))
		if _, err := conn.Write(line); err != nil {
			t.Fatalf("第 %d 条发送被阻塞（读循环疑似被 Handler 卡住）: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	t.Logf("40000 条（~20MB）发送耗时 %v", elapsed)
	if elapsed > 5*time.Second {
		t.Fatalf("发送耗时 %v 超过 5s：读循环被 Handler 阻塞（P0-2 回归）", elapsed)
	}
}

func TestSendToNonBlockingUnderStalledPeer(t *testing.T) {
	h := &recordHandler{}
	n1, addr1 := startTestNode(t, h)
	defer n1.Stop()

	stalled := dialStalledPeer(t, addr1)
	defer stalled.Close()
	// 压缩慢对端接收缓冲：旧代码下同步写必然触发阻塞（新代码走出站队列不受影响）。
	// 显式 SetReadBuffer 会关闭该 socket 的接收缓冲自适应，4KB 即上限。
	if tc, ok := stalled.(*net.TCPConn); ok {
		if err := tc.SetReadBuffer(4 * 1024); err != nil {
			t.Fatalf("设置接收缓冲失败: %v", err)
		}
	}
	stalledAddr := stalled.LocalAddr().String()
	waitFor(t, func() bool { return n1.PeerCount() == 1 }, 5*time.Second, "慢对端未注册")

	payload, _ := json.Marshal(p2p.TxPayload{Encoded: strings.Repeat("ab", 350)})
	msg := p2p.Message{Type: p2p.MsgNewTx, Payload: payload}

	// 30000 条 ≈ 30MB：超过发送方 TCP 缓冲自适应上限（通常 4~6MB），旧代码下
	// 同步写必然在缓冲塞满后阻塞（每次最长 10s 写超时）；新代码仅入队，毫秒级。
	start := time.Now()
	for i := 0; i < 30000; i++ {
		// 队列满返回错误是设计的非阻塞语义，此处只计时不 care 结果。
		_ = n1.SendTo(stalledAddr, msg)
	}
	elapsed := time.Since(start)
	t.Logf("30000 次 SendTo（~30MB，慢对端）耗时 %v", elapsed)
	if elapsed > 5*time.Second {
		t.Fatalf("SendTo 耗时 %v 超过 5s：同步写被慢对端卡住（P0-3 回归）", elapsed)
	}
}
