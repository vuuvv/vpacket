package node

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

type CalcNode struct {
	core.BaseNode
	core.BaseEncodable
	Formula  *core.CelEvaluator
	Size     int                // 一般用于encode
	SizeExpr *core.CelEvaluator // 一般用于encode
}

func (n *CalcNode) Compile(yf *core.YamlField, structures core.DataStructures) error {
	_ = n.BaseNode.Compile(yf, structures)
	err := n.BaseEncodable.Compile(yf, structures)
	if err != nil {
		return errors.WithStack(err)
	}

	n.Size = yf.Size
	if yf.SizeExpr != "" {
		expr, err := core.CompileExpression(yf.SizeExpr)
		if err != nil {
			return errors.Wrapf(err, "Compile 'size_expr' of field %s: %s", n.Name, err.Error())
		}
		n.SizeExpr = expr
	}

	expr, err := core.CompileExpression(yf.Formula)
	if err != nil {
		return errors.WithStack(err)
	}
	n.Formula = expr
	return nil
}

func (n *CalcNode) Decode(ctx *core.Context) error {
	res, err := n.Formula.Execute(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	ctx.SetField(n.Name, res)
	return nil
}

func (n *CalcNode) Encode(ctx *core.Context) error {
	if ctx.Round > n.GetRound() { // 编译的轮次大于节点轮次，跳过
		return nil
	}

	size, err := ctx.GetSize(n.Size, n.SizeExpr)
	if err != nil {
		return errors.WithStack(err)
	}

	if ctx.Round < n.GetRound() { // 编译的轮次小于节点轮次，写入占位符
		return ctx.WritePlaceholder(size)
	}

	val, err := n.Formula.Execute(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	switch v := val.(type) {
	case []byte:
		return ctx.WriteBytes(v[:size])
	case int:
		return ctx.Write(core.NodeTypeInt, v, size, n)
	case int8:
		return ctx.Write(core.NodeTypeInt, v, size, n)
	case int16:
		return ctx.Write(core.NodeTypeInt, v, size, n)
	case int32:
		return ctx.Write(core.NodeTypeInt, v, size, n)
	case int64:
		return ctx.Write(core.NodeTypeInt, v, size, n)
	case uint:
		return ctx.Write(core.NodeTypeInt, v, size, n)
	case uint8:
		return ctx.Write(core.NodeTypeInt, v, size, n)
	case uint16:
		return ctx.Write(core.NodeTypeInt, v, size, n)
	case uint32:
		return ctx.Write(core.NodeTypeInt, v, size, n)
	case uint64:
		return ctx.Write(core.NodeTypeInt, v, size, n)
	case string:
		return ctx.Write(core.NodeTypeString, v, size, n)
	case float64:
		return ctx.Write(core.NodeTypeFloat, v, size, n)
	case float32:
		return ctx.Write(core.NodeTypeFloat, v, size, n)
	}
	return nil
}

func registerCalc() {
	core.RegisterNodeCompilerFactory[CalcNode](core.NodeTypeCalc, false)
}
