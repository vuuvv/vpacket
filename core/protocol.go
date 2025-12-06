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
	Round             int
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
	for _, field := range fields {
		r := field.GetRound()
		if r > p.Round {
			p.Round = r
		}
	}
	return nil
}

func (p *Protocol) Decode(packet []byte) (any, error) {
	ctx := NewContext(packet)
	ctx.Flow = FlowDecode
	ctx.Vars["packetLen"] = len(packet)
	err := NodeDecode(ctx, p.ParsedFields...)
	return ctx.Fields, err
}

func (p *Protocol) Encode(ctx *Context) ([]byte, error) {
	for i := 0; i <= p.Round; i++ {
		ctx.NodeIndex = 0
		ctx.Round = i
		err := NodeEncode(ctx, p.ParsedFields...)
		if i == 0 {
			bs := ctx.Writer.Bytes()
			ctx.Vars["packetLen"] = len(bs)
			ctx.Data = bs
		}
		if err != nil {
			return ctx.Data, errors.WithStack(err)
		}
	}
	return ctx.Data, nil
}
