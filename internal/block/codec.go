// 区块与交易的规范二进制编解码（用于磁盘持久化与网络传输的确定性格式）。
//
// 设计要点：
//   - 定长字段用固定宽度小端编码；变长字段（切片）前置 uint32 长度前缀；
//   - 字段顺序与结构体定义一致，跨平台/跨节点可复现；
//   - Decode 对长度与上限做防御性检查，避免恶意数据造成超大分配（DoS）。
package block

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"p2pchain/internal/transaction"
)

// 编解码限制（防御恶意输入）。
const (
	maxTxPerBlock = 100_000
	maxInPerTx    = 10_000
	maxOutPerTx   = 10_000
	maxScriptLen  = 10_000
)

// 错误复用 transaction 包的截断/超限语义，便于上层统一处理
var (
	ErrDecodeTruncated = transaction.ErrTxDecodeTruncated
	ErrDecodeTooLarge  = transaction.ErrTxDecodeTooLarge
)

// Encode 将区块编码为规范字节流。
func (b *Block) Encode() []byte {
	buf := new(bytes.Buffer)
	buf.Write(b.Header.SerializeHeader())

	_ = binary.Write(buf, binary.LittleEndian, uint32(len(b.Transactions)))
	for _, tx := range b.Transactions {
		encodeTx(buf, tx)
	}
	return buf.Bytes()
}

// Size 返回区块规范编码后的字节数（用于体积上限校验与存储）。
func (b *Block) Size() int { return len(b.Encode()) }

// DecodeBlock 解析规范字节流为区块。
func DecodeBlock(data []byte) (*Block, error) {
	r := bytes.NewReader(data)
	b := &Block{}
	if err := decodeHeader(r, &b.Header); err != nil {
		return nil, err
	}
	n, err := readU32(r)
	if err != nil {
		return nil, err
	}
	if n > maxTxPerBlock {
		return nil, fmt.Errorf("%w: 交易数 %d", ErrDecodeTooLarge, n)
	}
	b.Transactions = make([]*transaction.Transaction, 0, n)
	for i := uint32(0); i < n; i++ {
		tx, err := decodeTx(r)
		if err != nil {
			return nil, fmt.Errorf("第 %d 笔交易解析失败: %w", i, err)
		}
		b.Transactions = append(b.Transactions, tx)
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("区块数据尾部有 %d 字节多余内容", r.Len())
	}
	return b, nil
}

// ---- 头部 ----

// SerializeHeader 见 block.go（定长 80 字节：4+32+32+8+4+8，但 Go 结构体对齐不参与，
// 这里按字段逐个小端写入，实际长度 = 4+32+32+8+4+8 = 88 字节）。

func decodeHeader(r io.Reader, h *Header) error {
	if err := binary.Read(r, binary.LittleEndian, &h.Version); err != nil {
		return ErrDecodeTruncated
	}
	if _, err := io.ReadFull(r, h.PrevBlockHash[:]); err != nil {
		return ErrDecodeTruncated
	}
	if _, err := io.ReadFull(r, h.MerkleRoot[:]); err != nil {
		return ErrDecodeTruncated
	}
	if err := binary.Read(r, binary.LittleEndian, &h.Timestamp); err != nil {
		return ErrDecodeTruncated
	}
	if err := binary.Read(r, binary.LittleEndian, &h.Bits); err != nil {
		return ErrDecodeTruncated
	}
	if err := binary.Read(r, binary.LittleEndian, &h.Nonce); err != nil {
		return ErrDecodeTruncated
	}
	return nil
}

// ---- 交易 ----（编解码实现位于 transaction 包，区块与网络共用同一格式）

func encodeTx(buf *bytes.Buffer, tx *transaction.Transaction) {
	buf.Write(tx.Encode())
}

func decodeTx(r *bytes.Reader) (*transaction.Transaction, error) {
	return transaction.DecodeTxFrom(r)
}

// ---- 基础读写 ----

func readU32(r *bytes.Reader) (uint32, error) {
	var v uint32
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, ErrDecodeTruncated
	}
	return v, nil
}
