package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type BitWriter struct {
	Buffer bytes.Buffer
}

func NewBitWriter() *BitWriter {
	return &BitWriter{}
}

// WriteBits 简化实现，假设输入数据总是按字节对齐（符合本 DSL 的 YAML 定义）
func (w *BitWriter) WriteBits(value uint64, n int, byteOrder binary.ByteOrder) error {
	if n%8 != 0 {
		return fmt.Errorf("bit field size %d is not byte-aligned for encoding", n)
	}
	numBytes := n / 8

	buf := make([]byte, 8)
	byteOrder.PutUint64(buf, value)

	// 写入最后 numBytes 个字节
	switch byteOrder {
	case binary.BigEndian:
		w.Buffer.Write(buf[8-numBytes:])
	case binary.LittleEndian:
		w.Buffer.Write(buf[:numBytes])
	}
	return nil
}

// WriteBytes 写入完整的字节
func (w *BitWriter) WriteBytes(data []byte) error {
	_, err := w.Buffer.Write(data)
	return err
}

func (w *BitWriter) Set(data []byte, offset int) error {
	if offset < 0 || offset+len(data) > w.Buffer.Len() {
		return fmt.Errorf("invalid set offset or length")
	}
	copy(w.Buffer.Bytes()[offset:offset+len(data)], data)
	return nil
}
