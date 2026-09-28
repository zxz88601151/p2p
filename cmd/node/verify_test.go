package main

// 本文件覆盖 PHASE BRAND-1.2 的 `verify` 离线只读校验：
//   - §6   正向：合法链 → PASS / exit 0
//   - §7   负向矩阵 N1–N5：篡改/截断/重复记录 → FAIL / exit != 0，且给出具体原因
//   - §8   幂等与非变异：verify 前后 blocks.dat 的 SHA-256 与大小不变，且不产生锁/钱包/新区块
//
// 所有用例都在 t.TempDir() 内的独立数据目录运行，不触碰 ~/.p2pchain。

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"p2pchain/internal/block"
	"p2pchain/internal/blockchain"
	"p2pchain/internal/pow"
	"p2pchain/internal/storage"
	"p2pchain/internal/transaction"
	"p2pchain/internal/utxo"
)

// ---- 测试夹具 ----

// buildChainForVerify 在 dir 中建立一条合法的确定性链（创世 + n 个 PoW 区块）。
func buildChainForVerify(t *testing.T, dir string, n int) {
	t.Helper()
	store, err := storage.OpenFileBlockStore(dir)
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	chain, err := blockchain.NewBlockchainFromStoreForTest(store)
	if err != nil {
		t.Fatalf("初始化链失败: %v", err)
	}
	var pkh [20]byte
	pkh[0] = 0xAA
	for i := 0; i < n; i++ {
		tip, err := chain.Tip()
		if err != nil {
			t.Fatalf("读取链尾失败: %v", err)
		}
		height := chain.Height() + 1
		cb := transaction.NewCoinbaseTx(pkh, utxo.Subsidy(height), height)
		b := block.NewCandidateBlock(tip.Header.Hash(), pow.MaxTargetBits,
			[]*transaction.Transaction{cb})
		if found, _ := pow.Mine(b); !found {
			t.Fatalf("高度 %d 挖矿失败", height)
		}
		if err := chain.AddBlock(b); err != nil {
			t.Fatalf("追加高度 %d 区块失败: %v", height, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("关闭存储失败: %v", err)
	}
}

// runVerify 以 JSON 模式执行 verify，返回解析后的报告与退出码。
func runVerify(t *testing.T, dir string) (blockchain.ChainVerifyReport, int, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code := cmdVerify([]string{"-datadir", dir, "-json"}, &out, &errBuf)
	var rep blockchain.ChainVerifyReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("校验报告不是合法 JSON（exit=%d, stderr=%s, stdout=%s）: %v",
			code, errBuf.String(), out.String(), err)
	}
	return rep, code, out.String() + errBuf.String()
}

// fileDigest 返回文件内容的 SHA-256 与字节大小。
func fileDigest(t *testing.T, path string) (string, int64) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), int64(len(data))
}

// recordSpans 解析 blocks.dat 的「4 字节小端长度前缀 + 载荷」记录布局，
// 返回每条记录载荷的 [起始偏移, 长度]。
func recordSpans(t *testing.T, data []byte) [][2]int {
	t.Helper()
	var spans [][2]int
	pos := 0
	for pos+4 <= len(data) {
		n := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
		pos += 4
		if n < 0 || pos+n > len(data) {
			t.Fatalf("blocks.dat 记录长度越界: 偏移 %d 长度 %d（文件 %d 字节）", pos-4, n, len(data))
		}
		spans = append(spans, [2]int{pos, n})
		pos += n
	}
	if len(spans) == 0 {
		t.Fatal("blocks.dat 中没有任何记录")
	}
	return spans
}

// flipByteAt 翻转文件中指定偏移的一个字节（制造人为篡改）。
func flipByteAt(t *testing.T, path string, off int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if off < 0 || off >= len(data) {
		t.Fatalf("偏移 %d 越界（文件 %d 字节）", off, len(data))
	}
	data[off] ^= 0x01
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("写回文件失败: %v", err)
	}
}

// ---- §6 正向测试 ----

