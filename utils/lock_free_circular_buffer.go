package utils

import (
	"sync/atomic"
	"unsafe"
)

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
