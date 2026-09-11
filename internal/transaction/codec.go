// 交易的规范二进制编解码（网络传输与磁盘存储共用）。
package transaction

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// 解码防御上限（避免恶意数据造成超大分配）。
const (
	maxInputsPerTx  = 10_000
	maxOutputsPerTx = 10_000
	maxFieldLen     = 10_000
)

var (
	ErrTxDecodeTruncated = errors.New("交易数据被截断")
	ErrTxDecodeTooLarge  = errors.New("交易数据超出解码上限")
)

// Encode 将交易编码为规范字节流。
func (tx *Transaction) Encode() []byte {
	buf := new(bytes.Buffer)
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
	return buf.Bytes()
}

// Size 返回规范编码长度。
func (tx *Transaction) Size() int { return len(tx.Encode()) }

// DecodeTx 解析规范字节流为交易（要求无尾部多余字节）。
func DecodeTx(data []byte) (*Transaction, error) {
	r := bytes.NewReader(data)
	tx, err := DecodeTxFrom(r)
	if err != nil {
		return nil, err
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("交易数据尾部有 %d 字节多余内容", r.Len())
	}
	return tx, nil
}

// DecodeTxFrom 从字节流中读取一笔交易（不检查尾部剩余，供区块解码连续读取）。
func DecodeTxFrom(r *bytes.Reader) (*Transaction, error) {
	tx := &Transaction{}

	nIn, err := readU32(r)
	if err != nil {
		return nil, err
	}
	if nIn > maxInputsPerTx {
		return nil, fmt.Errorf("%w: 输入数 %d", ErrTxDecodeTooLarge, nIn)
	}
	for i := uint32(0); i < nIn; i++ {
		var in TxInput
		if _, err := io.ReadFull(r, in.PrevTxHash[:]); err != nil {
			return nil, ErrTxDecodeTruncated
		}
		if in.OutIndex, err = readU32(r); err != nil {
			return nil, err
		}
		if in.Signature, err = readBytes(r); err != nil {
			return nil, err
		}
		if in.PubKey, err = readBytes(r); err != nil {
			return nil, err
		}
		tx.Inputs = append(tx.Inputs, in)
	}

	nOut, err := readU32(r)
	if err != nil {
		return nil, err
	}
	if nOut > maxOutputsPerTx {
		return nil, fmt.Errorf("%w: 输出数 %d", ErrTxDecodeTooLarge, nOut)
	}
	for i := uint32(0); i < nOut; i++ {
		var out TxOutput
		if out.Value, err = readU64(r); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(r, out.PubKeyHash[:]); err != nil {
			return nil, ErrTxDecodeTruncated
		}
		tx.Outputs = append(tx.Outputs, out)
	}
	return tx, nil
}

func writeU32(buf *bytes.Buffer, v uint32) { _ = binary.Write(buf, binary.LittleEndian, v) }
func writeU64(buf *bytes.Buffer, v uint64) { _ = binary.Write(buf, binary.LittleEndian, v) }

func writeBytes(buf *bytes.Buffer, b []byte) {
	writeU32(buf, uint32(len(b)))
	buf.Write(b)
}

func readU32(r *bytes.Reader) (uint32, error) {
	var v uint32
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, ErrTxDecodeTruncated
	}
	return v, nil
}

func readU64(r *bytes.Reader) (uint64, error) {
	var v uint64
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, ErrTxDecodeTruncated
	}
	return v, nil
}

func readBytes(r *bytes.Reader) ([]byte, error) {
	n, err := readU32(r)
	if err != nil {
		return nil, err
	}
	if n > maxFieldLen {
		return nil, fmt.Errorf("%w: 字段长度 %d", ErrTxDecodeTooLarge, n)
	}
	if n == 0 {
		return nil, nil
	}
	if uint32(r.Len()) < n {
		return nil, ErrTxDecodeTruncated
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, ErrTxDecodeTruncated
	}
	return b, nil
}
