package core

import (
	"encoding/binary"
	"fmt"
	"github.com/spf13/cast"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"
	"unsafe"
)

func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func CastTo[T any](src any) (target T, ok bool) {
	target, ok = src.(T)
	return
}

// B2S converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
func B2S(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// S2B converts string to a byte slice without memory allocation.
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func S2B(s string) (b []byte) {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	bh.Data = sh.Data
	bh.Cap = sh.Len
	bh.Len = sh.Len

	return b
}

func ConvertBytesToInt(data []byte, byteOrder binary.ByteOrder) (uint64, error) {
	if byteOrder == binary.LittleEndian {
		return ConvertBytesToIntLE(data)
	}
	return ConvertBytesToIntBE(data)
}

func ToString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case error:
		return fmt.Sprintf("%+v", v)
	default:
		return cast.ToString(val)
	}
}

func ToUint64(val any) (uint64, bool) {
	switch v := val.(type) {
	case int:
		return uint64(v), true
	case int8:
		return uint64(v), true
	case int16:
		return uint64(v), true
	case int32:
		return uint64(v), true
	case int64:
		return uint64(v), true
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	case float32:
		return uint64(v), true
	case float64:
		return uint64(v), true
	}

	s := ToString(val)
	i, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return uint64(i), true
	}
	return 0, false
}

func ToFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	}

	s := ToString(val)
	i, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return i, true
	}
	return 0, false
}

func ConvertBytesToIntBE(data []byte) (uint64, error) {
	byteLen := len(data)
	if byteLen < 1 || byteLen > 8 {
		return 0, fmt.Errorf("字节长度必须在1-8之间")
	}

	var result uint64
	for i := 0; i < byteLen; i++ {
		result = (result << 8) | uint64(data[i])
	}
	return result, nil
}

func ConvertBytesToIntLE(data []byte) (uint64, error) {
	byteLen := len(data)
	if byteLen < 1 || byteLen > 8 {
		return 0, fmt.Errorf("字节长度必须在1-8之间")
	}

	var result uint64
	for i := 0; i < byteLen; i++ {
		result |= uint64(data[i]) << (i * 8)
	}
	return result, nil
}

const (
	PaddingLeft  string = "left"  // 在前面填充
	PaddingRight string = "right" // 在后面填充
)

func ResizeBytes(data []byte, size int, padByte byte, position string) []byte {
	if position == "" {
		position = PaddingRight
	}

	if size < 0 {
		return nil
	}

	currentLen := len(data)

	// 长度正好
	if currentLen == size {
		return data
	}

	// 需要截断
	if currentLen > size {
		return data[:size]
	}

	// 需要填充
	needPad := size - currentLen

	// 检查容量是否足够复用
	if position == PaddingRight && cap(data) >= size {
		// 在后面填充且容量足够，可以复用数组
		data = data[:size]
		for i := currentLen; i < size; i++ {
			data[i] = padByte
		}
		return data
	}

	// 其他情况需要创建新数组
	result := make([]byte, size)

	switch position {
	case PaddingLeft:
		// 在前面填充
		for i := 0; i < needPad; i++ {
			result[i] = padByte
		}
		copy(result[needPad:], data)

	case PaddingRight:
		// 在后面填充
		copy(result, data)
		for i := currentLen; i < size; i++ {
			result[i] = padByte
		}
	}

	return result
}

//func BytesTo(bytes []byte, typ string) any {
//}

type WithTime struct {
	Time  time.Time `json:"time,omitempty"`
	Data  any       `json:"data,omitempty"`
	Error error     `json:"error,omitempty"`
}

// LockFreeCircularBuffer 无锁环形缓冲区
type LockFreeCircularBuffer struct {
	data  []unsafe.Pointer
	size  int32
	head  int32
	count int32
}

// NewLockFreeCircularBuffer 创建无锁环形缓冲区
func NewLockFreeCircularBuffer(size int) *LockFreeCircularBuffer {
	return &LockFreeCircularBuffer{
		data:  make([]unsafe.Pointer, size),
		size:  int32(size),
		head:  0,
		count: 0,
	}
}

// Add 添加元素（无锁）
func (cb *LockFreeCircularBuffer) Add(item any) {
	pos := atomic.AddInt32(&cb.head, 1) - 1
	index := pos % cb.size
	if pos >= cb.size {
		atomic.AddInt32(&cb.count, -1)
	}

	atomic.StorePointer(&cb.data[index], unsafe.Pointer(&item))
	atomic.AddInt32(&cb.count, 1)
}

// GetAll 获取所有元素
func (cb *LockFreeCircularBuffer) GetAll() []any {
	count := atomic.LoadInt32(&cb.count)
	if count == 0 {
		return nil
	}

	result := make([]interface{}, count)
	head := atomic.LoadInt32(&cb.head)
	start := head - count

	for i := int32(0); i < count; i++ {
		pos := (start + i) % cb.size
		if pos < 0 {
			pos += cb.size
		}
		ptr := atomic.LoadPointer(&cb.data[pos])
		if ptr != nil {
			result[i] = *(*interface{})(ptr)
		}
	}

	return result
}
