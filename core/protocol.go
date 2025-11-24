package core

import (
	"bytes"
	"github.com/vuuvv/errors"
	"gopkg.in/yaml.v3"
)

type Protocol struct {
	Name              string       `yaml:"name"`
	Type              string       `yaml:"type"`
	FramingRule       yaml.Node    `yaml:"framing_rule"`
	Fields            []*YamlField `yaml:"fields"`
	ParsedFramingRule FramingRule
	ParsedFields      []Node
}

// Setup 获取分包规则
func (p *Protocol) Setup(structures DataStructures) error {
	if p.FramingRule.IsZero() {
		return errors.New("framing_rule not set")
	}
	rule, err := FramingRuleDecode(p.Type, &p.FramingRule)
	if err != nil {
		return errors.WithStack(err)
	}
	p.ParsedFramingRule = rule

	fields, err := NodeCompile(p.Fields, structures)
	if err != nil {
		return errors.WithStack(err)
	}

	p.ParsedFields = fields
	return nil
}

func (p *Protocol) CanParse(token []byte) bool {
	if p.ParsedFramingRule == nil {
		return false
	}
	return bytes.HasPrefix(token, p.ParsedFramingRule.GetHeaderMarker())
}

func (p *Protocol) Parse(packet []byte) (any, error) {
	ctx := NewContext(packet)
	ctx.Vars["packet_len"] = len(packet)
	for _, node := range p.ParsedFields {
		if err := node.Decode(ctx); err != nil {
			return ctx.Fields, errors.WithStack(err)
		}
	}
	return ctx.Fields, nil
}

func (p *Protocol) Encode(ctx *Context) ([]byte, error) {
	for _, node := range p.ParsedFields {
		if err := node.Encode(ctx); err != nil {
			return nil, errors.Wrapf(err, "Encode field %s: %s", node.GetName(), err.Error())
		}
	}
	return ctx.Writer.Bytes(), nil
}
