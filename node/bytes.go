package node

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

const Bytes = "byte"

type BytesNode struct {
	Name     string
	Type     string
	Size     int
	SizeExpr *core.CelEvaluator
	Bits     int
	Check    *core.CelEvaluator
	Endian   string
	Crc      string
	CrcStart *core.CelEvaluator
	CrcEnd   *core.CelEvaluator
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
			ctx.Fields[this.Name] = val
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
	ctx.Fields[this.Name] = val

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

func (this *BytesNode) readBits(ctx *core.Context) (any, error) {
	val, err := ctx.ReadBits(this.Bits)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return val, nil
}

func (this *BytesNode) readBytes(ctx *core.Context) (any, error) {
	readSize := this.Size
	if this.SizeExpr != nil {
		val, err := this.SizeExpr.Execute(ctx)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		switch v := val.(type) {
		case int64:
			readSize = int(v)
		case uint64:
			readSize = int(v)
		default:
			return nil, errors.Errorf("size expr return invalid type: %T", val)
		}
	}
	if readSize < 0 {
		return nil, errors.Errorf("negative size: %d", readSize)
	}
	if readSize == 0 {
		return "", nil
	}

	bytesVal, err := ctx.ReadBytes(readSize)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var byteOrder binary.ByteOrder = binary.BigEndian
	if this.Endian == "litter" {
		byteOrder = binary.LittleEndian
	}

	switch this.Type {
	case "", "hex":
		return fmt.Sprintf("%02X", bytesVal), nil
	case "string":
		return string(bytesVal), nil
	case "int":
		v, err := core.ConvertBytesToInt(bytesVal, byteOrder)
		return int64(v), errors.WithStack(err)
	case "uint":
		v, err := core.ConvertBytesToInt(bytesVal, byteOrder)
		return v, errors.WithStack(err)
	case "float":
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

func (this *BytesNode) Encode(input map[string]any, writer *core.BitWriter) error {
	//TODO implement me
	panic("implement me")
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
	core.RegisterNodeCompilerFactory[BytesNode](Bytes, true)
}
