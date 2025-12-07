package node

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

type SwitchNode struct {
	core.BaseNode
	FieldName   string
	Cases       map[any][]core.Node
	DefaultCase []core.Node
}

func (n *SwitchNode) Compile(yf *core.YamlField, structures core.DataStructures) (err error) {
	_ = n.BaseNode.Compile(yf, structures)
	n.FieldName = yf.Field
	n.Cases = make(map[any][]core.Node)

	// 编译所有 Cases (混合模式)
	for _, c := range yf.Cases {
		nodes, err := core.NodeCompileWithRef(c.Ref, c.Fields, structures, true)
		if err != nil {
			return errors.Wrapf(err, "switch case for value %v compile failure: %s", c.Value, err.Error())
		}

		var caseKey any
		if v, ok := c.Value.(int); ok {
			caseKey = uint64(v)
		} else {
			caseKey = c.Value
		}

		n.Cases[caseKey] = nodes
	}

	// 编译 Default Case (混合模式)
	if yf.DefaultRef != "" || len(yf.DefaultFields) > 0 {
		n.DefaultCase, err = core.NodeCompileWithRef(yf.DefaultRef, yf.DefaultFields, structures, false)
		if err != nil {
			return errors.Wrapf(err, "switch node '%s' default case compile failure: %s", n.Name, err.Error())
		}
	}

	return nil
}

func (this *SwitchNode) GetName() string {
	if this.Name != "" {
		return this.Name
	}
	return "switch"
}

func (n *SwitchNode) Decode(ctx *core.Context) error {
	nodes, err := n.getNodesToExecute(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	return core.NodeDecode(ctx, nodes...)
}

func (n *SwitchNode) Encode(ctx *core.Context) error {
	nodes, err := n.getNodesToExecute(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	// 执行节点编码
	return core.NodeEncode(ctx, nodes...)
}

func (n *SwitchNode) getNodesToExecute(ctx *core.Context) ([]core.Node, error) {
	switchValue, ok := ctx.GetField(n.FieldName)
	if !ok {
		return nil, errors.Errorf("switch field '%s' not found in context", n.FieldName)
	}

	var nodesToExecute []core.Node
	var caseKey any

	// Handle uint64 case keys for command parsing
	if uval, isUint := switchValue.(uint64); isUint {
		caseKey = uval
	} else {
		caseKey = switchValue
	}

	if nodes, found := n.Cases[caseKey]; found {
		nodesToExecute = nodes
	} else if n.DefaultCase != nil {
		nodesToExecute = n.DefaultCase
	} else {
		return nil, errors.Errorf("command value %v not supported for payload parsing, no default case defined", switchValue)
	}

	return nodesToExecute, nil
}

func registerSwitch() {
	core.RegisterNodeCompilerFactory[SwitchNode](core.NodeTypeSwitch, false)
}
