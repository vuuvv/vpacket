package node

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

type ArrayNode struct {
	core.BaseNode
	Item     []core.Node
	Size     int
	SizeExpr *core.CelEvaluator
}

func (n *ArrayNode) Compile(yf *core.YamlField, structures core.DataStructures) (err error) {
	_ = n.BaseNode.Compile(yf, structures)

	n.Size = yf.Size
	if yf.SizeExpr != "" {
		expr, err := core.CompileExpression(yf.SizeExpr)
		if err != nil {
			return errors.Wrapf(err, "Compile 'size_expr' of field %s: %s", n.Name, err.Error())
		}
		n.SizeExpr = expr
	}

	n.Item, err = core.NodeCompileWithRef(yf.Ref, yf.Fields, structures, true)
	if err != nil {
		return errors.Wrapf(err, "compile 'item' failed: %s", err.Error())
	}
	return nil
}

func (n *ArrayNode) Decode(ctx *core.Context) error {
	var val []any
	//ctx.SetField(n.Name, val)
	//ctx.ArrayStack = append(ctx.ArrayStack, val)
	//ctx.Array = val
	//defer func() {
	//	ctx.ArrayStack = ctx.ArrayStack[:len(ctx.ArrayStack)-1]
	//	if len(ctx.ArrayStack) > 0 {
	//		ctx.Array = ctx.Array[:len(ctx.Array)-1]
	//	}
	//}()
	length, err := ctx.GetSize(n.Size, n.SizeExpr)
	if err != nil {
		return errors.WithStack(err)
	}
	for i := 0; i < length; i++ {
		err = core.NodeDecode(ctx, n.Item...)
		if err != nil {
			return errors.WithStack(err)
		}
		item, ok := ctx.GetField(n.Name)
		if ok {
			val = append(val, item)
		}
		ctx.SetField(n.Name, nil)
	}
	ctx.SetField(n.Name, val)
	return nil
}

func (n *ArrayNode) Encode(ctx *core.Context) error {
	length, err := ctx.GetSize(n.Size, n.SizeExpr)
	if err != nil {
		return errors.WithStack(err)
	}
	for i := 0; i < length; i++ {
		err = core.NodeEncode(ctx, n.Item...)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func registerArray() {
	core.RegisterNodeCompilerFactory[ArrayNode](core.NodeTypeArray, false)
}
