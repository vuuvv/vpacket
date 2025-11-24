package core

import (
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
