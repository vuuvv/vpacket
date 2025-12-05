package node

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

type SwitchNode struct {
	FieldName   string
	Cases       map[any][]core.Node
	DefaultCase []core.Node
	Flow        string
}

func (n *SwitchNode) Compile(yf *core.YamlField, structures core.DataStructures) error {
	n.FieldName = yf.Field
	n.Flow = yf.Flow
	n.Cases = make(map[any][]core.Node)

	// 编译所有 Cases (混合模式)
	for _, c := range yf.Cases {
		var definition []*core.YamlField

		if c.Ref != "" { // 外部引用优先
			var ok bool
			structure, ok := structures[c.Ref]
			if !ok {
				return errors.Errorf("switch case ref '%s' not found", c.Ref)
			}
			definition = structure.Fields
		} else if len(c.Fields) > 0 { // 内联定义
			definition = c.Fields
		} else {
			return errors.Errorf("switch case for value %v requires either 'ref' or 'fields'", c.Value)
		}

		compiledNodes, err := core.NodeCompile(definition, structures)
		if err != nil {
			return errors.WithStack(err)
		}

		var caseKey any
		if v, ok := c.Value.(int); ok {
			caseKey = uint64(v)
		} else {
			caseKey = c.Value
		}

		n.Cases[caseKey] = compiledNodes
	}

	// 编译 Default Case (混合模式)
	if yf.DefaultRef != "" || len(yf.DefaultFields) > 0 {
		var defaultDefinition []*core.YamlField

		if yf.DefaultRef != "" { // 默认外部引用优先
			var ok bool
			structure, ok := structures[yf.DefaultRef]
			if !ok {
				return errors.Errorf("switch default ref '%s' not found", yf.DefaultRef)
			}
			defaultDefinition = structure.Fields
		} else if len(yf.DefaultFields) > 0 { // 默认内联定义
			defaultDefinition = yf.DefaultFields
		}

		n.DefaultCase, _ = core.NodeCompile(defaultDefinition, structures)
	}

	return nil
}

func (this *SwitchNode) GetName() string {
	return "switch"
}

func (this *SwitchNode) GetFlow() string {
	return this.Flow
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
