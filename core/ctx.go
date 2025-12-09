package core

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/utils"
	"math"
	"strings"
)

const (
	FlowEncode = "encode"
	FlowDecode = "decode"
)

type Context struct {
	Writer      bytes.Buffer
	Data        []byte // 解析时使用
	BytePos     int
	BitPos      int
	Fields      map[string]any // 字段值
	Vars        map[string]any // 变量值
	Offsets     map[string]int // 字段的偏移量
	Flow        string         // 表明当前处理的流程,编码或解码
	Round       int            // 编码的第几轮
	NodeOffsets []int          // 正在处理的node的索引, 每个node有一个起始位置
	NodeIndex   int
	ArrayStack  [][]any // 当前元素所在的数组
	ArrayDeep   int     // 是否在数组中
	Array       []any   // 当前的Array
}

func NewContext(data []byte) *Context {
	return &Context{
		Data:    data,
		Vars:    make(map[string]any),
		Fields:  make(map[string]any),
		Offsets: make(map[string]int),
	}
}

func (c *Context) InArray() bool {
	return len(c.ArrayStack) > 0
}

// SetField 将 value 嵌套地放入 dict 中。
// name 是一个用 "." 分割的路径字符串，例如 "a.b.c"。
// 如果路径中的中间 map 不存在，该函数会自动创建它们。
// 如果路径中的中间某个 key 对应的值存在但不是 map (例如是个 int)，它会被新的 map 覆盖以继续路径。
func (c *Context) SetField(name string, value any) {
	// 1. 如果 name 为空字符串，直接返回，不做任何操作 (或者可以根据需求决定是否允许空 key)
	if name == "" {
		return
	}

	// 2. 使用 "." 分割路径
	keys := strings.Split(name, ".")

	// currentMap 用来追踪当前正在处理的层级的 map。
	// 初始时指向最外层的 dict。
	currentMap := c.Fields

	// 3. 遍历路径中的 key，除了最后一个。
	// 这个循环的目标是确保通往最终目标的路径上的所有中间结构都存在且是 map。
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]

		// 尝试获取当前 key 对应的值，并断言它是一个 map[string]any。
		// currentMap[key] 获取值。
		// .(map[string]any) 是类型断言。
		subMap, ok := currentMap[key].(map[string]any)

		// 如果断言失败 (!ok)，有两种情况：
		// a) key 不存在。
		// b) key 存在，但它的值不是 map[string]any (例如，可能是 int 或 string)。
		// 在这两种情况下，我们需要创建一个新的 map 来继续向下的路径。
		// 注意：如果情况是 b)，原有的非 map 值会被覆盖。
		if !ok {
			subMap = make(map[string]any)
			currentMap[key] = subMap
		}

		// 将指针向下移动到下一层 map
		currentMap = subMap
	}

	// 4. 循环结束后，currentMap 指向了倒数第二层的 map。
	// 获取最后一个 key，并将 value 赋值给它。
	lastKey := keys[len(keys)-1]
	currentMap[lastKey] = value
}

// GetField 根据被点号分割的路径 name，从 dict 中尝试获取嵌套的值。
// 返回值:
// 1. any: 找到的值。如果未找到，则为 nil。
// 2. bool: 一个布尔标志。true 表示找到了路径对应的值（即使该值本身是 nil）；false 表示路径不存在或中途断裂。
func (c *Context) GetField(name string) (any, bool) {
	// 1. 处理边界情况：如果路径为空，通常视为获取根 map 本身
	if name == "" {
		// 根据具体需求，这里也可以返回 nil, false
		return c.Fields, true
	}

	// 2. 使用 "." 分割路径
	keys := strings.Split(name, ".")

	// 3. 初始化当前 map 指针指向根 dict
	currentMap := c.Fields

	// 4. 遍历所有的 key
	for i, key := range keys {
		// 尝试在当前层级查找 key
		val, ok := currentMap[key]
		if !ok {
			// 路径断裂：当前 key 不存在
			return nil, false
		}

		// 检查是否到达路径的最后一个 key
		if i == len(keys)-1 {
			// 到达终点，成功找到值并返回
			return val, true
		}

		// 如果不是最后一个 key，我们需要继续向下遍历。
		// 这就要求刚才找到的 val 必须是一个 map[string]any。
		// 进行类型断言：
		subMap, isMap := val.(map[string]any)
		if !isMap {
			// 路径受阻：虽然 key 存在，但其值不是 map，无法继续下一层的查找。
			// 例如：路径是 "a.b.c"，但 dict["a"]["b"] 的值是一个整数。
			return nil, false
		}

		// 指针下移到下一层 map
		currentMap = subMap
	}

	// 理论上代码不会执行到这里
	return nil, false
}

func (c *Context) MatchFlow(node Node) bool {
	if node.GetFlow() == "" {
		return true
	}
	return node.GetFlow() == c.Flow
}

func (c *Context) MatchRound(node Node) bool {
	return node.GetRound() == c.Round
}

func (ctx *Context) GetSize(size int, sizeExpr *CelEvaluator) (int, error) {
	if sizeExpr != nil {
		val, err := sizeExpr.Execute(ctx)
		if err != nil {
			return 0, errors.WithStack(err)
		}
		switch v := val.(type) {
		case int64:
			size = int(v)
		case uint64:
			size = int(v)
		case float32:
			size = int(math.Round(float64(v)))
		case float64:
			size = int(math.Round(v))
		default:
			return 0, errors.Errorf("size expr return invalid type: %T", val)
		}
	}
	//if size < 0 {
	//	return 0, errors.Errorf("negative size: %d", size)
	//}
	return size, nil
}

