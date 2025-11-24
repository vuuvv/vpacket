package node

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
	"math"
)

type BytesNode struct {
	Name        string
	Type        string
	Size        int
	SizeExpr    *core.CelEvaluator
	Bits        int
	Check       *core.CelEvaluator
	Endian      string
	Crc         string
	CrcStart    *core.CelEvaluator
	CrcEnd      *core.CelEvaluator
	PadByte     byte
	PadPosition string
}

func (this *BytesNode) GetName() string {
	return this.Name
}

func (this *BytesNode) GetByteOrder() (byteOrder binary.ByteOrder) {
	byteOrder = binary.BigEndian
	if this.Endian == "litter" {
		byteOrder = binary.LittleEndian
	}
	return byteOrder
}

func (this *BytesNode) Decode(ctx *core.Context) (err error) {
	var val any

	if this.Crc != "" {
		this.Type = "uint"
	}

	if this.Bits > 0 {
		if this.Bits%8 != 0 {
			val, err = this.readBits(ctx)
			if err != nil {
				return errors.Wrapf(err, "Parse field %s: %s", this.Name, err.Error())
			}
			ctx.SetField(this.Name, val)
			return nil
		} else {
			this.Size = this.Bits / 8
			this.Type = "uint"
		}
	}
	if this.Size == 0 && this.SizeExpr == nil {
		return errors.Errorf("Parse field %s: should specify a size or bits or size_expr", this.Name)
	}
	val, err = this.readBytes(ctx)
	if err != nil {
		return errors.Wrapf(err, "Parse field %s: %s", this.Name, err.Error())
	}
	ctx.SetField(this.Name, val)

	if this.Crc != "" {
		crcVal, err := this.crc(ctx)
		if err != nil {
			return errors.Wrapf(err, "Parse field %s: %s", this.Name, err.Error())
		}

		if crcVal != val {
			return errors.Errorf("Parse field %s: CRC check failed, expect '%X', actual '%X'", this.Name, val, crcVal)
		}
	}

	if this.Check != nil {
		res, err := this.Check.Execute(ctx)
		if err != nil {
			return errors.Wrapf(err, "Parse field %s: %s", this.Name, err.Error())
		}
		b, ok := res.(bool)
		if !ok {
			return errors.Errorf("Parse field %s: check result is not a bool", this.Name)
		}
		if !b {
			return errors.Errorf("Parse field %s: check failed", this.Name)
		}
	}
	return nil
}

