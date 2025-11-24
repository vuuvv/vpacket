package core

import (
	"github.com/vuuvv/errors"
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
	Compile(fields *YamlField, structures DataStructures) error
}

var nodeCompilers = make(map[string]NodeCompileFunc)
var defaultNodeCompiler NodeCompileFunc = nil

type NodeCompileFunc func(fields *YamlField, structures DataStructures) (Node, error)

func RegisterNodeCompilerFactory[T any](name string, isDefault bool) {
	fn := func(fields *YamlField, structures DataStructures) (Node, error) {
		var v T
		node, ok := CastTo[Node](&v)
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
			return nil, errors.WithStack(err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}
