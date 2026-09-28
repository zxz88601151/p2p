package blockchain_test

// AUTH-2-C1 §9：Test D 场景 event stream 采集器（C-1 前后逐字段对比用）。
//
// 两种运行模式（复用 I0 的 buildScenario/runScenario，12 步确定性场景）：
//
//   - P2PCHAIN_C1_FREEZE=path：挖矿构造场景 → runScenario → 把每个 obsOp 的块
//     经 block.Encode() 冻结到 path（JSONL，base64），同时 dump stream 到
//     P2PCHAIN_C1_DUMP（如设置）。
//   - P2PCHAIN_C1_THAW=path：从冻结文件 DecodeBlock 重建同一批块对象（逐字节
//     相同的输入）→ runScenario → dump stream。跨进程重放保证 before/after
//     对比时块哈希完全一致，隔离「实现差异」与「场景随机性」。
//
// 本测试不引用 C-1 index，行为与 C-1 无关——仅提供 event stream 对照证据。

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/obs"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
)

type frozenOp struct {
	Label    string `json:"label"`
	Action   string `json:"action"`
	BlockB64 string `json:"block_b64"`
}

func TestC1DumpEventStream(t *testing.T) {
	t.Cleanup(func() { obs.Disable() })

	freezePath := os.Getenv("P2PCHAIN_C1_FREEZE")
	thawPath := os.Getenv("P2PCHAIN_C1_THAW")

	var ops []obsOp
	switch {
	case thawPath != "":
		data, err := os.ReadFile(thawPath)
		if err != nil {
			t.Fatalf("读取冻结场景失败: %v", err)
		}
		var lines []frozenOp
		if err := json.Unmarshal(data, &lines); err != nil {
			t.Fatalf("解析冻结场景失败: %v", err)
		}
		for i, fo := range lines {
			raw, err := base64.StdEncoding.DecodeString(fo.BlockB64)
			if err != nil {
				t.Fatalf("冻结块 %d base64 解码失败: %v", i, err)
			}
			blk, err := block.DecodeBlock(raw)
			if err != nil {
				t.Fatalf("冻结块 %d 解码失败: %v", i, err)
			}
			ops = append(ops, obsOp{label: fo.Label, action: fo.Action, blk: blk})
		}
	default:
		bits := uint32(pow.MaxTargetBits)
		store0, err := storage.OpenFileBlockStore(t.TempDir())
		if err != nil {
			t.Fatalf("打开探针存储失败: %v", err)
		}
		bc0, err := blockchain.NewBlockchainFromStoreForTest(store0)
		if err != nil {
			t.Fatalf("探针链加载失败: %v", err)
		}
		genesisHash := mustTip(t, bc0).Header.Hash()
		_ = store0.Close()
		ops = buildScenario(t, genesisHash, bits)
	}

	steps, events := runScenario(t, ops, true)
	assertEventPresent(t, events, "REORG_REJECT", "REORG_ACCEPT", "FORK_VALIDATION_START", "FORK_VALIDATION_END")

	dumpPath := os.Getenv("P2PCHAIN_C1_DUMP")
	if dumpPath != "" {
		if err := os.WriteFile(dumpPath, []byte(events), 0o644); err != nil {
			t.Fatalf("dump event stream 失败: %v", err)
		}
		t.Logf("event stream dumped: %s (%d bytes, %d steps)", dumpPath, len(events), len(steps))
	}

	if freezePath != "" {
		out := make([]frozenOp, 0, len(ops))
		for _, op := range ops {
			out = append(out, frozenOp{
				Label:    op.label,
				Action:   op.action,
				BlockB64: base64.StdEncoding.EncodeToString(op.blk.Encode()),
			})
		}
		data, err := json.MarshalIndent(out, "", " ")
		if err != nil {
			t.Fatalf("冻结场景序列化失败: %v", err)
		}
		if err := os.WriteFile(freezePath, data, 0o644); err != nil {
			t.Fatalf("冻结场景写入失败: %v", err)
		}
		t.Logf("scenario frozen: %s (%d ops)", freezePath, len(ops))
	}
}
