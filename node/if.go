package node

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

const If = "if"

type IfNode struct {
	Condition *core.CelEvaluator
	Then      []core.Node
}

func (n *IfNode) Decode(ctx *core.Context) error {
	res, err := n.Condition.Execute(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	if b, ok := res.(bool); ok && b {
		for _, node := range n.Then {
			if err := node.Decode(ctx); err != nil {
				return errors.WithStack(err)
			}
		}
	}
	return nil
}

func (n *IfNode) Encode(input map[string]any, writer *core.BitWriter) error {

	//TODO implement me
	panic("implement me")
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
	return nil
}

func registerIf() {
	core.RegisterNodeCompilerFactory[IfNode](If, false)
}
