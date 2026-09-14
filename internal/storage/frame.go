package storage

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
)

// ════════════════════════════════════════════════════════════════════════════
// REORG-1E · M1 —— Record Frame v2（PCC2）
//
// 本文件实现 REORG-1E 冻结契约 F1/F2 规定的 v2 记录帧编解码，**仅此一处**定义
// 字节布局。所有解析路径 fail-stop：任何字段非法立即返回具名错误，绝不跳过、
// 绝不猜测、绝不在损坏点之后尝试重新同步。
//
// 帧布局（F2，总开销固定 80 B）：
//
//	偏移  长度  字段        说明
//	----  ----  ----------  ------------------------------------------------
//	  0     4   magic       'P','C','C','2'（见下）
//	  4     1   version     固定 2
//	  5     1   type        1=BLOCK 2=UNDO 3=TIP 4=DELETE
//	  6     2   reserved    必须为 0（小端）
//	  8     4   payloadLen  载荷字节数（小端）
//	 12     4   height      逻辑高度（小端）
//	 16    32   blockHash   本记录所绑定区块的双 SHA-256 哈希
//	 48     n   payload     类型相关载荷
//	 48+n  32   checksum    SHA256(magic||version||type||reserved||payloadLen||height||blockHash||payload)
//
// 帧头 48 B + 校验和 32 B = 80 B（F2 冻结值）。
//
// 魔数与 legacy 的零迁移兼容（F1）：`'P','C','C','2'` 按小端 u32 解析 =
// 843,268,944，**大于** legacy 记录长度上限 64 MiB。因此一天也不迁移：
// 旧（v1/legacy）读者遇到 v2 记录必然以「非法记录长度」fail-stop——这正是
// 「v2 记录不可能被 legacy 读者误当成合法记录」的结构性保证，也是 M2 无需
// 迁移历史字节的前提。
//
// 端序说明：除 magic 需按小端 u32 语义解读（F1 反误解析设计）外，帧内多字节
// 整数统一小端，与 legacy 前缀的 `[u32 LE len]` 保持一致，便于人工排查。
// 本阶段为**纯本地非共识**阶段（审计 §10.1），帧字节布局不可能引发网络分叉。
// ════════════════════════════════════════════════════════════════════════════

const (
	// frameMagic v2 记录魔数。按小端 u32 = 843,268,944 > 64 MiB 长度上限。
	frameMagic = "PCC2"

	// frameVersion 本实现唯一支持的帧版本。
	frameVersion = 2

	// 帧固定尺寸。
	frameHeaderSize   = 48 // magic(4)+version(1)+type(1)+reserved(2)+payloadLen(4)+height(4)+blockHash(32)
	frameChecksumSize = 32
	frameOverhead     = frameHeaderSize + frameChecksumSize // 80（F2）

	// maxPayloadLen 载荷长度上限。与 legacy 长度上限同值（64 MiB），
	// 使「非法长度」的判定在两个语义区完全一致。
	maxPayloadLen = 64 << 20
)

// 记录类型（type 字段取值）。
const (
	recTypeBlock  byte = 1 // payload = block.Encode()
	recTypeUndo   byte = 2 // payload = utxo.EncodeUndo(undo)
	recTypeTip    byte = 3 // payload = tipPayload（40 B 定长）
	recTypeDelete byte = 4 // payload = 32 B 目标区块哈希（逻辑删除墓碑）
)

// tipPayloadLen TIP 载荷定长：chainwork(32 BE) + tipHeight(4 LE) + reserved(4 LE)。
const tipPayloadLen = 40

// ── 具名错误（全部 fail-stop 语义）─────────────────────────────────────────

