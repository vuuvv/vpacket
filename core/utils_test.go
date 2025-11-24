package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

func BenchmarkStringToBytes(b *testing.B) {
	str := "test string for benchmark"
	//byt := []byte("test string for benchmark")

	// 原生方式
	b.Run("Native", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = []byte(str)
		}
	})

	// 零拷贝方式
	b.Run("ZeroCopy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			//_ = S2B(str)
			//_ = 1 + 1
			//_ = string(byt)
			//_ = *(*string)(unsafe.Pointer(&byt))
		}
	})
}

func TestByteOrder(t *testing.T) {
	var buf1 bytes.Buffer
	_ = binary.Write(&buf1, binary.BigEndian, uint64(3539))
	fmt.Printf("%X\n", buf1.Bytes())
	var buf2 bytes.Buffer
	_ = binary.Write(&buf2, binary.LittleEndian, uint64(3539))
	fmt.Printf("%X\n", buf2.Bytes())
}

func TestWriteBits(t *testing.T) {
	writer := NewBitWriter()
	_ = writer.WriteBits(3539, 16, binary.BigEndian)
	fmt.Printf("%X\n", writer.Buffer.Bytes())
	writer = NewBitWriter()
	_ = writer.WriteBits(3539, 16, binary.LittleEndian)
	fmt.Printf("%X\n", writer.Buffer.Bytes())
}
