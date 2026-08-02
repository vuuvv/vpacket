package utils

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseTValueEval(t *testing.T) {
	// e 类型用于协议配置中的静态算术常量；结果必须按指定字节序写成定长整数。
	actual, err := ParseTValue("e'1 + 2 * 3'", 2, binary.BigEndian)
	if err != nil {
		t.Fatalf("解析静态表达式失败: %v", err)
	}
	if expected := []byte{0x00, 0x07}; !bytes.Equal(actual, expected) {
		t.Fatalf("表达式结果错误: %X，期望 %X", actual, expected)
	}
}

func TestParseTValueEvalRejectsUnsafeResult(t *testing.T) {
	// 负数和非整数若被截断或转换为无符号数，会改变控制指令语义，必须明确失败。
	for _, input := range []string{"e'-1'", "e'1.5'", "e'true'", "e'unknownValue'"} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseTValue(input, 2, binary.BigEndian); err == nil {
				t.Fatalf("不安全表达式 %s 被错误接受", input)
			}
		})
	}
}
