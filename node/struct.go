package node

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

type StructNode struct {
	core.BaseNode
	Ref    string
	Fields []core.Node
}

func (n *StructNode) Compile(yf *core.YamlField, structures core.DataStructures) (err error) {
	_ = n.BaseNode.Compile(yf, structures)
	n.Ref = yf.Ref

	if n.Ref == "" {
		return errors.Errorf("struct node ref should not be empty")
	}

	structure, ok := structures[n.Ref]
	if !ok {
		return errors.Errorf("struct ref '%s' not found", n.Ref)
	}
	n.Fields, err = core.NodeCompile(structure.Fields, structures)
	if err != nil {
		return errors.Wrapf(err, "struct fields compile failed: %s", err.Error())
	}
	return nil
}

func (n *StructNode) Decode(ctx *core.Context) error {
	return core.NodeDecode(ctx, n.Fields...)
}

func (n *StructNode) Encode(ctx *core.Context) error {
	return core.NodeEncode(ctx, n.Fields...)
}

func registerStruct() {
	core.RegisterNodeCompilerFactory[StructNode](core.NodeTypeStruct, false)
}
