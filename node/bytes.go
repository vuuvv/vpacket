package node

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/utils"
	"math"
)

type BytesNode struct {
	Name        string
	Type        string
	Size        int
	SizeExpr    *core.CelEvaluator
	Default     any
	HasDefault  bool
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
				return err
			}
			ctx.SetField(this.Name, val)
			return nil
		} else {
			this.Size = this.Bits / 8
			this.Type = "uint"
		}
	}
	if this.Size == 0 && this.SizeExpr == nil {
		return errors.New("should specify a size or bits or size_expr")
	}
	val, err = this.readBytes(ctx)
	if err != nil {
		return err
	}
	ctx.SetField(this.Name, val)

	if this.Crc != "" {
		crcVal, err := this.crc(ctx)
		if err != nil {
			return err
		}

		if crcVal != val {
			return errors.Errorf("CRC check failed, expect '%X', actual '%X'", val, crcVal)
		}
	}

	if this.Check != nil {
		res, err := this.Check.Execute(ctx)
		if err != nil {
			return err
		}
		b, ok := res.(bool)
		if !ok {
			return errors.New("check result is not a bool")
		}
		if !b {
			return errors.New("check failed")
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
	case core.NodeTypeHex:
		return fmt.Sprintf("%02X", bytesVal), nil
	case core.NodeTypeString:
		return string(bytesVal), nil
	case core.NodeTypeInt:
		v, err := utils.ConvertBytesToInt(bytesVal, byteOrder)
		return int64(v), errors.WithStack(err)
	case core.NodeTypeUint:
		v, err := utils.ConvertBytesToInt(bytesVal, byteOrder)
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
		if v, ok := utils.ToUint64(res); ok {
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
		if v, ok := utils.ToUint64(res); ok {
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

	val, ok := ctx.GetField(this.Name)
	if !ok {
		if !this.HasDefault {
			return ctx.WritePlaceholder(size)
		} else {
			val = this.Default
		}
	}

	switch this.Type {
	case "", core.NodeTypeHex:
		return this.WriteHex(ctx, val, size)
	case core.NodeTypeString:
		return this.WriteString(ctx, val, size)
	case core.NodeTypeInt:
		return this.WriteUint(ctx, val, size)
	case core.NodeTypeUint:
		return this.WriteUint(ctx, val, size)
	case core.NodeTypeFloat:
		return this.WriteFloat(ctx, val, size)
	}
	return nil
}

func (this *BytesNode) WriteHex(ctx *core.Context, val any, size int) error {
	str, ok := val.(string)
	if !ok {
		return errors.Errorf("value of '%s' should be a string, '%v'", this.Name, val)
	}

	bs, err := hex.DecodeString(str)
	if err != nil {
		return errors.Errorf("value of '%s' should be a valid hex string, '%s'", this.Name, str)
	}

	return ctx.WriteBytes(utils.ResizeBytes(bs, size, this.PadByte, this.PadPosition))
}

func (this *BytesNode) WriteString(ctx *core.Context, val any, size int) error {
	str, ok := val.(string)
	if !ok {
		return errors.Errorf("value of '%s' should be a string, '%v'", this.Name, val)
	}
	return ctx.WriteBytes(utils.ResizeBytes([]byte(str), size, this.PadByte, this.PadPosition))
}

func (this *BytesNode) WriteUint(ctx *core.Context, val any, size int) error {
	i, ok := utils.ToUint64(val)
	if !ok {
		return errors.Errorf("value of '%s' should be a int, '%v'", this.Name, val)
	}
	return ctx.WriteInt(i, size, this.GetByteOrder())
}

func (this *BytesNode) WriteFloat(ctx *core.Context, val any, size int) error {
	f, ok := utils.ToFloat64(val)
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
	this.HasDefault = !yf.Default.IsZero()
	if this.Type == "" {
		this.Type = core.NodeTypeHex
	}

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

	if this.HasDefault {
		switch this.Type {
		case core.NodeTypeHex:
			fallthrough
		case core.NodeTypeString:
			defaultVal, err := utils.YamlDecode[string](&yf.Default)
			if err != nil {
				return errors.Wrapf(err, "Compile 'default' of field %s failed: %s", this.Name, err.Error())
			}
			this.Default = *defaultVal
		case core.NodeTypeInt:
			fallthrough
		case core.NodeTypeUint:
			defaultVal, err := utils.YamlDecode[uint64](&yf.Default)
			if err != nil {
				return errors.Wrapf(err, "Compile 'default' of field %s failed: %s", this.Name, err.Error())
			}
			this.Default = *defaultVal
		case core.NodeTypeFloat:
			defaultVal, err := utils.YamlDecode[float64](&yf.Default)
			if err != nil {
				return errors.Wrapf(err, "Compile 'default' of field %s failed: %s", this.Name, err.Error())
			}
			this.Default = *defaultVal
		}
	}

	return nil
}

func registerBytes() {
	core.RegisterNodeCompilerFactory[BytesNode](core.NodeTypeBytes, true)
}
