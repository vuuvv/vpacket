package core

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/utils"
	"gopkg.in/yaml.v3"
)

type FramingRuleMatchResult struct {
	Protocol  *Protocol
	Abandoned bool
	Advance   int
	Token     []byte
	Error     error
}

func NewFramingRuleMatchResult(advance int, token []byte) *FramingRuleMatchResult {
	return &FramingRuleMatchResult{
		Advance: advance,
		Token:   token,
	}
}

// AbandonFramingRuleMatchResult 表示丢弃一个数据
func AbandonFramingRuleMatchResult(size int, data []byte) *FramingRuleMatchResult {
	return &FramingRuleMatchResult{
		Abandoned: true,
		Advance:   size,
		Token:     []byte{data[0]},
	}
}

func WaitFramingRuleMatchResult() *FramingRuleMatchResult {
	return &FramingRuleMatchResult{}
}

func ErrorFramingRuleMatchResult(err error) *FramingRuleMatchResult {
	return &FramingRuleMatchResult{Error: err}
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
