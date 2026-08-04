package core

import (
	"bytes"
	"encoding/json"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/utils"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"time"
)

type ScanResult struct {
	Sn          string     `json:"sn"`                 // 直接连接的设备序列号
	Abaddon     bool       `json:"abaddon,omitempty"`  // 是否为丢弃的包
	Packet      []byte     `json:"packet,omitempty"`   // 为解析的原始包
	Protocol    *Protocol  `json:"protocol,omitempty"` // 使用的协议
	Data        any        `json:"data,omitempty"`     // 解析出来的数据
	ScanError   error      `json:"scanError,omitempty"`
	HandleError error      `json:"handleError,omitempty"`
	Start       *time.Time `json:"start,omitempty"`
	End         *time.Time `json:"end,omitempty"`
}

func (this *ScanResult) Run(fn ScanResultHandler) {
	if this == nil {
		return
	}
	err := fn(this)
	if err != nil {
		this.HandleError = err
	}
	if this.End != nil {
		now := time.Now()
		this.End = &now
	}
}

type ScanResultHandler func(result *ScanResult) error

type Codec struct {
	scheme  *Scheme
	stream  io.Reader
	history *utils.LockFreeCircularBuffer
}

func NewCodec() *Codec {
	scanner := &Codec{}
	scanner.history = utils.NewLockFreeCircularBuffer(10)
	return scanner
}

func NewCodecFromBytes(configBytes []byte) (*Codec, error) {
	scanner := NewCodec()
	err := yaml.Unmarshal(configBytes, &scanner.scheme)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	err = scanner.scheme.Setup()
	return scanner, err
}

func NewCodecFromFile(configFile string) (*Codec, error) {
	scanner := NewCodec()
	f, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	scanner.scheme = &Scheme{}
	err = yaml.NewDecoder(f).Decode(scanner.scheme)
	if err != nil {
		return nil, err
	}
	err = scanner.scheme.Setup()
	return scanner, err
}

func (this *Codec) Config(config *Scheme) *Codec {
	this.scheme = config
	return this
}

func (this *Codec) Stream(stream io.Reader) *Codec {
	this.stream = stream
	return this
}

func (this *Codec) AddHistory(history any) *Codec {
	this.history.Add(&utils.WithTime{Time: time.Now(), Data: history})
	return this
}

func (this *Codec) Histories() []*utils.WithTime {
	histories := this.history.GetAll()
	var res []*utils.WithTime
	for _, h := range histories {
		if t, ok := h.(*utils.WithTime); ok {
			res = append(res, t)
		}
	}
	return res
}

func (this *Codec) Encode(input map[string]any) ([]byte, error) {
	if len(this.scheme.Protocols) < 1 {
		return nil, errors.New("No Protocols configured")
	}
	protocol := this.scheme.Protocols[0]

	ctx := NewContext(nil)
	ctx.Fields = input
	ctx.Flow = FlowEncode
	return protocol.Encode(ctx)
}

func (this *Codec) EncodeFromJson(input string) ([]byte, error) {
	data := make(map[string]any)
	err := json.Unmarshal([]byte(input), &data)
	if err != nil {
		return nil, err
	}
	return this.Encode(data)
}

func (this *Codec) Scan(fn ScanResultHandler) error {
	scanner := utils.NewScanner(this.stream)
	scanner.Split(this.Splitter(scanner))

	for scanner.Scan() {
		result := this.scanResult(scanner)
		if result == nil {
			continue
		}
		this.EmitResult(result, fn)
	}

	// 解码结束, 移除结果
	scanner.SetResult(nil)

	// 如果scanner.err是EOF错误,scanner.Err()返回的错误是空,代表不是错误,是正常结束
	if err := scanner.Err(); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// ScanSync 与 Scan 使用完全相同的分帧器和协议解码流程，但在当前调用栈
// 同步执行回调。调试注入需要在 HTTP/流程节点返回前拿到所有 messageHandle
// 错误；如果复用 Scan 的异步回调，接口会在下游尚未执行时提前返回，导致
// 调试结果丢失且错误无法反馈给调用方。
func (this *Codec) ScanSync(fn ScanResultHandler) error {
	if fn == nil {
		return errors.New("同步扫描回调不能为空")
	}

	scanner := utils.NewScanner(this.stream)
	scanner.Split(this.Splitter(scanner))
	defer scanner.SetResult(nil)

	for scanner.Scan() {
		result := this.scanResult(scanner)
		if result == nil {
			continue
		}
		this.history.Add(result)
		result.Run(fn)
		if result.HandleError != nil {
			// 同步路径必须把回调（通常是 messageHandle）返回的错误交给
			// 调用方，否则调试接口会误报成功而掩盖下游处理失败。
			return errors.WithStack(result.HandleError)
		}
	}

	if err := scanner.Err(); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// scanResult 从 Scanner 当前结果构建 ScanResult，并负责唯一的一份协议
// Decode 调用。Scan 和 ScanSync 都必须经过这里，避免两条路径的分帧、错误
// 或字段解析语义逐渐漂移。
func (this *Codec) scanResult(scanner *utils.Scanner) *ScanResult {
	scannerResult := scanner.Result()
	if scannerResult == nil {
		return nil
	}
	framingRuleResult, ok := scannerResult.(*FramingRuleMatchResult)
	if !ok {
		return nil
	}

	result := &ScanResult{
		Abaddon:   framingRuleResult.Abandoned,
		Packet:    framingRuleResult.Token,
		Protocol:  framingRuleResult.Protocol,
		ScanError: framingRuleResult.Error,
		Start:     &framingRuleResult.Time,
	}
	if result.ScanError != nil || result.Abaddon {
		return result
	}
	data, err := result.Protocol.Decode(result.Packet)
	result.Data = data
	if err != nil {
		result.ScanError = err
	}
	return result
}

func (this *Codec) EmitResult(result *ScanResult, fn ScanResultHandler) {
	this.history.Add(result)
	// 因为是指针,所有后面的修改会影响history中的数据
	go result.Run(fn)
}

func (this *Codec) Splitter(scanner *utils.Scanner) utils.SplitFunc {
	type Matcher struct {
		Def    *Protocol
		Marker []byte
	}
	var matchers []Matcher

	for _, p := range this.scheme.Protocols {
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
					res.Protocol = m.Def
					scanner.SetResult(res)
					return res.Advance, res.Token, res.Error
				}
			}
		}

		// 没有匹配到就丢弃第一个数据
		res = AbandonFramingRuleMatchResult(1, data)
		scanner.SetResult(res)
		return 1, []byte{data[0]}, nil // 脏数据处理
	}
}
