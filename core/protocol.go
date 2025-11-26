package core

import (
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

func (p *Protocol) Decode(packet []byte) (any, error) {
	ctx := NewContext(packet)
	ctx.Vars["packet_len"] = len(packet)
	err := NodeDecode(ctx, p.ParsedFields...)
	return ctx.Fields, err
}

func (p *Protocol) Encode(ctx *Context) ([]byte, error) {
	err := NodeEncode(ctx, p.ParsedFields...)
	return ctx.Writer.Bytes(), err
}
