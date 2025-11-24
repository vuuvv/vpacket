package node

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

type CalcNode struct {
	Name    string
	Formula *core.CelEvaluator
}

func (this *CalcNode) GetName() string {
	return this.Name
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
	return nil
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
	core.RegisterNodeCompilerFactory[CalcNode](core.NodeTypeCalc, false)
}
