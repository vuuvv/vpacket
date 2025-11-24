package framing

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
)

const Text = "text"

type TextRule struct {
	HeaderMarker      string `yaml:"header_marker"`
	StartDelimiter    string `yaml:"start_delimiter"`
	EndDelimiter      string `yaml:"end_delimiter"` // 结束符号
	Length            int    `yaml:"length"`        // 固定长度, 和EndDelimiter须有一个有值
	MaxLen            int    `yaml:"max_len"`
	headerMarkerBytes []byte
}

func (this *TextRule) Setup() (err error) {
	if this.StartDelimiter == "" {
		return errors.New("BinaryRule.Setup: start_delimiter should not be empty")
	}
	if this.EndDelimiter == "" {
		return errors.New("BinaryRule.Setup: end_delimiter should not be empty")
	}
	if this.HeaderMarker == "" {
		return errors.New("BinaryRule.Setup: no header marker")
	}
	this.headerMarkerBytes, err = hex.DecodeString(this.HeaderMarker)
	if err != nil {
		return errors.Wrapf(err, "BinaryRule.Setup: invalid header marker: %s, should be valid hex format. eg: 7a7b", this.HeaderMarker)
	}
	return nil
}

func (this *TextRule) Split(data []byte) *core.FramingRuleMatchResult {
	endDelimiter := []byte(this.EndDelimiter)
	idx := bytes.Index(data, endDelimiter)

	if idx >= 0 {
		totalLen := idx + len(endDelimiter)
		return &core.FramingRuleMatchResult{Advance: totalLen, Token: data[:totalLen]}
	}

	if len(data) > this.MaxLen {
		return &core.FramingRuleMatchResult{Error: fmt.Errorf("text packet exceeds max length %d", this.MaxLen)}
	}

	return nil
}

func (this *TextRule) GetHeaderMarker() []byte {
	return this.headerMarkerBytes
}

func (this *TextRule) CanParse(token []byte) bool {
	return bytes.HasPrefix(token, []byte(this.HeaderMarker))
}

func registerText() {
	core.RegisterFramingRuleDecoderFactory[TextRule](Text)
}
