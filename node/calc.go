package node

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

const Calc = "calc"

type CalcNode struct {
	Name    string
	Formula *core.CelEvaluator
}

func (n *CalcNode) Decode(ctx *core.Context) error {
	res, err := n.Formula.Execute(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	ctx.Fields[n.Name] = res
	return nil
}

func (n *CalcNode) Encode(input map[string]any, writer *core.BitWriter) error {
	//TODO implement me
	panic("implement me")
}

func (n *CalcNode) Compile(yf *core.YamlField, structures core.DataStructures) error {
	expr, err := core.CompileExpression(yf.Formula)
	if err != nil {
		return errors.WithStack(err)
	}
	n.Name = yf.Name
	n.Formula = expr
	return nil
}

func registerCalc() {
	core.RegisterNodeCompilerFactory[CalcNode](Calc, false)
}
