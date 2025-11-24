package crc16

import (
	"fmt"
	"hash"
)

const (
	ARC         = 0
	AUG_CCITT   = 1
	BUYPASS     = 2
	CCITT_FALSE = 3
	CDMA2000    = 4
	DDS_110     = 5
	DECT_R      = 6
	DECT_X      = 7
	DNP         = 8
	EN_13757    = 9
	GENIBUS     = 10
	MAXIM       = 11
	MCRF4XX     = 12
	RIELLO      = 13
	T10_DIF     = 14
	TELEDISK    = 15
	TMS37157    = 16
	USB         = 17
	CRC_A       = 18
	KERMIT      = 19
	MODBUS      = 20
	X_25        = 21
	XMODEM      = 22
)

var params = []*Params{
	&CRC16_ARC,
	&CRC16_AUG_CCITT,
	&CRC16_BUYPASS,
	&CRC16_CCITT_FALSE,
	&CRC16_CDMA2000,
	&CRC16_DDS_110,
	&CRC16_DECT_R,
	&CRC16_DECT_X,
	&CRC16_DNP,
	&CRC16_EN_13757,
	&CRC16_GENIBUS,
	&CRC16_MAXIM,
	&CRC16_MCRF4XX,
	&CRC16_RIELLO,
	&CRC16_T10_DIF,
	&CRC16_TELEDISK,
	&CRC16_TMS37157,
	&CRC16_USB,
	&CRC16_CRC_A,
	&CRC16_KERMIT,
	&CRC16_MODBUS,
	&CRC16_X_25,
	&CRC16_XMODEM,
}

var tables [23]*Table

// This file contains the CRC16 implementation of the
// go standard library hash.Hash interface

type Hash16 interface {
	hash.Hash
	Sum16() uint16
}

type digest struct {
	sum uint16
	t   *Table
}

// Write adds more data to the running digest.
// It never returns an error.
func (h *digest) Write(data []byte) (int, error) {
	h.sum = Update(h.sum, data, h.t)
	return len(data), nil
}

// Sum appends the current digest (leftmost byte first, big-endian)
// to b and returns the resulting slice.
// It does not change the underlying digest state.
func (h *digest) Sum(b []byte) []byte {
	s := h.Sum16()
	return append(b, byte(s>>8), byte(s))
}

// Reset resets the Hash to its initial state.
func (h *digest) Reset() {
	h.sum = h.t.params.Init
}

// Size returns the number of bytes Sum will return.
func (h *digest) Size() int {
	return 2
}

// BlockSize returns the undelying block size.
// See digest.Hash.BlockSize
func (h *digest) BlockSize() int {
	return 1
}

// Sum16 returns the CRC16 checksum.
func (h *digest) Sum16() uint16 {
	return Complete(h.sum, h.t)
}

// New creates a new CRC16 digest for the given table.
func New(t int) Hash16 {
	if t < 0 || t > 22 {
		panic(fmt.Sprintf("crc16: invalid table,%d, value should in 0~22", t))
	}

	if tables[t] == nil {
		p := params[t]
		tables[t] = MakeTable(*p)
	}

	h := digest{t: tables[t]}
	h.Reset()
	return &h
}
