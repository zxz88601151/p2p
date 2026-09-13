package blockchain

import (
	"fmt"

	"p2pchain/internal/storage"
)

// ChainVerifyReport 是「只读链校验」的结果报告。
//
// 约定（供 CLI 与第三方解析）：
//   - Valid == true  时：FailHeight = -1，FailHash / Reason 为空串；
//   - Valid == false 时：FailHeight 为首个不合法区块的高度（可确定时），
//     FailHash 为该区块头哈希（可确定时），Reason 为具体原因，绝不为泛化的 "invalid"；
//   - Height 为存储中的最高区块高度，空库时为 -1。
type ChainVerifyReport struct {
	DataDir     string `json:"data_dir"`
	Height      int    `json:"height"`
	Blocks      int    `json:"blocks_checked"`
	GenesisHash string `json:"genesis_hash"`
	TipHash     string `json:"tip_hash"`
	Valid       bool   `json:"valid"`
	FailHeight  int    `json:"fail_height"`
	FailHash    string `json:"fail_hash"`
	Reason      string `json:"reason"`
}

// VerifyStoredChain 对已持久化的本地链执行一次**只读**的完整确定性校验。
//
// 产品契约（PHASE BRAND-1.2 §3）：
//
//	INPUT        已落盘的本地链（blocks.dat）
//	PROCESS      只读回放：逐块重新执行与运行时完全相同的共识校验
//	             （复用 applyBlock —— 与启动回放同一条路径，因此 persist=false，绝不写回）
//	OUTPUT       verified / rejected + 明确的失败高度、区块哈希与具体原因
//	SIDE EFFECT  零持久化变更
//
// 只读保证的实现方式：本函数只通过 storage.BlockStore 的读接口取块，
// 追加路径一律走 applyBlock（persist=false），因此
//   - 不写 blocks.dat；
//   - 不创建/不删除/不获取任何目录锁；
//   - 不加载或写入 wallet.json；
//   - 不启动网络、不触发 P2P、不触发挖矿。
//
// 调用方应自行以只读方式打开存储（storage.OpenFileBlockStoreReadOnly），
// 从机制上杜绝写入可能。
//
// 校验失败时返回 (report, nil)：校验本身成功执行、结论是「链不合法」；
// 只有读取/索引等基础设施错误才返回 error。
func VerifyStoredChain(store storage.BlockStore) (*ChainVerifyReport, error) {
	h, err := store.Height()
	if err != nil {
		return nil, fmt.Errorf("读取存储高度失败: %w", err)
	}

	rep := &ChainVerifyReport{Height: h, FailHeight: -1}
	if h < 0 {
		rep.Reason = "本地链为空（数据目录中没有区块可校验）"
		return rep, nil
	}

	first, err := store.GetBlockByHeight(0)
	if err != nil {
		rep.FailHeight = 0
		rep.Reason = fmt.Sprintf("读取创世区块失败: %v", err)
		return rep, nil
	}
	// 注：创世区块按既有设计视为可信输入（见 NewBlockchainWithGenesis 注释），
	// 本函数不新增创世侧校验规则 —— 这里只用它建立初始 UTXO 状态。
	bc, err := NewBlockchainWithGenesis(first)
	if err != nil {
		rep.FailHeight = 0
		rep.FailHash = first.Header.HashHex()
		rep.Reason = err.Error()
		return rep, nil
	}
	rep.GenesisHash = first.Header.HashHex()
	rep.Blocks = 1

	for i := 1; i <= h; i++ {
		b, err := store.GetBlockByHeight(i)
		if err != nil {
			rep.FailHeight = i
			rep.Reason = fmt.Sprintf("读取高度 %d 区块失败: %v", i, err)
			return rep, nil
		}
		if err := bc.applyBlock(b); err != nil {
			rep.FailHeight = i
			rep.FailHash = b.Header.HashHex()
			rep.Reason = err.Error()
			return rep, nil
		}
		rep.Blocks++
	}

	tip, err := bc.Tip()
	if err != nil {
		return nil, fmt.Errorf("读取链尾失败: %w", err)
	}
	rep.TipHash = tip.Header.HashHex()
	rep.Valid = true
	return rep, nil
}
