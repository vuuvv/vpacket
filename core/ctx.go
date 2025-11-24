package core

import "github.com/vuuvv/errors"

type Context struct {
	Data    []byte
	BytePos int
	BitPos  int
	Fields  map[string]any // 字段值
	Vars    map[string]any // 变量值
}

func NewContext(data []byte) *Context {
	return &Context{
		Data:   data,
		Vars:   make(map[string]any),
		Fields: make(map[string]any),
	}
}

func (c *Context) ReadBits(n int) (uint64, error) {
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
	if c.BytePos+fullBytes+BoolToInt(remainingBits > 0) > len(c.Data) {
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
		c.BytePos += BoolToInt(c.BitPos == 8)
		c.BitPos %= 8
	} else {
		c.BitPos = 0
	}

	return value, nil
}

func (c *Context) ReadBytes(n int) ([]byte, error) {
	if c.BitPos != 0 {
		return nil, errors.New("read bytes must be aligned")
	}
	if c.BytePos+n > len(c.Data) {
		return nil, errors.Errorf("EOF reading bytes, need %d, have %d", n, len(c.Data)-c.BytePos)
	}
	ret := c.Data[c.BytePos : c.BytePos+n]
	c.BytePos += n
	return ret, nil
}
