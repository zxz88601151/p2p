// P0-5 Eclipse 缓解：连接槽位划分、不良行为记分封禁、地址本硬化。
//
// 威胁模型：攻击者控制一批 IP，试图 (1) 用入站连接挤占全部槽位，
// (2) 污染地址本，(3) 零成本长期占据连接位喂垃圾。本文件实现四层防御：
//
//	D5-1 槽位划分：maxPeers=128 不变，其中 16 个槽位预留给外拨；
//	     入站上限 = 112；per-IP 入站上限 4；外拨 per-/16 上限 2。
//	     入站打满不再堵死外拨/种子拨号。
//	D5-2 种子锚点：ConnectToSeed 满员时驱逐价值最低的入站腾出槽位；
//	     watchSeeds 改为抖动指数退避（见 cmd/node/main.go）。
//	D5-3 记分封禁：Penalize 按 IP 记分，阈值 100 → 断开该 IP 全部连接并封禁 10 分钟；
//	     记分按 60s 周期半衰，防正常抖动累积误伤。
//	D5-4 地址本硬化：learnPeer 加格式校验 + 1000 上限随机驱逐。
//
// 范围边界：不做完整 tried/new 表、不做 peer 身份签名、不做传输加密（B 档工程）。
package p2p

import (
	"fmt"
	"log"
	"net"
	"time"

	"p2pchain/internal/obs"
)

// P0-5 常量。
const (
	// maxOutboundReserved 外拨预留槽位：即使入站打满，外拨/种子仍有路。
	maxOutboundReserved = 16
	// maxInboundPerIP 单个 IP 的入站连接上限（回环豁免，见下）。
	maxInboundPerIP = 4
	// maxOutboundPer16 单个 /16 网段的外拨连接上限（回环豁免）。
	maxOutboundPer16 = 2
	// banThreshold 记分封禁阈值。
	banThreshold = 100
	// banDuration 封禁时长（内存表，重启清空）。
	banDuration = 10 * time.Minute
	// scoreDecayInterval 记分衰减周期：每经过一个周期既有分数减半。
	scoreDecayInterval = 60 * time.Second
	// maxKnownPeers 地址本上限。
	maxKnownPeers = 1000

	// 记分值。
	scoreInvalidBlock   = 50 // 共识层拒绝的非法块
	scoreMalformedMsg   = 10 // 编解码失败的畸形消息
	scoreSyncFlood      = 20 // get_blocks 洪水
	scorePreHandshake   = 10 // 握手前非握手消息
	scoreInvalidTxFlood = 10 // 畸形交易

	// syncFloodWindow / syncFloodMax：窗口内 get_blocks 超过该次数判为洪水。
	syncFloodWindow = 10 * time.Second
	syncFloodMax    = 20
)

// maxInbound 入站连接上限 = 总数 - 外拨预留（P0-5：112，之前为 125）。
const maxInbound = maxPeers - maxOutboundReserved

// MaxInbound 入站连接上限（导出别名，供外部测试包 p2p_test 引用）。
const MaxInbound = maxInbound

// peerScore 单个 IP 的不良行为记分。
type peerScore struct {
	points  int
	updated time.Time
}

// hostOf 取地址的 host 部分（IP）；解析失败时原样返回。
func hostOf(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}

// isLoopbackHost 回环地址豁免：本地测试流量不参与 per-IP/per-/16 限制与记分。
func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// subnet16 取 IPv4 的 /16 网段标识；非 IPv4 原样返回。
func subnet16(host string) string {
	if ip := net.ParseIP(host).To4(); ip != nil {
		return fmt.Sprintf("%d.%d", ip[0], ip[1])
	}
	return host
}

// Penalize 记录对端不良行为（addr 为 IP:port，记分按 IP 归集）。
// 达到阈值时断开该 IP 全部连接并封禁 banDuration。
func (n *Node) Penalize(addr string, points int, reason string) {
	ip := hostOf(addr)
	if isLoopbackHost(ip) {
		return
	}
	n.mu.Lock()
	if until, ok := n.banned[ip]; ok && time.Now().Before(until) {
		n.mu.Unlock()
		return
	}
	ps := n.scores[ip]
	if ps == nil {
		ps = &peerScore{}
		n.scores[ip] = ps
	}
	// 衰减：每经过一个完整周期既有分数减半（防正常抖动累积误伤）。
	if elapsed := time.Since(ps.updated); elapsed >= scoreDecayInterval {
		for i := 0; i < int(elapsed/scoreDecayInterval) && ps.points > 0; i++ {
			ps.points /= 2
		}
	}
	ps.points += points
	ps.updated = time.Now()
	total := ps.points
	n.mu.Unlock()

	obs.Emit("MISBEHAVIOR", "peer", addr, "reason", reason, "points", points, "total", total)
	log.Printf("[p2p] 对端 %s 不良行为：%s（+%d，累计 %d）", addr, reason, points, total)
	if total >= banThreshold {
		n.banIP(ip, reason)
	}
}