func (c *Context) ReadBits(n int) (uint64, error) {
	if n == 0 {
		return 0, nil
	}
	if n > 64 {
		return 0, errors.New("cannot read more than 64 bits")
	}
	if c.BytePos >= len(c.Data) {
		return 0, errors.New("EOF")
	}

	// 计算可以读取的完整字节数和剩余位数
	fullBytes := (n + c.BitPos) / 8
	remainingBits := (n + c.BitPos) % 8

	// 检查是否有足够的数据
	if c.BytePos+fullBytes+utils.BoolToInt(remainingBits > 0) > len(c.Data) {
		return 0, errors.New("unexpected EOF inside bits")
	}

	var value uint64 = 0
	// 读取完整字节
	for i := 0; i < fullBytes; i++ {
		value = (value << 8) | uint64(c.Data[c.BytePos])
		c.BytePos++
	}

	// 读取剩余位
	if remainingBits > 0 {
		value = (value << remainingBits) | uint64(c.Data[c.BytePos]>>(8-remainingBits))
		c.BitPos = remainingBits
		c.BytePos += utils.BoolToInt(c.BitPos == 8)
		c.BitPos %= 8
	} else {
		c.BitPos = 0
	}

	return value, nil
}

func (c *Context) ReadBytes(n int) ([]byte, error) {
	if n == 0 {
		return nil, nil
	}
	if c.BitPos != 0 {
		return nil, errors.New("read bytes must be aligned")
	}
	if n < 0 {
		n = len(c.Data) - c.BytePos
	}
	if c.BytePos+n > len(c.Data) {
		return nil, errors.Errorf("EOF reading bytes, need %d, have %d", n, len(c.Data)-c.BytePos)
	}
	ret := c.Data[c.BytePos : c.BytePos+n]
	c.BytePos += n
	return ret, nil
}

func (w *Context) WriteInt(value uint64, size int, byteOrder binary.ByteOrder) error {
	buf := make([]byte, 8)
	byteOrder.PutUint64(buf, value)

	// 写入最后 numBytes 个字节
	switch byteOrder {
	case binary.BigEndian:
		return w.WriteBytes(buf[8-size:])
	case binary.LittleEndian:
		return w.WriteBytes(buf[:size])
	}
	return nil
}

func (w *Context) WriteFloat(value float64, size int, byteOrder binary.ByteOrder) error {
	buf := bytes.Buffer{}
	err := binary.Write(&buf, byteOrder, value)
	if err != nil {
		return errors.WithStack(err)
	}
	switch byteOrder {
	case binary.BigEndian:
		return w.WriteBytes(buf.Bytes()[8-size:])
	case binary.LittleEndian:
		return w.WriteBytes(buf.Bytes()[:size])
	}
	return nil
}

// WriteBytes 写入完整的字节
func (w *Context) WriteBytes(data []byte) error {
	if w.Round == 0 {
		_, err := w.Writer.Write(data)
		return err
	}

	idx := w.NodeIndex - 1

	if idx >= len(w.NodeOffsets) {
		return errors.New("node index out of range")
	}
	offset := w.NodeOffsets[idx]
	copy(w.Data[offset:], data)
	return nil
}

func (w *Context) WritePlaceholder(size int) error {
	return w.WriteBytes(bytes.Repeat([]byte{0}, size))
}

func (w *Context) Write(typ string, val any, size int, encodable Encodable) error {
	switch typ {
	case "", NodeTypeHex:
		return w.writeHex(val, size, encodable.GetPadByte(), encodable.GetPadPosition())
	case NodeTypeString:
		return w.writeString(val, size, encodable.GetPadByte(), encodable.GetPadPosition())
	case NodeTypeInt:
		return w.writeUint(val, size, encodable.GetByteOrder())
	case NodeTypeUint:
		return w.writeUint(val, size, encodable.GetByteOrder())
	case NodeTypeFloat:
		return w.writeFloat(val, size, encodable.GetByteOrder())
	}
	return nil
}

func (ctx *Context) writeHex(val any, size int, padByte byte, padPosition string) error {
	str, ok := val.(string)
	if !ok {
		return errors.Errorf("value should be a string, '%v'", val)
	}

	bs, err := hex.DecodeString(str)
	if err != nil {
		return errors.Errorf("value should be a valid hex string, '%s'", str)
	}

	return ctx.WriteBytes(utils.ResizeBytes(bs, size, padByte, padPosition))
}

func (ctx *Context) writeString(val any, size int, padByte byte, padPosition string) error {
	str, ok := val.(string)
	if !ok {
		return errors.Errorf("value should be a string, '%v'", val)
	}
	return ctx.WriteBytes(utils.ResizeBytes([]byte(str), size, padByte, padPosition))
}

func (ctx *Context) writeUint(val any, size int, byteOrder binary.ByteOrder) error {
	i, ok := utils.ToUint64(val)
	if !ok {
		return errors.Errorf("value should be a int, '%v'", val)
	}
	return ctx.WriteInt(i, size, byteOrder)
}

func (ctx *Context) writeFloat(val any, size int, byteOrder binary.ByteOrder) error {
	f, ok := utils.ToFloat64(val)
	if !ok {
		return errors.Errorf("value should be a float, '%v'", val)
	}
	return ctx.WriteFloat(f, size, byteOrder)
}