var (
	// ErrBadRecordMagic 帧魔数不是 "PCC2"。
	ErrBadRecordMagic = errors.New("记录魔数非法（不是 PCC2）")
	// ErrUnsupportedRecordVersion 帧版本不受支持（仅支持 2）。
	ErrUnsupportedRecordVersion = errors.New("不支持的记录版本")
	// ErrInvalidRecordType 记录类型不在 {1,2,3,4} 内。
	ErrInvalidRecordType = errors.New("非法的记录类型")
	// ErrNonZeroReserved 保留位非零。
	ErrNonZeroReserved = errors.New("记录 reserved 字段必须为 0")
	// ErrInvalidPayloadLen 载荷长度为 0 或超过 64 MiB。
	ErrInvalidPayloadLen = errors.New("非法的记录载荷长度")
	// ErrTruncatedHeader 帧头不足 48 B。
	ErrTruncatedHeader = errors.New("记录帧头被截断")
	// ErrTruncatedPayload 载荷不完整。
	ErrTruncatedPayload = errors.New("记录载荷被截断")
	// ErrMissingChecksum 载荷完整但校验和缺失。
	ErrMissingChecksum = errors.New("记录校验和缺失")
	// ErrChecksumMismatch 校验和与内容不一致。
	ErrChecksumMismatch = errors.New("记录校验和不匹配")
	// ErrInvalidTipPayload TIP 载荷长度或内容非法。
	ErrInvalidTipPayload = errors.New("非法的 TIP 载荷")
	// ErrChainworkOverflow 累积工作量超出 256 bit，无法写入 32 B 字段。
	ErrChainworkOverflow = errors.New("累积工作量超出 256 bit")
	// ErrDeletePayloadLen DELETE 载荷必须恰为 32 B。
	ErrDeletePayloadLen = errors.New("DELETE 载荷长度必须为 32")
)

// validRecType 判定 type 字段是否为已知记录类型。
func validRecType(t byte) bool {
	switch t {
	case recTypeBlock, recTypeUndo, recTypeTip, recTypeDelete:
		return true
	default:
		return false
	}
}

// recTypeName 返回记录类型的人类可读名（诊断/报告用）。
func recTypeName(t byte) string {
	switch t {
	case recTypeBlock:
		return "BLOCK"
	case recTypeUndo:
		return "UNDO"
	case recTypeTip:
		return "TIP"
	case recTypeDelete:
		return "DELETE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", t)
	}
}

// hasFrameMagic 报告 buf 起始 4 字节是否为 v2 魔数。
// 这是 legacy / v2 两种记录语义的唯一判别点：legacy 长度不可能等于该魔数
// （见 frameMagic 注释），故判别无歧义。
func hasFrameMagic(buf []byte) bool {
	return len(buf) >= 4 &&
		buf[0] == 'P' && buf[1] == 'C' && buf[2] == 'C' && buf[3] == '2'
}

// frameChecksum 计算 SHA256(magic||version||type||reserved||payloadLen||height||blockHash||payload)。
// 入参 head 即「帧头 + 载荷」的连续字节（长度 = 48 + n）。
func frameChecksum(head []byte) [32]byte {
	return sha256.Sum256(head)
}

// frameInfo 一枚已成功解析并通过全部校验的 v2 帧。
type frameInfo struct {
	Type      byte
	Height    uint32
	BlockHash [32]byte
	Payload   []byte
	TotalLen  int // frameOverhead + len(Payload)
}

// encodeFrame 按 F2 布局序列化一枚 v2 帧（含校验和）。
// 调用方负责保证 payload 语义与该 type 匹配（见 encodeTipPayload 等）。
func encodeFrame(typ byte, height uint32, blockHash [32]byte, payload []byte) []byte {
	buf := make([]byte, frameOverhead+len(payload))
	copy(buf[0:4], frameMagic)
	buf[4] = frameVersion
	buf[5] = typ
	binary.LittleEndian.PutUint16(buf[6:8], 0) // reserved
	binary.LittleEndian.PutUint32(buf[8:12], uint32(len(payload)))
	binary.LittleEndian.PutUint32(buf[12:16], height)
	copy(buf[16:48], blockHash[:])
	copy(buf[48:48+len(payload)], payload)
	sum := frameChecksum(buf[:frameHeaderSize+len(payload)])
	copy(buf[frameHeaderSize+len(payload):], sum[:])
	return buf
}