func TestVerifyValidChainPasses(t *testing.T) {
	dir := t.TempDir()
	buildChainForVerify(t, dir, 3)

	rep, code, out := runVerify(t, dir)
	if code != 0 {
		t.Fatalf("合法链 verify 退出码 = %d，期望 0（输出: %s）", code, out)
	}
	if !rep.Valid {
		t.Fatalf("合法链 verify 结论 = false，期望 true（原因: %s）", rep.Reason)
	}
	if rep.Height != 3 {
		t.Errorf("Height = %d，期望 3", rep.Height)
	}
	if rep.Blocks != 4 {
		t.Errorf("Blocks = %d，期望 4（创世 + 3）", rep.Blocks)
	}
	if rep.GenesisHash != "00003d97723c3cccec83a664f5d22da6f66dfa72c9f28b286c746f4bc4dce4a3" {
		t.Errorf("创世哈希 = %s，与确定性创世不符", rep.GenesisHash)
	}
	if rep.TipHash == "" || rep.FailHeight != -1 || rep.Reason != "" {
		t.Errorf("通过时不应带失败信息: FailHeight=%d Reason=%q", rep.FailHeight, rep.Reason)
	}
}

func TestVerifyHumanReadableOutputMentionsPass(t *testing.T) {
	dir := t.TempDir()
	buildChainForVerify(t, dir, 1)

	var out, errBuf bytes.Buffer
	code := cmdVerify([]string{"-datadir", dir}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("退出码 = %d，期望 0（stderr: %s）", code, errBuf.String())
	}
	text := out.String()
	for _, want := range []string{"PASS", "创世哈希", "链尾哈希"} {
		if !strings.Contains(text, want) {
			t.Errorf("人类可读输出缺少 %q:\n%s", want, text)
		}
	}
}

// ---- §8 非变异 / 幂等 ----

