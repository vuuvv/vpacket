package core

import (
	"encoding/binary"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/utils"
)

const (
	NodeTypeBytes  = "bytes"  // 默认类型
	NodeTypeCalc   = "calc"   // 计算类型
	NodeTypeIf     = "if"     // 条件类型
	NodeTypeSwitch = "switch" // switch类型
	NodeTypeArray  = "array"  // 数组类型
	NodeTypeStruct = "struct" // 结构类型,嵌套
	NodeTypeHex    = "hex"
	NodeTypeString = "string"
	NodeTypeInt    = "int"
	NodeTypeUint   = "uint"
	NodeTypeFloat  = "float"
)

type Node interface {
	Decode(ctx *Context) error
	Encode(ctx *Context) error
	GetName() string
	GetFlow() string
	GetRound() int
	IsTrackOffset() bool
	Compile(fields *YamlField, structures DataStructures) error
}

type BaseNode struct {
	Name        string
	Flow        string // 流程，编码或解码
	Round       int    // 第几轮进行计算,用于编码流程
	TrackOffset bool   // 是否跟踪偏移量, 用于回填
}

func (b *BaseNode) Decode(ctx *Context) error {
	//TODO implement me
	panic("implement me")
}

func (b *BaseNode) Encode(ctx *Context) error {
	//TODO implement me
	panic("implement me")
}

func (b *BaseNode) GetName() string {
	return b.Name
}

func (b *BaseNode) GetFlow() string {
	return b.Flow
}

func (b *BaseNode) GetRound() int {
	return b.Round
}

func (b *BaseNode) IsTrackOffset() bool {
	return b.TrackOffset
}

func (b *BaseNode) Compile(yf *YamlField, structures DataStructures) error {
	b.Name = yf.Name
	b.Flow = yf.Flow
	b.Round = yf.Round
	b.TrackOffset = yf.TrackOffset
	return nil
}

type Encodable interface {
	GetByteOrder() binary.ByteOrder
	GetPadByte() byte
	GetPadPosition() string
}

type BaseEncodable struct {
	ByteOrder   binary.ByteOrder
	PadByte     byte
	PadPosition string
}

func (b *BaseEncodable) GetByteOrder() binary.ByteOrder {
	return b.ByteOrder
}

func (b *BaseEncodable) GetPadByte() byte {
	return b.PadByte
}

func (b *BaseEncodable) GetPadPosition() string {
	return b.PadPosition
}

func (b *BaseEncodable) Compile(yf *YamlField, structures DataStructures) (err error) {
	b.ByteOrder = utils.GetByteOrder(yf.Endian)
	bs, err := utils.ParseTValue(yf.PadByte, 1, b.ByteOrder)
	if err != nil {
		return err
	}
	b.PadByte = bs[0]
	b.PadPosition = yf.PadPosition
	return nil
}

var nodeCompilers = make(map[string]NodeCompileFunc)
var defaultNodeCompiler NodeCompileFunc = nil

type NodeCompileFunc func(fields *YamlField, structures DataStructures) (Node, error)

func RegisterNodeCompilerFactory[T any](name string, isDefault bool) {
	fn := func(fields *YamlField, structures DataStructures) (Node, error) {
		var v T
		node, ok := utils.CastTo[Node](&v)
		if !ok {
			return nil, errors.Errorf("Node type [%s] not match: %T", name, node)
		}
		err := node.Compile(fields, structures)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return node, nil
	}
	nodeCompilers[name] = fn
	if isDefault {
		defaultNodeCompiler = fn
	}
}

func NodeCompile(fields []*YamlField, structures DataStructures) ([]Node, error) {
	var nodes []Node
	for _, yf := range fields {
		fn, ok := nodeCompilers[yf.Type]
		if !ok {
			if defaultNodeCompiler != nil {
				fn = defaultNodeCompiler
			}
		}
		if fn == nil {
			return nil, errors.Errorf("Node type [%s] not match, and not set default compiler", yf.Type)
		}
		node, err := fn(yf, structures)
		if err != nil {
			return nil, errors.Wrapf(err, "Field '%s' compile failed", yf.Name)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func NodeEncode(ctx *Context, nodes ...Node) error {
	for _, node := range nodes {
		if !ctx.MatchFlow(node) {
			continue
		}
		ctx.NodeIndex++

		if ctx.Round == 0 {
			// 父节点和节点可能会重复
			pos := ctx.Writer.Len()
			ctx.NodeOffsets = append(ctx.NodeOffsets, pos)
			if node.IsTrackOffset() {
				ctx.Offsets[node.GetName()] = pos
			}
		}
		if err := node.Encode(ctx); err != nil {
			return errors.Wrapf(err, "Encode field %s failure: %s", node.GetName(), err.Error())
		}
	}
	return nil
}

func NodeDecode(ctx *Context, nodes ...Node) error {
	for _, node := range nodes {
		if !ctx.MatchFlow(node) {
			continue
		}
		if err := node.Decode(ctx); err != nil {
			return errors.Wrapf(err, "Decode field '%s' failure: %s", node.GetName(), err.Error())
		}
	}
	return nil
}
