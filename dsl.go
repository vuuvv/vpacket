package vpacket

//
//import (
//	"bufio"
//	"bytes"
//	"encoding/hex"
//	"fmt"
//	"github.com/vuuvv/errors"
//	"github.com/vuuvv/vpacket/core"
//	"github.com/vuuvv/vpacket/framing"
//	"gopkg.in/yaml.v3"
//)
//
//func Setup() {
//	framing.Register()
//}
//
//// ==========================================
//// 4. Compiler (YAML -> Nodes)
//// ==========================================
//
//// --- YAML 结构定义 ---
//type DataStructure struct {
//	Fields []*YamlField `yaml:"fields"`
//}
//
//type DataStructures map[string]*DataStructure
//
//type YamlSwitchCase struct {
//	Value  any          `yaml:"value"`
//	Ref    string       `yaml:"ref"`    // 外部引用
//	Fields []*YamlField `yaml:"fields"` // 内联定义
//}
//
//type YamlField struct {
//	Name      string       `yaml:"name"`
//	Bits      int          `yaml:"bits"`
//	Type      string       `yaml:"type"`
//	Size      int          `yaml:"size"`
//	SizeExpr  string       `yaml:"size_expr"`
//	Check     string       `yaml:"check"`
//	Condition string       `yaml:"condition"`
//	Formula   string       `yaml:"formula"`
//	Then      []*YamlField `yaml:"then"`
//
//	// Switch 相关的字段
//	Field         string            `yaml:"field"`
//	Cases         []*YamlSwitchCase `yaml:"cases"`
//	DefaultRef    string            `yaml:"default_ref"`    // 默认外部引用
//	DefaultFields []*YamlField      `yaml:"default_fields"` // 默认内联定义
//}
//
//// Compile 函数签名更新：必须传入 DataStructures
//func Compile(fields []*YamlField, structures DataStructures) ([]core.Node, error) {
//	var nodes []core.Node
//	for _, yf := range fields {
//		node, err := compileSingle(yf, structures)
//		if err != nil {
//			return nil, errors.WithStack(err)
//		}
//		nodes = append(nodes, node)
//	}
//	return nodes, nil
//}
//
//func compileSingle(yf *YamlField, structures DataStructures) (core.Node, error) {
//	switch yf.Type {
//	case "bytes":
//		node := &BytesNode{Name: yf.Name, Size: yf.Size}
//		if yf.SizeExpr != "" {
//			expr, err := CompileExpression(yf.SizeExpr)
//			if err != nil {
//				return nil, errors.WithStack(err)
//			}
//			node.SizeExpr = expr
//		}
//		return node, nil
//	case "calc":
//		expr, err := CompileExpression(yf.Formula)
//		if err != nil {
//			return nil, errors.WithStack(err)
//		}
//		return &CalcNode{Name: yf.Name, Formula: expr}, nil
//	case "if":
//		cond, err := CompileExpression(yf.Condition)
//		if err != nil {
//			return nil, errors.WithStack(err)
//		}
//		thenNodes, err := Compile(yf.Then, structures)
//		if err != nil {
//			return nil, errors.WithStack(err)
//		}
//		return &IfNode{Condition: cond, Then: thenNodes}, nil
//
//	case "switch":
//		s := &SwitchNode{FieldName: yf.Field, Cases: make(map[any][]Node)}
//
//		// 编译所有 Cases (混合模式)
//		for _, c := range yf.Cases {
//			var definition []*YamlField
//
//			if c.Ref != "" { // 外部引用优先
//				var ok bool
//				structure, ok := structures[c.Ref]
//				if !ok {
//					return nil, fmt.Errorf("switch case ref '%s' not found", c.Ref)
//				}
//				definition = structure.Fields
//			} else if len(c.Fields) > 0 { // 内联定义
//				definition = c.Fields
//			} else {
//				return nil, fmt.Errorf("switch case for value %v requires either 'ref' or 'fields'", c.Value)
//			}
//
//			compiledNodes, err := Compile(definition, structures)
//			if err != nil {
//				return nil, errors.WithStack(err)
//			}
//
//			var caseKey any
//			if v, ok := c.Value.(int); ok {
//				caseKey = uint64(v)
//			} else {
//				caseKey = c.Value
//			}
//
//			s.Cases[caseKey] = compiledNodes
//		}
//
//		// 编译 Default Case (混合模式)
//		if yf.DefaultRef != "" || len(yf.DefaultFields) > 0 {
//			var defaultDefinition []*YamlField
//
//			if yf.DefaultRef != "" { // 默认外部引用优先
//				var ok bool
//				structure, ok := structures[yf.DefaultRef]
//				if !ok {
//					return nil, fmt.Errorf("switch default ref '%s' not found", yf.DefaultRef)
//				}
//				defaultDefinition = structure.Fields
//			} else if len(yf.DefaultFields) > 0 { // 默认内联定义
//				defaultDefinition = yf.DefaultFields
//			}
//
//			s.DefaultCase, _ = Compile(defaultDefinition, structures)
//		}
//
//		return s, nil
//
//		//default:
//		//	node := &BitFieldNode{Name: yf.Name, Bits: yf.Bits}
//		//	if yf.Check != "" {
//		//		chk, err := CompileExpression(yf.Check)
//		//		if err != nil {
//		//			return nil, errors.WithStack(err)
//		//		}
//		//		node.Check = chk
//		//	}
//		//	return node, nil
//	}
//}
//
//// ==========================================
//// 5. Modular Splitter (分包器)
//// ==========================================
//
//type ProtocolDefinition struct {
//	Name         string       `yaml:"name"`
//	Type         string       `yaml:"type"`
//	FramingRule yaml.Node    `yaml:"framing_rules"`
//	Fields       []*YamlField `yaml:"fields"`
//	ParsedFramingRule  core.FramingRule
//}
//
//type RootConfig struct {
//	Protocols      []*ProtocolDefinition `yaml:"protocols"`
//	DataStructures DataStructures        `yaml:"data_structures"` // 保持
//}
//
//// Setup 获取分包规则
//func (p *ProtocolDefinition) Setup() error {
//	rule, err := core.FramingRuleDecode(p.Type, &p.FramingRule)
//	if err != nil {
//		return errors.WithStack(err)
//	}
//	p.ParsedFramingRule = rule
//	return nil
//}
//
//func NewModularSplitter(protocols []*ProtocolDefinition) bufio.SplitFunc {
//	type Matcher struct {
//		Def    *ProtocolDefinition
//		Marker []byte
//	}
//	var matchers []Matcher
//
//	for _, p := range protocols {
//		marker := p.ParsedFramingRule.GetHeaderMarker()
//
//		if len(marker) > 0 {
//			markerBytes, _ := hex.DecodeString(marker)
//			if len(markerBytes) > 0 {
//				matchers = append(matchers, Matcher{Def: p, Marker: markerBytes})
//			}
//		}
//	}
//
//	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
//		if len(data) == 0 {
//			return 0, nil, nil
//		}
//		var res core.FramingRuleMatchResult
//
//		for _, m := range matchers {
//			if bytes.HasPrefix(data, m.Marker) {
//				res = m.Def.ParsedFramingRule.Split(data)
//
//				if res.Advance > 0 || res.Error != nil {
//					res.ProtocolName = m.Def.Name
//					return res.Advance, res.Token, res.Error
//				}
//			}
//		}
//
//		return 1, []byte{data[0]}, nil // 脏数据处理
//	}
//}
