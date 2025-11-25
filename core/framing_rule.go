package core

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/utils"
	"gopkg.in/yaml.v3"
)

type FramingRuleMatchResult struct {
	ProtocolName string
	Advance      int
	Token        []byte
	Error        error
}

type FramingRule interface {
	Split(data []byte) *FramingRuleMatchResult
	GetHeaderMarker() []byte
	Setup() error
}

var FramingRuleDecoders = make(map[string]FramingRuleDecodeFunc)

type FramingRuleDecodeFunc func(yamlNode *yaml.Node) (FramingRule, error)

func RegisterFramingRuleDecoderFactory[T any](name string) {
	fn := func(yamlNode *yaml.Node) (FramingRule, error) {
		var rule T
		err := yamlNode.Decode(&rule)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if ret, ok := utils.CastTo[FramingRule](&rule); ok {
			return ret, ret.Setup()
		}
		return nil, errors.Errorf("Framing rule type [%s] not match: %T", name, rule)
	}
	FramingRuleDecoders[name] = fn
}

func FramingRuleDecode(name string, yamlNode *yaml.Node) (FramingRule, error) {
	if fn, ok := FramingRuleDecoders[name]; ok {
		return fn(yamlNode)
	}
	return nil, errors.Errorf("Framing Decoder not found %s", name)
}