func TestVerifyDoesNotMutateAnything(t *testing.T) {
	dir := t.TempDir()
	buildChainForVerify(t, dir, 2)

	blocksPath := filepath.Join(dir, "blocks.dat")
	hashBefore, sizeBefore := fileDigest(t, blocksPath)

	rep1, code1, _ := runVerify(t, dir)
	if code1 != 0 || !rep1.Valid {
		t.Fatalf("第一次 verify 失败: exit=%d valid=%v reason=%s", code1, rep1.Valid, rep1.Reason)
	}
	hashAfter, sizeAfter := fileDigest(t, blocksPath)
	if hashBefore != hashAfter {
		t.Errorf("blocks.dat 内容被修改:\n before=%s\n after =%s", hashBefore, hashAfter)
	}
	if sizeBefore != sizeAfter {
		t.Errorf("blocks.dat 大小变化: %d → %d", sizeBefore, sizeAfter)
	}

	// 幂等：第二次执行结果完全一致
	rep2, code2, _ := runVerify(t, dir)
	if code2 != code1 || rep2 != rep1 {
		t.Errorf("verify 非幂等: code %d→%d, report %+v → %+v", code1, code2, rep1, rep2)
	}

	// 不产生任何副作用文件：不取锁、不建钱包、不产生新区块
	for _, forbidden := range []string{"node.lock", "wallet.json"} {
		if _, err := os.Stat(filepath.Join(dir, forbidden)); err == nil {
			t.Errorf("verify 产生了 %s（不应创建：verify 不取锁、不加载钱包）", forbidden)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取数据目录失败: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "blocks.dat" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("数据目录出现预期外文件: %v（应只有 blocks.dat）", names)
	}
}

// ---- §7 负向矩阵 ----

// N1 — block corruption：篡改最后一条记录的区块结构（交易计数字段）
func TestVerifyDetectsBlockCorruptionN1(t *testing.T) {
	dir := t.TempDir()
	buildChainForVerify(t, dir, 2)
	blocksPath := filepath.Join(dir, "blocks.dat")

	data, err := os.ReadFile(blocksPath)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	spans := recordSpans(t, data)
	last := spans[len(spans)-1]
	// 载荷布局：88 字节区块头 + 4 字节交易计数 → 偏移 88 即交易计数字段
	flipByteAt(t, blocksPath, last[0]+88)

	assertVerifyFails(t, dir, "N1 区块结构篡改")
}

// N2 — header corruption：篡改最后一条记录区块头中的 Merkle 根（偏移 4+32=36 起）
func TestVerifyDetectsHeaderCorruptionN2(t *testing.T) {
	dir := t.TempDir()
	buildChainForVerify(t, dir, 2)
	blocksPath := filepath.Join(dir, "blocks.dat")

	data, err := os.ReadFile(blocksPath)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	spans := recordSpans(t, data)
	last := spans[len(spans)-1]
	flipByteAt(t, blocksPath, last[0]+40) // MerkleRoot 内部

	dirOut, code, _ := verifyRaw(t, dir)
	if code == 0 {
		t.Fatalf("N2 区块头篡改后 verify 仍通过（输出: %s）", dirOut)
	}
	// 区块头参与哈希，因此篡改 Merkle 根会先在第 2 步（PoW）被拦截——
	// 这不是缺陷，而是校验全序的必然结果：PrevHash → PoW → Bits → 时间戳 → Merkle。
	// Merkle 重验路径由 N3（只改交易、不改头）覆盖。
	if !strings.Contains(dirOut, "工作量证明无效") {
		t.Errorf("N2 期望给出 PoW 相关具名原因，实际输出:\n%s", dirOut)
	}
}

// N3 — transaction corruption：篡改 coinbase 交易内容（输出区域）
func TestVerifyDetectsTransactionCorruptionN3(t *testing.T) {
	dir := t.TempDir()
	buildChainForVerify(t, dir, 2)
	blocksPath := filepath.Join(dir, "blocks.dat")

	data, err := os.ReadFile(blocksPath)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	spans := recordSpans(t, data)
	last := spans[len(spans)-1]
	// 载荷布局：头 88 + 交易计数 4 + 输入计数 4 + PrevTxHash 32 + OutIndex 4
	//           + sig 长度 4 + sig 4 + pubkey 长度 4 + 输出计数 4 + Value 8 ...
	// 偏移 148 落在一笔 coinbase 的输出金额（u64）区域
	flipByteAt(t, blocksPath, last[0]+150)

	dirOut, code, _ := verifyRaw(t, dir)
	if code == 0 {
		t.Fatalf("N3 交易篡改后 verify 仍通过（输出: %s）", dirOut)
	}
	// 交易被改 → TxID 变化 → Merkle 重验先于状态迁移失败（共识校验全序第 5 步）
	if !strings.Contains(dirOut, "Merkle") {
		t.Errorf("N3 期望 Merkle 重验拦截，实际输出:\n%s", dirOut)
	}
}

// N4 — truncated storage：截断 blocks.dat
func TestVerifyDetectsTruncatedStorageN4(t *testing.T) {
	dir := t.TempDir()
	buildChainForVerify(t, dir, 2)
	blocksPath := filepath.Join(dir, "blocks.dat")

	data, err := os.ReadFile(blocksPath)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if err := os.WriteFile(blocksPath, data[:len(data)-16], 0o644); err != nil {
		t.Fatalf("截断写入失败: %v", err)
	}

	assertVerifyFails(t, dir, "N4 存储截断")
}

// N5 — invalid / inconsistent chain state：把最后一条区块记录再追加一遍，
// 使新记录的 PrevBlockHash 与当前链尾不匹配（触发具名错误 ErrInvalidPrevHash）
func TestVerifyDetectsInconsistentChainStateN5(t *testing.T) {
	dir := t.TempDir()
	buildChainForVerify(t, dir, 2)
	blocksPath := filepath.Join(dir, "blocks.dat")

	data, err := os.ReadFile(blocksPath)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	spans := recordSpans(t, data)
	last := spans[len(spans)-1]
	rec := data[last[0]-4 : last[0]+last[1]] // 含 4 字节长度前缀的完整记录
	dup := append([]byte{}, data...)
	dup = append(dup, rec...)
	if err := os.WriteFile(blocksPath, dup, 0o644); err != nil {
		t.Fatalf("写入重复记录失败: %v", err)
	}

	out, code, _ := verifyRaw(t, dir)
	if code == 0 {
		t.Fatalf("N5 重复区块记录后 verify 仍通过（输出: %s）", out)
	}
	if !strings.Contains(out, "前置哈希与当前链尾不匹配") {
		t.Errorf("N5 期望具名错误 ErrInvalidPrevHash，实际输出:\n%s", out)
	}
}

// ---- 辅助断言 ----

// verifyRaw 以人类可读模式执行 verify，返回合并输出与退出码。
func verifyRaw(t *testing.T, dir string) (string, int, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code := cmdVerify([]string{"-datadir", dir}, &out, &errBuf)
	return out.String(), code, errBuf.String()
}

// assertVerifyFails 断言 verify 以非零码失败，且原因非泛化。
func assertVerifyFails(t *testing.T, dir, scenario string) {
	t.Helper()
	out, code, errOut := verifyRaw(t, dir)
	if code == 0 {
		t.Fatalf("%s：verify 仍返回 0（输出: %s）", scenario, out)
	}
	combined := out + errOut
	if strings.TrimSpace(combined) == "" {
		t.Fatalf("%s：verify 失败但未给出任何原因", scenario)
	}
	if strings.Contains(combined, "invalid\"") && len(strings.TrimSpace(combined)) < 20 {
		t.Errorf("%s：失败原因是泛化的 invalid，不合格", scenario)
	}
}
