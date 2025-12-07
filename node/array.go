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

	item := yf.Item

	n.Item, err = core.NodeCompileWithRef(item.Ref, item.Fields, structures, true)
	if err != nil {
		return errors.Wrapf(err, "compile 'item' failed: %s", err.Error())
	}
	return nil
}

func (n *ArrayNode) Decode(ctx *core.Context) error {
	length, err := ctx.GetSize(n.Size, n.SizeExpr)
	if err != nil {
		return errors.WithStack(err)
	}
	for i := 0; i < length; i++ {
		err = core.NodeDecode(ctx, n.Item...)
		if err != nil {
			return errors.WithStack(err)
		}
	}
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
