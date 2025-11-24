package core

import (
	"bufio"
	"bytes"
	"github.com/vuuvv/errors"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"time"
)

type ScanResult struct {
	Abaddon     bool      `json:"abaddon,omitempty"`
	Packet      []byte    `json:"packet,omitempty"`
	Protocol    *Protocol `json:"protocol,omitempty"`
	Data        any       `json:"data,omitempty"`
	ScanError   error     `json:"scanError,omitempty"`
	HandleError error     `json:"handleError,omitempty"`
	Start       time.Time `json:"start,omitempty"`
	End         time.Time `json:"end,omitempty"`
}

type ScanResultHandler func(result *ScanResult) error

type Scanner struct {
	config  *Config
	stream  io.Reader
	history *LockFreeCircularBuffer
}

func NewScanner() *Scanner {
	scanner := &Scanner{}
	scanner.history = NewLockFreeCircularBuffer(10)
	return scanner
}

func NewScannerFromBytes(configBytes []byte) (*Scanner, error) {
	scanner := NewScanner()
	err := yaml.Unmarshal(configBytes, &scanner.config)
	if err != nil {
		return nil, err
	}
	err = scanner.config.Setup()
	return scanner, err
}

func NewScannerFromFile(configFile string) (*Scanner, error) {
	scanner := NewScanner()
	f, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	scanner.config = &Config{}
	err = yaml.NewDecoder(f).Decode(scanner.config)
	if err != nil {
		return nil, err
	}
	err = scanner.config.Setup()
	return scanner, err
}

func (scanner *Scanner) Config(config *Config) *Scanner {
	scanner.config = config
	return scanner
}

func (scanner *Scanner) Stream(stream io.Reader) *Scanner {
	scanner.stream = stream
	return scanner
}

func (scanner *Scanner) AddHistory(history any) *Scanner {
	scanner.history.Add(&WithTime{Time: time.Now(), Data: history})
	return scanner
}

func (scanner *Scanner) Histories() []*WithTime {
	histories := scanner.history.GetAll()
	var res []*WithTime
	for _, h := range histories {
		if t, ok := h.(*WithTime); ok {
			res = append(res, t)
		}
	}
	return res
}

func (this *Scanner) Scan(fn ScanResultHandler) error {
	scanner := bufio.NewScanner(this.stream)
	scanner.Split(this.Splitter())

	for scanner.Scan() {
		packet := scanner.Bytes()
		result := &ScanResult{Packet: packet, Start: time.Now()}

		if len(packet) == 1 {
			result.Abaddon = true
			this.EmitResult(result, fn)
			continue
		}

		protocol := this.config.FindProtocol(packet)
		if protocol == nil {
			result.ScanError = errors.New("protocol not found")
			this.EmitResult(result, fn)
			continue
		}
		result.Protocol = protocol

		data, err := protocol.Parse(packet)
		result.Data = data
		if err != nil {
			result.ScanError = err
			this.EmitResult(result, fn)
			continue
		}
		this.EmitResult(result, fn)
	}

	if err := scanner.Err(); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (this *Scanner) EmitResult(result *ScanResult, fn ScanResultHandler) {
	this.history.Add(result)
	// 因为是指针,所有后面的修改会影响history中的数据
	err := fn(result)
	if err != nil {
		result.HandleError = err
	}
	result.End = time.Now()
}

func (scanner *Scanner) Splitter() bufio.SplitFunc {
	type Matcher struct {
		Def    *Protocol
		Marker []byte
	}
	var matchers []Matcher

	for _, p := range scanner.config.Protocols {
		marker := p.ParsedFramingRule.GetHeaderMarker()

		if len(marker) > 0 {
			matchers = append(matchers, Matcher{Def: p, Marker: marker})
		}
	}

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if len(data) == 0 {
			return 0, nil, nil
		}
		var res *FramingRuleMatchResult

		for _, m := range matchers {
			if bytes.HasPrefix(data, m.Marker) {
				res = m.Def.ParsedFramingRule.Split(data)

				if res != nil && (res.Advance > 0 || res.Error != nil) {
					res.ProtocolName = m.Def.Name
					return res.Advance, res.Token, res.Error
				}
			}
		}

		return 1, []byte{data[0]}, nil // 脏数据处理
	}
}
