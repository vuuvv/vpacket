package node

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/utils"
)

type BytesNode struct {
	core.BaseNode
	core.BaseEncodable
	Type       string
	Size       int
	SizeExpr   *core.CelEvaluator
	Default    []byte
	HasDefault bool
	Bits       int
	Check      *core.CelEvaluator
	Crc        string
	CrcStart   *core.CelEvaluator
	CrcEnd     *core.CelEvaluator
}

func (this *BytesNode) Compile(yf *core.YamlField, structures core.DataStructures) error {
	_ = this.BaseNode.Compile(yf, structures)
	err := this.BaseEncodable.Compile(yf, structures)
	if err != nil {
		return errors.WithStack(err)
	}
	this.Size = yf.Size
	this.Bits = yf.Bits
	this.Type = yf.Type
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
		defaultVal, err := utils.YamlDecode[string](&yf.Default)
		if err != nil {
			return err
		}
		this.Default, err = utils.ParseTValue(*defaultVal, this.Size, this.ByteOrder)
		if err != nil {
			return errors.Wrapf(err, "Compile 'default' of field %s: %s", this.Name, err.Error())
		}
	}

	return nil
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

func (this *BytesNode) readBits(ctx *core.Context) (any, error) {
	val, err := ctx.ReadBits(this.Bits)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return val, nil
}

func (this *BytesNode) readBytes(ctx *core.Context) (any, error) {
	readSize, err := ctx.GetSize(this.Size, this.SizeExpr)
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

	byteOrder := this.ByteOrder

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

	if startOffset < 0 || endOffset > len(ctx.Data) || startOffset > endOffset {
		return 0, errors.Errorf("invalid dynamic CRC scope: start=%d, end=%d, total_len=%d", startOffset, endOffset, len(ctx.Data))
	}

	return core.Crc(ctx.Data[startOffset:endOffset], this.Crc)
}

func (this *BytesNode) Encode(ctx *core.Context) error {
	size, err := ctx.GetSize(this.Size, this.SizeExpr)
	if err != nil {
		return errors.WithStack(err)
	}

	if size == 0 {
		return nil
	}

	if ctx.Round > this.GetRound() { // 编译的轮次大于节点轮次，跳过
		return nil
	}

	if ctx.Round < this.GetRound() { // 编译的轮次小于节点轮次，写入占位符
		return ctx.WritePlaceholder(size)
	}

	var val any
	var ok bool

	if this.Crc != "" {
		this.Type = "uint"
		crcVal, err := this.crc(ctx)
		if err != nil {
			return err
		}
		val = crcVal
	} else {
		val, ok = ctx.GetField(this.Name)
		if !ok {
			// 如果从输入中没有获取到对应的字段, 则根据是否有默认值来判断是否写入默认值, 没有的话写0补充
			if this.HasDefault {
				return ctx.WriteBytes(this.Default)
			} else {
				return ctx.WritePlaceholder(size)
			}
		}
	}

	return ctx.Write(this.Type, val, size, this)
}

func registerBytes() {
	core.RegisterNodeCompilerFactory[BytesNode](core.NodeTypeBytes, true)
}
