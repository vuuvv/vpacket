package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// LengthPatch 结构用于记录长度字段的位置，以便后续回填
type LengthPatch struct {
	Position int // 长度字段在缓冲区中的起始偏移
	Size     int // 长度字段的字节大小 (2 for uint16)
}

type BitWriter struct {
	Buffer        bytes.Buffer
	LengthPatches []LengthPatch
}

func NewBitWriter() *BitWriter {
	return &BitWriter{}
}

// WriteBits 简化实现，假设输入数据总是按字节对齐（符合本 DSL 的 YAML 定义）
func (w *BitWriter) WriteBits(value uint64, n int) error {
	if n%8 != 0 {
		return fmt.Errorf("bit field size %d is not byte-aligned for encoding", n)
	}
	numBytes := n / 8

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, value)

	// 写入最后 numBytes 个字节
	w.Buffer.Write(buf[8-numBytes:])
	return nil
}

// WriteBytes 写入完整的字节
func (w *BitWriter) WriteBytes(data []byte) error {
	_, err := w.Buffer.Write(data)
	return err
}

// WriteLengthPlaceholder 写入长度占位符并记录位置 (用于 data_len 字段)
func (w *BitWriter) WriteLengthPlaceholder(size int) error {
	w.LengthPatches = append(w.LengthPatches, LengthPatch{
		Position: w.Buffer.Len(),
		Size:     size,
	})
	// 写入 size 个零作为占位符
	_, err := w.Buffer.Write(bytes.Repeat([]byte{0x00}, size))
	return err
}

// PatchLength 回填长度字段 (在 EncodePacket 中调用)
func (w *BitWriter) PatchLength(bodyLen int, patch LengthPatch) {
	if bodyLen > 65535 {
		// 如果长度字段是 2 字节，但 bodyLen 超过了 uint16
		fmt.Printf("Warning: Length %d exceeds uint16 max for patch size %d\n", bodyLen, patch.Size)
	}

	// 创建一个新的缓冲区来放置回填后的数据
	finalBytes := w.Buffer.Bytes()

	// 写入实际长度值
	lengthBytes := make([]byte, patch.Size)

	// 假设长度字段总是 uint16 (2 bytes)
	if patch.Size == 2 {
		binary.BigEndian.PutUint16(lengthBytes, uint16(bodyLen))
	} else {
		// 简化：对于其他大小，仅写入低位字节
		copy(lengthBytes, []byte{byte(bodyLen >> 8), byte(bodyLen)})
	}

	// 将长度值复制回原始缓冲区的位置
	copy(finalBytes[patch.Position:], lengthBytes)
}
