package node

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

type IfNode struct {
	Condition *core.CelEvaluator
	Then      []core.Node
	Flow      string
}

func (n *IfNode) Compile(yf *core.YamlField, structures core.DataStructures) error {
	cond, err := core.CompileExpression(yf.Condition)
	if err != nil {
		return errors.WithStack(err)
	}
	thenNodes, err := core.NodeCompile(yf.Then, structures)
	if err != nil {
		return errors.WithStack(err)
	}
	n.Condition = cond
	n.Then = thenNodes
	n.Flow = yf.Flow
	return nil
}

func (this *IfNode) GetName() string {
	return "if"
}

func (this *IfNode) GetFlow() string {
	return this.Flow
}

func (n *IfNode) Decode(ctx *core.Context) error {
	res, err := n.Condition.Execute(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	if b, ok := res.(bool); ok && b {
		err = core.NodeDecode(ctx, n.Then...)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (n *IfNode) Encode(ctx *core.Context) error {
	res, err := n.Condition.Execute(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	if b, ok := res.(bool); ok && b {
		err = core.NodeEncode(ctx, n.Then...)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func registerIf() {
	core.RegisterNodeCompilerFactory[IfNode](core.NodeTypeIf, false)
}