func (this *BytesNode) getSize(ctx *core.Context) (int, error) {
	size := this.Size
	if this.SizeExpr != nil {
		val, err := this.SizeExpr.Execute(ctx)
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
	if size < 0 {
		return 0, errors.Errorf("negative size: %d", size)
	}
	return size, nil
}

func (this *BytesNode) readBits(ctx *core.Context) (any, error) {
	val, err := ctx.ReadBits(this.Bits)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return val, nil
}

func (this *BytesNode) readBytes(ctx *core.Context) (any, error) {
	readSize, err := this.getSize(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if readSize == 0 {
		return "", nil
	}

	bytesVal, err := ctx.ReadBytes(readSize)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	byteOrder := this.GetByteOrder()

	switch this.Type {
	case "", core.NodeTypeHex:
		return fmt.Sprintf("%02X", bytesVal), nil
	case core.NodeTypeString:
		return string(bytesVal), nil
	case core.NodeTypeInt:
		v, err := core.ConvertBytesToInt(bytesVal, byteOrder)
		return int64(v), errors.WithStack(err)
	case core.NodeTypeUint:
		v, err := core.ConvertBytesToInt(bytesVal, byteOrder)
		return v, errors.WithStack(err)
	case core.NodeTypeFloat:
		bytesLen := len(bytesVal)
		if bytesLen == 32 {
			var v float32
			err = binary.Read(bytes.NewBuffer(bytesVal), byteOrder, &v)
			return v, errors.WithStack(err)
		} else if bytesLen == 64 {
			var v float64
			err = binary.Read(bytes.NewBuffer(bytesVal), byteOrder, &v)
			return v, errors.WithStack(err)
		}
		return nil, errors.Errorf("float size should be 4 or 8, actual %d", bytesLen)
	default:
		return nil, errors.Errorf("unsupported type: %s", this.Type)
	}
}

func (this *BytesNode) crc(ctx *core.Context) (uint64, error) {
	if this.Crc == "" {
		return 0, nil
	}

	// 计算校验范围的起始和结束字节偏移量
	var startOffset, endOffset int

	if this.CrcStart != nil {
		res, err := this.CrcStart.Execute(ctx)
		if err != nil {
			return 0, errors.Wrapf(err, "CRC start expression execute failed")
		}
		if v, ok := core.ToUint64(res); ok {
			startOffset = int(v)
		} else {
			return 0, errors.Errorf("CRC start expression did not return an integer, %v", res)
		}
	} else {
		// 如果未指定，则从报文开始 (0)
		startOffset = 0
	}

	// 计算结束偏移量
	if this.CrcEnd != nil {
		res, err := this.CrcEnd.Execute(ctx)
		if err != nil {
			return 0, errors.Wrapf(err, "CRC end expression execute failed: %s", err.Error())
		}
		if v, ok := core.ToUint64(res); ok {
			endOffset = int(v)
		} else {
			return 0, errors.Errorf("CRC end expression did not return an integer, %v", res)
		}
	} else {
		// 如果未指定，则使用 CRC 字段开始的字节偏移量
		endOffset = ctx.BytePos - (this.Bits / 8)
	}

	if startOffset < 0 || endOffset > len(ctx.Data) || startOffset >= endOffset {
		return 0, errors.Errorf("invalid dynamic CRC scope: start=%d, end=%d, total_len=%d", startOffset, endOffset, len(ctx.Data))
	}

	return core.Crc(ctx.Data[startOffset:endOffset], this.Crc)
}

func (this *BytesNode) Encode(ctx *core.Context) error {
	size, err := this.getSize(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	if size == 0 {
		return nil
	}

	if this.Crc != "" {
		this.Type = "uint"
	}

	switch this.Type {
	case "", core.NodeTypeHex:
		return this.WriteHex(ctx, size)
	case core.NodeTypeString:
		return this.WriteString(ctx, size)
	case core.NodeTypeInt:
		return this.WriteUint(ctx, size)
	case core.NodeTypeUint:
		return this.WriteUint(ctx, size)
	case core.NodeTypeFloat:
		return this.WriteFloat(ctx, size)
	}
	return nil
}

func (this *BytesNode) WriteHex(ctx *core.Context, size int) error {
	val, ok := ctx.GetField(this.Name)
	if !ok {
		return ctx.WritePlaceholder(this.Size)
	}
	str, ok := val.(string)
	if !ok {
		return errors.Errorf("value of '%s' should be a string, '%v'", this.Name, val)
	}

	bs, err := hex.DecodeString(str)
	if err != nil {
		return errors.Errorf("value of '%s' should be a valid hex string, '%s'", this.Name, str)
	}

	return ctx.WriteBytes(core.ResizeBytes(bs, size, this.PadByte, this.PadPosition))
}

func (this *BytesNode) WriteString(ctx *core.Context, size int) error {
	val, ok := ctx.GetField(this.Name)
	if !ok {
		return ctx.WritePlaceholder(this.Size)
	}
	str, ok := val.(string)
	if !ok {
		return errors.Errorf("value of '%s' should be a string, '%v'", this.Name, val)
	}
	return ctx.WriteBytes(core.ResizeBytes([]byte(str), size, this.PadByte, this.PadPosition))
}

func (this *BytesNode) WriteUint(ctx *core.Context, size int) error {
	val, ok := ctx.GetField(this.Name)
	if !ok {
		return ctx.WritePlaceholder(this.Size)
	}
	i, ok := core.ToUint64(val)
	if !ok {
		return errors.Errorf("value of '%s' should be a int, '%v'", this.Name, val)
	}
	return ctx.WriteInt(i, size, this.GetByteOrder())
}

func (this *BytesNode) WriteFloat(ctx *core.Context, size int) error {
	val, ok := ctx.GetField(this.Name)
	if !ok {
		return ctx.WritePlaceholder(this.Size)
	}
	f, ok := core.ToFloat64(val)
	if !ok {
		return errors.Errorf("value of '%s' should be a float, '%v'", this.Name, val)
	}
	return ctx.WriteFloat(f, size, this.GetByteOrder())
}

func (this *BytesNode) Compile(yf *core.YamlField, structures core.DataStructures) error {
	this.Name = yf.Name
	this.Size = yf.Size
	this.Bits = yf.Bits
	this.Type = yf.Type
	this.Endian = yf.Endian
	this.Crc = yf.Crc

	if yf.SizeExpr != "" {
		expr, err := core.CompileExpression(yf.SizeExpr)
		if err != nil {
			return errors.Wrapf(err, "Compile 'size_expr' of field %s: %s", this.Name, err.Error())
		}
		this.SizeExpr = expr
	}

	if yf.Check != "" {
		expr, err := core.CompileExpression(yf.Check)
		if err != nil {
			return errors.Wrapf(err, "Compile 'check' of field %s: %s", this.Name, err.Error())
		}
		this.Check = expr
	}

	if this.Crc != "" {
		if yf.CrcStart != "" {
			expr, err := core.CompileExpression(yf.CrcStart)
			if err != nil {
				return errors.Wrapf(err, "Compile crc_start of field %s: %s", this.Name, err.Error())
			}
			this.CrcStart = expr
		}
		if yf.CrcEnd != "" {
			expr, err := core.CompileExpression(yf.CrcEnd)
			if err != nil {
				return errors.Wrapf(err, "Compile crc_end of field %s: %s", this.Name, err.Error())
			}
			this.CrcEnd = expr
		}
	}

	return nil
}

func registerBytes() {
	core.RegisterNodeCompilerFactory[BytesNode](core.NodeTypeBytes, true)
}
