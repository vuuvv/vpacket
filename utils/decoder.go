package utils

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/google/cel-go/cel"
	"github.com/vuuvv/errors"
	"golang.org/x/exp/constraints"
	"strconv"
	"strings"
)

func Bytes2Hex(bytes []byte) string {
	buf := strings.Builder{}
	for _, b := range bytes {
		buf.WriteString(fmt.Sprintf(" %02x", b))
	}
	if buf.Len() > 0 {
		return buf.String()[1:]
	}
	return ""
}

func Uint64ToBytes[T constraints.Integer](u T, size int, order binary.ByteOrder) []byte {
	data := make([]byte, 8)
	order.PutUint64(data, uint64(u))

	switch order {
	case binary.LittleEndian:
		return data[:size]
	default: // 默认情况是大端的
		return data[8-size:]
	}
}

// ParseTValue 核心解析函数：将输入字符串解析为 []byte
func ParseTValue(inputString string, size int, byteOrder binary.ByteOrder) ([]byte, error) {
	// 1. 预处理：判断是否符合 T'xxx' 格式
	var typeID string
	var dataStr string

	// 检查是否符合 T'xxx' 格式：长度大于等于 4，第二个字符是 '，最后一个字符是 '
	if len(inputString) >= 4 && inputString[1] == '\'' && inputString[len(inputString)-1] == '\'' {
		// 符合 T'xxx' 格式
		typeID = string(inputString[0])
		dataStr = inputString[2 : len(inputString)-1]
	} else {
		// 不符合 T'xxx' 格式，使用默认规则：h'xxx'
		// 整个输入字符串视为十六进制数据
		typeID = "h"
		dataStr = inputString
	}

	var value []byte
	var err error

	// 2. 根据类型标识符进行分派解析
	switch typeID {
	case "b": // 二进制 (Binary)
		binStr := strings.TrimPrefix(strings.TrimPrefix(dataStr, "0b"), "b")
		u, e := strconv.ParseUint(binStr, 2, 64)
		if e != nil {
			return nil, errors.Errorf("invalid binary number 'b''%s': %w", dataStr, e)
		}
		value = Uint64ToBytes(u, size, byteOrder)

	case "o": // 八进制 (Octal)
		octStr := strings.TrimPrefix(dataStr, "0")
		u, e := strconv.ParseUint(octStr, 8, 64)
		if e != nil {
			return nil, errors.Errorf("invalid octal number 'o''%s': %w", dataStr, e)
		}
		value = Uint64ToBytes(u, size, byteOrder)

	case "d": // 十进制 (Decimal)
		i, e := strconv.ParseInt(dataStr, 10, 64)
		if e != nil {
			return nil, errors.Errorf("invalid decimal number 'd''%s': %w", dataStr, e)
		}
		value = Uint64ToBytes(i, size, byteOrder)

	case "x", "h": // 十六进制 (Hex)
		hexStr := strings.TrimPrefix(strings.TrimPrefix(dataStr, "0x"), "h")

		// 确保长度是偶数，如果不是，则补零以保证字节对齐
		if len(hexStr)%2 != 0 {
			hexStr = "0" + hexStr
		}

		value, err = hex.DecodeString(hexStr)
		if err != nil {
			return nil, errors.Errorf("invalid hex string '%s''%s': %w", typeID, dataStr, err)
		}
		if size < 0 {
			return value, nil
		}
		value = ResizeBytes(value, size, 0, PaddingRight)

	case "s": // 字符串 (String)
		value = []byte(dataStr)
		if size < 0 {
			return value, nil
		}
		value = ResizeBytes(value, size, 0, PaddingRight)
	case "e": // 静态计算表达式（eval）
		// ParseTValue 没有协议上下文，只允许无变量的静态整数表达式。
		// 动态字段计算必须使用 calc 节点；在这里放开 fields/vars 会让默认值解析承担不必要的执行风险。
		if size < 1 || size > 8 {
			return nil, errors.Errorf("eval value requires a byte size between 1 and 8, actual %d", size)
		}
		env, e := cel.NewEnv()
		if e != nil {
			return nil, errors.Errorf("create eval environment failed: %v", e)
		}
		ast, issues := env.Compile(dataStr)
		if issues != nil && issues.Err() != nil {
			return nil, errors.Errorf("invalid eval expression 'e''%s': %v", dataStr, issues.Err())
		}
		program, e := env.Program(ast)
		if e != nil {
			return nil, errors.Errorf("compile eval expression 'e''%s' failed: %v", dataStr, e)
		}
		out, _, e := program.Eval(map[string]any{})
		if e != nil {
			return nil, errors.Errorf("evaluate expression 'e''%s' failed: %v", dataStr, e)
		}

		var number uint64
		switch result := out.Value().(type) {
		case int64:
			if result < 0 {
				return nil, errors.Errorf("eval expression 'e''%s' returned a negative value", dataStr)
			}
			number = uint64(result)
		case uint64:
			number = result
		default:
			// 只接受整数，避免浮点舍入或布尔值被隐式转换后写入错误控制字节。
			return nil, errors.Errorf("eval expression 'e''%s' must return an integer, actual %T", dataStr, out.Value())
		}
		value = Uint64ToBytes(number, size, byteOrder)

	default:
		// 如果是 T'xxx' 格式但 T 是未知类型
		return nil, errors.Errorf("unrecognized type identifier: %s. Expected b, o, d, x, h, s, or e.", typeID)
	}

	return value, nil
}
