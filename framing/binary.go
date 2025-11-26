package framing

import (
	"encoding/binary"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/utils"
)

const Binary = "binary"

type BinaryRule struct {
	HeaderMarker      string `yaml:"header_marker"` // 分隔符,Hex
	MinHeaderSize     int    `yaml:"min_header_size"`
	LengthOffset      int    `yaml:"length_offset"`
	LengthSize        int    `yaml:"length_size"`
	LengthAdjustment  int    `yaml:"length_adjustment"`
	headerMarkerBytes []byte
}

func (this *BinaryRule) Setup() (err error) {
	if this.LengthSize == 0 {
		return errors.New("BinaryRule.Setup: length size should not be zero")
	}
	if this.LengthAdjustment == 0 {
		return errors.New("BinaryRule.Setup: length adjustment should not be zero")
	}
	if this.HeaderMarker == "" {
		return errors.New("BinaryRule.Setup: no header marker")
	}
	this.headerMarkerBytes, err = utils.ParseTValue(this.HeaderMarker, -1, nil)
	if err != nil {
		return errors.Wrapf(err, "BinaryRule.Setup: invalid header marker: %s, should be valid hex format. eg: 7a7b", this.HeaderMarker)
	}
	return nil
}

func (this *BinaryRule) Split(data []byte) *core.FramingRuleMatchResult {
	if len(data) < this.MinHeaderSize {
		return core.WaitFramingRuleMatchResult()
	}

	var bodyLen int
	if len(data) < this.LengthOffset+this.LengthSize {
		return core.WaitFramingRuleMatchResult()
	}
	lenBytes := data[this.LengthOffset : this.LengthOffset+this.LengthSize]
	bodyLen = int(binary.BigEndian.Uint16(lenBytes))

	totalLen := this.LengthAdjustment + bodyLen

	if len(data) < totalLen {
		return nil
	}

	return core.NewFramingRuleMatchResult(totalLen, data[:totalLen])
}

func (this *BinaryRule) GetHeaderMarker() []byte {
	return this.headerMarkerBytes
}

func registerBinary() {
	core.RegisterFramingRuleDecoderFactory[BinaryRule](Binary)
}
