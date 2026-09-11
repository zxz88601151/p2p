package main

// 本文件覆盖两处「只有真跑才暴露」的行为：
//   - 种子节点断线重连：种子重启后，掉线的节点必须自己重新连上，
//     否则会永久成为孤岛链继续挖自己的分叉；
//   - 多核并行挖矿：并行路径产出的区块必须能通过共识校验（PoW 有效且 Nonce 落在搜索类内）。

import (
	"testing"
	"time"

	"p2pchain/internal/control"
	"p2pchain/internal/pow"
)

// startRuntimeAt 在指定地址启动节点，绑定失败时短暂重试（端口可能仍在回收中）。
func startRuntimeAt(t *testing.T, addr string, seeds []string, dir string) *nodeRuntime {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < 20; attempt++ {
		rt, err := newNodeRuntime(nodeConfig{
			ListenAddr: addr,
			RPCAddr:    "127.0.0.1:0",
			Seeds:      seeds,
			DataDir:    dir,
		})
		if err == nil {
			t.Cleanup(rt.Close)
			return rt
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("地址 %s 上启动节点失败: %v", addr, lastErr)
	return nil
}

// TestSeedReconnectAfterRestart 种子节点重启后，对端应自动重连。
func TestSeedReconnectAfterRestart(t *testing.T) {
	// 节点 A：先启动以取得一个稳定监听地址
	rtA := startRuntimeAt(t, "127.0.0.1:0", nil, t.TempDir())
	addrA := rtA.p2p.ListenAddr()

	// 节点 B：以 A 为种子
	rtB := startRuntimeAt(t, "127.0.0.1:0", []string{addrA}, t.TempDir())

	waitFor(t, func() bool { return rtB.p2p.PeerCount() == 1 }, 10*time.Second, "B 未连上种子节点 A")

	// A 下线 → B 的连接应被清理
	rtA.Close()
	waitFor(t, func() bool { return rtB.p2p.PeerCount() == 0 }, 10*time.Second, "A 下线后 B 未清理连接")

	// A 在同一地址重新启动 → B 应通过周期性种子检查自动重连
	rtA2 := startRuntimeAt(t, addrA, nil, t.TempDir())
	waitFor(t, func() bool { return rtB.p2p.PeerCount() == 1 }, 30*time.Second, "种子节点恢复后 B 未自动重连")

	if got := rtA2.p2p.PeerCount(); got == 0 {
		// 允许 A 侧稍晚完成握手
		waitFor(t, func() bool { return rtA2.p2p.PeerCount() == 1 }, 10*time.Second, "A 侧未记录到 B")
	}
}

// TestParallelMiningProducesValidBlock 并行挖矿产出的区块必须通过完整共识校验。
func TestParallelMiningProducesValidBlock(t *testing.T) {
	const miners = 4
	dir := t.TempDir()
	rt, err := newNodeRuntime(nodeConfig{
		ListenAddr: "127.0.0.1:0",
		RPCAddr:    "127.0.0.1:0",
		DataDir:    dir,
		Miners:     miners,
	})
	if err != nil {
		t.Fatalf("启动节点失败: %v", err)
	}
	t.Cleanup(rt.Close)
	if rt.svc.miners != miners {
		t.Fatalf("miners 配置未生效: %d", rt.svc.miners)
	}

	client := control.NewClient(rt.ctl.Addr())
	resp, err := client.Mine(6)
	if err != nil {
		t.Fatalf("按需出块失败: %v", err)
	}
	if resp.Mined != 6 || resp.Height != 6 {
		t.Fatalf("出块结果 = %+v, want mined=6 height=6", resp)
	}

	// 逐块校验：PoW 有效 + 链式结构正确（并行挖矿最容易在这里出错）
	prev := [32]byte{}
	for h := 0; h <= 6; h++ {
		b, err := rt.chain.BlockByHeight(h)
		if err != nil {
			t.Fatalf("取高度 %d 区块失败: %v", h, err)
		}
		if !pow.Validate(&b.Header) {
			t.Fatalf("高度 %d 的区块未通过 PoW 校验（nonce=%d）", h, b.Header.Nonce)
		}
		if b.Header.PrevBlockHash != prev {
			t.Fatalf("高度 %d 的区块父哈希断裂", h)
		}
		prev = b.Header.Hash()
	}

	// 注：不再断言「tip nonce 落在某个 worker 的等差类」——任何非负整数
	// 对 miners 取模都必然小于 miners，该断言恒真、无法证伪。搜索空间
	// 划分的正确性由 pow 包的单/多 worker 等价性测试保证。
}
