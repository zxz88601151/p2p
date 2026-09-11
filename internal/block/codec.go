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
	"errors"
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

var (
	ErrDecodeTruncated = errors.New("区块数据被截断")
	ErrDecodeTooLarge  = errors.New("区块数据超出解码上限")
)

// Encode 将区块编码为规范字节流。
func (b *Block) Encode() []byte {
	buf := new(bytes.Buffer)
	buf.Write(b.Header.SerializeHeader())

	writeU32(buf, uint32(len(b.Transactions)))
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

// ---- 交易 ----

func encodeTx(buf *bytes.Buffer, tx *transaction.Transaction) {
	writeU32(buf, uint32(len(tx.Inputs)))
	for _, in := range tx.Inputs {
		buf.Write(in.PrevTxHash[:])
		writeU32(buf, in.OutIndex)
		writeBytes(buf, in.Signature)
		writeBytes(buf, in.PubKey)
	}
	writeU32(buf, uint32(len(tx.Outputs)))
	for _, out := range tx.Outputs {
		writeU64(buf, out.Value)
		buf.Write(out.PubKeyHash[:])
	}
}

func decodeTx(r *bytes.Reader) (*transaction.Transaction, error) {
	tx := &transaction.Transaction{}

	nIn, err := readU32(r)
	if err != nil {
		return nil, err
	}
	if nIn > maxInPerTx {
		return nil, fmt.Errorf("%w: 输入数 %d", ErrDecodeTooLarge, nIn)
	}
	if nIn > 0 {
		tx.Inputs = make([]transaction.TxInput, 0, nIn)
	}
	for i := uint32(0); i < nIn; i++ {
		var in transaction.TxInput
		if _, err := io.ReadFull(r, in.PrevTxHash[:]); err != nil {
			return nil, ErrDecodeTruncated
		}
		if in.OutIndex, err = readU32(r); err != nil {
			return nil, err
		}
		if in.Signature, err = readBytes(r, maxScriptLen); err != nil {
			return nil, err
		}
		if in.PubKey, err = readBytes(r, maxScriptLen); err != nil {
			return nil, err
		}
		tx.Inputs = append(tx.Inputs, in)
	}

	nOut, err := readU32(r)
	if err != nil {
		return nil, err
	}
	if nOut > maxOutPerTx {
		return nil, fmt.Errorf("%w: 输出数 %d", ErrDecodeTooLarge, nOut)
	}
	if nOut > 0 {
		tx.Outputs = make([]transaction.TxOutput, 0, nOut)
	}
	for i := uint32(0); i < nOut; i++ {
		var out transaction.TxOutput
		if out.Value, err = readU64(r); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(r, out.PubKeyHash[:]); err != nil {
			return nil, ErrDecodeTruncated
		}
		tx.Outputs = append(tx.Outputs, out)
	}
	return tx, nil
}

// ---- 基础读写 ----

func writeU32(buf *bytes.Buffer, v uint32) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}

func writeU64(buf *bytes.Buffer, v uint64) {
	_ = binary.Write(buf, binary.LittleEndian, v)
}

func writeBytes(buf *bytes.Buffer, b []byte) {
	writeU32(buf, uint32(len(b)))
	buf.Write(b)
}

func readU32(r *bytes.Reader) (uint32, error) {
	var v uint32
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, ErrDecodeTruncated
	}
	return v, nil
}

func readU64(r *bytes.Reader) (uint64, error) {
	var v uint64
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, ErrDecodeTruncated
	}
	return v, nil
}

func readBytes(r *bytes.Reader, limit uint32) ([]byte, error) {
	n, err := readU32(r)
	if err != nil {
		return nil, err
	}
	if n > limit {
		return nil, fmt.Errorf("%w: 字段长度 %d > %d", ErrDecodeTooLarge, n, limit)
	}
	if n == 0 {
		return nil, nil
	}
	if uint32(r.Len()) < n {
		return nil, ErrDecodeTruncated
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, ErrDecodeTruncated
	}
	return b, nil
}
