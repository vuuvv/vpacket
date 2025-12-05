package node

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

type CalcNode struct {
	Name    string
	Flow    string
	Formula *core.CelEvaluator
}

func (n *CalcNode) Compile(yf *core.YamlField, structures core.DataStructures) error {
	expr, err := core.CompileExpression(yf.Formula)
	if err != nil {
		return errors.WithStack(err)
	}
	n.Name = yf.Name
	n.Flow = yf.Flow
	n.Formula = expr
	return nil
}

func (this *CalcNode) GetName() string {
	return this.Name
}
func (this *CalcNode) GetFlow() string {
	return this.Flow
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

func registerCalc() {
	core.RegisterNodeCompilerFactory[CalcNode](core.NodeTypeCalc, false)
}