// banIP 封禁 IP：记录解封时间并断开该 IP 的全部现有连接。
func (n *Node) banIP(ip, reason string) {
	n.mu.Lock()
	n.banned[ip] = time.Now().Add(banDuration)
	var victims []*Peer
	for _, p := range n.peers {
		if hostOf(p.Addr) == ip {
			victims = append(victims, p)
		}
	}
	n.mu.Unlock()
	for _, p := range victims {
		n.dropPeer(p, "不良行为封禁: "+reason)
	}
	obs.Emit("PEER_BANNED", "ip", ip, "reason", reason, "duration", banDuration.String())
	log.Printf("[p2p] 封禁 %s %v（%s）", ip, banDuration, reason)
}

// isBannedIP 检查 IP 是否在封禁期内（调用方持有 n.mu 或用 RLock）。
func (n *Node) isBannedIP(ip string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	until, ok := n.banned[ip]
	return ok && time.Now().Before(until)
}

// Disconnect 对外暴露的按地址断开（service 层对恶意对端使用）。
func (n *Node) Disconnect(addr, reason string) {
	n.dropPeerByAddr(addr, reason)
}

// NoteSyncRequest 记录一次 get_blocks 请求；超过洪水阈值时自动记分并返回 true
// （调用方应忽略本次请求，不再响应）。
func (n *Node) NoteSyncRequest(addr string) bool {
	now := time.Now()
	n.mu.Lock()
	reqs := n.syncReqs[addr]
	kept := reqs[:0]
	for _, t := range reqs {
		if now.Sub(t) <= syncFloodWindow {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	n.syncReqs[addr] = kept
	over := len(kept) > syncFloodMax
	n.mu.Unlock()
	if over {
		n.Penalize(addr, scoreSyncFlood, "get_blocks 同步请求洪水")
		return true
	}
	return false
}

// evictLowestValueInboundLocked 选出价值最低的入站连接用于驱逐
// （调用方持有 n.mu）：未握手优先（其中更老的优先）；其次记分高的；
// 最后按 lastActive 最老。返回 nil 表示无入站可驱逐。
func (n *Node) evictLowestValueInboundLocked() *Peer {
	var victim *Peer
	scoreOf := func(p *Peer) int {
		if ps := n.scores[hostOf(p.Addr)]; ps != nil {
			return ps.points
		}
		return 0
	}
	for _, p := range n.peers {
		if !p.inbound {
			continue
		}
		if victim == nil {
			victim = p
			continue
		}
		ph, vh := p.handshaked.Load(), victim.handshaked.Load()
		switch {
		case !ph && vh:
			victim = p // 未握手者优先驱逐
		case ph == vh:
			if pq, vq := scoreOf(p), scoreOf(victim); pq != vq {
				if pq > vq {
					victim = p // 记分高的优先驱逐
				}
			} else if p.lastActive.Before(victim.lastActive) {
				victim = p // 更久未活跃的优先驱逐
			}
		}
	}
	return victim
}

// ConnectToSeed 种子优先拨号（P0-5 D5-2 锚点）。
// 满员时驱逐一个价值最低的入站连接腾出槽位，而非直接放弃——
// 保证入站挤占攻击下种子永远有路。
func (n *Node) ConnectToSeed(addr string) error {
	if err := n.ConnectToPeer(addr); err == nil {
		return nil
	}
	n.mu.Lock()
	victim := n.evictLowestValueInboundLocked()
	n.mu.Unlock()
	if victim == nil {
		return fmt.Errorf("种子 %s 拨号失败且无可驱逐入站连接", addr)
	}
	n.dropPeer(victim, "为种子连接腾出槽位")
	return n.ConnectToPeer(addr)
}