// decodeFrame 解析一枚完整帧并执行全部 fail-stop 校验。
//
// 入参 b 必须是从帧起始偏移开始的字节切片，长度需 ≥ 帧实际长度；
// 多出的尾部字节被忽略（调用方通常只传入恰好一枚帧）。
// 校验顺序：帧头长度 → magic → version → type → reserved → payloadLen →
// 载荷完整性 → 校验和存在性 → 校验和一致性。
func decodeFrame(b []byte) (frameInfo, error) {
	var f frameInfo
	if len(b) < frameHeaderSize {
		return f, ErrTruncatedHeader
	}
	if !hasFrameMagic(b) {
		return f, ErrBadRecordMagic
	}
	if b[4] != frameVersion {
		return f, fmt.Errorf("%w: got %d want %d", ErrUnsupportedRecordVersion, b[4], frameVersion)
	}
	if !validRecType(b[5]) {
		return f, fmt.Errorf("%w: %d", ErrInvalidRecordType, b[5])
	}
	if binary.LittleEndian.Uint16(b[6:8]) != 0 {
		return f, ErrNonZeroReserved
	}
	n := binary.LittleEndian.Uint32(b[8:12])
	if n == 0 || n > maxPayloadLen {
		return f, fmt.Errorf("%w: %d", ErrInvalidPayloadLen, n)
	}
	need := uint64(frameOverhead) + uint64(n)
	if uint64(len(b)) < need {
		if uint64(len(b)) < uint64(frameHeaderSize)+uint64(n) {
			return f, ErrTruncatedPayload
		}
		return f, ErrMissingChecksum
	}
	body := b[:frameHeaderSize+int(n)]
	want := frameChecksum(body)
	var got [32]byte
	copy(got[:], b[frameHeaderSize+int(n):frameHeaderSize+int(n)+frameChecksumSize])
	if got != want {
		return f, ErrChecksumMismatch
	}
	f.Type = b[5]
	f.Height = binary.LittleEndian.Uint32(b[12:16])
	copy(f.BlockHash[:], b[16:48])
	f.Payload = body[frameHeaderSize:]
	f.TotalLen = int(need)
	return f, nil
}

// ── M5 TIP 载荷编解码 ──────────────────────────────────────────────────────

// tipPayload TIP 记录的载荷。
//
// 布局（F2/M5）：chainwork 32 B（**大端**，左零填充）+ tipHeight 4 B（小端）
// + reserved 4 B（小端，必须 0）。
type tipPayload struct {
	Chainwork [32]byte
	Height    uint32
}

// encodeTipPayload 序列化 TIP 载荷。累积工作量超过 256 bit 时拒绝写入
// （不可能出现在本链参数范围内，但绝不静默截断）。
func encodeTipPayload(cw *big.Int, height int) ([]byte, error) {
	if cw == nil {
		cw = big.NewInt(0)
	}
	if cw.Sign() < 0 || cw.BitLen() > 256 {
		return nil, fmt.Errorf("%w: bitlen=%d", ErrChainworkOverflow, cw.BitLen())
	}
	if height < 0 || height > int(^uint32(0)) {
		return nil, fmt.Errorf("%w: height=%d", ErrInvalidTipPayload, height)
	}
	p := make([]byte, tipPayloadLen)
	cw.FillBytes(p[0:32]) // 大端，左零填充
	binary.LittleEndian.PutUint32(p[32:36], uint32(height))
	binary.LittleEndian.PutUint32(p[36:40], 0)
	return p, nil
}

// decodeTipPayload 还原 TIP 载荷并校验 reserved == 0。
func decodeTipPayload(b []byte) (tipPayload, error) {
	var t tipPayload
	if len(b) != tipPayloadLen {
		return t, fmt.Errorf("%w: len=%d", ErrInvalidTipPayload, len(b))
	}
	copy(t.Chainwork[:], b[0:32])
	t.Height = binary.LittleEndian.Uint32(b[32:36])
	if binary.LittleEndian.Uint32(b[36:40]) != 0 {
		return t, ErrNonZeroReserved
	}
	return t, nil
}
