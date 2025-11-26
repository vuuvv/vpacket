package framing

import (
	"bytes"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/utils"
)

const Text = "text"

type TextRule struct {
	StartDelimiter      string `yaml:"start_delimiter"`
	EndDelimiter        string `yaml:"end_delimiter"`     // 结束符号
	ContainDelimiter    bool   `yaml:"contain_delimiter"` // 返回的token是否包含分隔符
	Length              int    `yaml:"length"`            // 固定长度, 和EndDelimiter须有一个有值, 如果设置了length,可以不设置max_len
	MaxLen              int    `yaml:"max_len"`
	headerMarkerBytes   []byte
	startDelimiterBytes []byte
	endDelimiterBytes   []byte
}

func (this *TextRule) Setup() (err error) {
	if this.StartDelimiter == "" {
		return errors.New("BinaryRule.Setup: start_delimiter should not be empty")
	}
	if this.EndDelimiter == "" && this.Length == 0 {
		return errors.New("TextRule.Setup: end_delimiter or max_len should not both be empty/null")
	}

	if this.MaxLen == 0 && this.Length == 0 {
		return errors.New("TextRule.Setup: max_len or length should not both be 0/empty/null")
	}
	this.startDelimiterBytes, err = utils.ParseTValue(this.StartDelimiter, -1, nil)
	if err != nil {
		return err
	}
	this.headerMarkerBytes = this.startDelimiterBytes

	this.endDelimiterBytes, err = utils.ParseTValue(this.EndDelimiter, -1, nil)
	if err != nil {
		return err
	}
	return nil
}

func (this *TextRule) Split(data []byte) *core.FramingRuleMatchResult {
	if len(data) < len(this.startDelimiterBytes) {
		// 这里应该是不会发生的
		return core.ErrorFramingRuleMatchResult(
			errors.Errorf("invalid text packet: packet length  [%d] litte then start_delimiter length [%d]", len(data), len(this.startDelimiterBytes)),
		)
	}

	if this.Length > 0 {
		if len(data) < this.Length {
			// 长度不足,不消费，等待
			return core.WaitFramingRuleMatchResult()
		} else {
			var ret []byte
			if this.ContainDelimiter {
				ret = data[:this.Length]
			} else {
				ret = data[len(this.startDelimiterBytes):this.Length]
			}
			return core.NewFramingRuleMatchResult(this.Length, ret)
		}
	}

	idx := bytes.Index(data, this.endDelimiterBytes)

	if idx >= 0 {
		totalLen := idx + len(this.endDelimiterBytes)
		// TODO: 需不需要判断超长
		var ret []byte
		if this.ContainDelimiter {
			ret = data[:totalLen]
		} else {
			ret = data[len(this.startDelimiterBytes):idx]
		}

		return core.NewFramingRuleMatchResult(totalLen, ret)
	}

	if len(data) > this.MaxLen {
		// 超长了就应该abandon
		return core.AbandonFramingRuleMatchResult(1, data)
	}
	// 否则等待

	return core.WaitFramingRuleMatchResult()
}

func (this *TextRule) GetHeaderMarker() []byte {
	return this.headerMarkerBytes
}

func registerText() {
	core.RegisterFramingRuleDecoderFactory[TextRule](Text)
}
