package vpacket

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vuuvv/vpacket/core"
)

const rsBoardTestUID = "FE0102030405060708090AFE"

type rsBoardVector struct {
	name         string
	command      string
	write        bool
	deviceReport bool
	data         map[string]any
}

func newRSBoardV2Codec(t *testing.T) *Codec {
	t.Helper()
	Setup()
	config, err := os.ReadFile("resources/protocols-v2.yaml")
	if err != nil {
		t.Fatalf("读取 RS 协议配置失败: %v", err)
	}
	codec, err := NewCodecFromBytes(config)
	if err != nil {
		t.Fatalf("编译 RS 协议配置失败: %v", err)
	}
	return codec
}

func rsData(values ...any) map[string]any {
	result := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		result[values[i].(string)] = values[i+1]
	}
	return result
}

func rsBoardInput(vector rsBoardVector) map[string]any {
	input := map[string]any{
		"sn":      rsBoardTestUID,
		"command": vector.command,
		"counter": "2A",
		"data":    vector.data,
	}
	if vector.write {
		input["write"] = true
	}
	return input
}

func rsBoardDirection(vector rsBoardVector) string {
	if vector.deviceReport {
		return "CC"
	}
	if vector.write {
		return "AA"
	}
	return "BB"
}

func encodeRSBoardVector(t *testing.T, codec *Codec, vector rsBoardVector) []byte {
	t.Helper()
	encoder := codec
	input := rsBoardInput(vector)
	if vector.deviceReport {
		// CC 是设备入站方向，正式编码接口只允许按 write 生成 BB/AA。
		// 为覆盖解码分支，测试使用等价的测试编码器构造设备回包，再交给真实 v2 配置解码。
		encoder = newRSBoardResponseEncoder(t)
		input["direction"] = rsBoardDirection(vector)
	}
	packet, err := encoder.Encode(input)
	if err != nil {
		t.Fatalf("%s 编码失败: %v", vector.name, err)
	}
	return packet
}

func newRSBoardResponseEncoder(t *testing.T) *Codec {
	t.Helper()
	config, err := os.ReadFile("resources/protocols-v2.yaml")
	if err != nil {
		t.Fatalf("读取 RS 协议配置失败: %v", err)
	}
	from := `      - name: "direction"
        flow: "encode"
        type: "calc"
        value_type: "hex"
        size: 1
        # 编码默认走查询方向 BB；只有调用方明确 write=true 时才允许生成写指令 AA。
        formula: "has(fields.write) && fields.write == true ? 'AA' : 'BB'"`
	to := `      - name: "direction"
        flow: "encode"
        type: "hex"
        size: 1`
	content := strings.Replace(string(config), from, to, 1)
	if content == string(config) {
		t.Fatal("未找到方向编码规则，无法构造设备 CC 回包测试夹具")
	}
	codec, err := NewCodecFromBytes([]byte(content))
	if err != nil {
		t.Fatalf("编译设备回包测试夹具失败: %v", err)
	}
	return codec
}

func assertRSBoardFrame(t *testing.T, packet []byte, vector rsBoardVector) {
	t.Helper()
	// 固定检查 21 字节帧头。这个断言能防止将旧协议多出的产品码/版本号继续写入 v2 帧头。
	if len(packet) < 22 {
		t.Fatalf("%s 帧长度过短: %d", vector.name, len(packet))
	}
	if got := hex.EncodeToString(packet[:2]); got != "7273" {
		t.Fatalf("%s 魔数错误: %s", vector.name, got)
	}
	if got := hex.EncodeToString(packet[2:14]); got != "fe0102030405060708090afe" {
		t.Fatalf("%s UID 偏移错误: %s", vector.name, got)
	}
	if got := hex.EncodeToString(packet[14:15]); got != lowerHex(rsBoardDirection(vector)) {
		t.Fatalf("%s 方向字节错误: %s", vector.name, got)
	}
	if got := hex.EncodeToString(packet[15:16]); got != lowerHex(vector.command) {
		t.Fatalf("%s 指令字节错误: %s", vector.name, got)
	}
	if got := hex.EncodeToString(packet[16:17]); got != "2a" {
		t.Fatalf("%s 指令计数器错误: %s", vector.name, got)
	}

	payload := packet[21:]
	if got := int(binary.BigEndian.Uint16(packet[17:19])); got != len(payload) {
		t.Fatalf("%s 数据长度错误: 帧头=%d，实际=%d", vector.name, got, len(payload))
	}
	// CRC 只覆盖数据区；若误覆盖帧头，设备会静默丢弃命令，因此这里独立计算并比对。
	expectedCRC, err := core.Crc(payload, "crc16_modbus")
	if err != nil {
		t.Fatalf("%s 计算 CRC 失败: %v", vector.name, err)
	}
	if got := uint64(binary.BigEndian.Uint16(packet[19:21])); got != expectedCRC {
		t.Fatalf("%s CRC 错误: 帧内=%04X，期望=%04X", vector.name, got, expectedCRC)
	}
}

func assertRSBoardRoundTrip(t *testing.T, codec *Codec, packet []byte, vector rsBoardVector) {
	t.Helper()
	decoded, err := decodeRSBoardPacket(codec, packet)
	if err != nil {
		t.Fatalf("%s 解码失败: %v", vector.name, err)
	}
	fields := decoded
	if got := fields["command"]; got != vector.command {
		t.Fatalf("%s 指令解码错误: %#v", vector.name, got)
	}
	if got := fields["direction"]; got != rsBoardDirection(vector) {
		t.Fatalf("%s 方向解码错误: %#v", vector.name, got)
	}

	// 编码端省略 code 时会写入默认成功码，解码结果必须补回该值；其余字段必须逐字节往返一致。
	wantData := make(map[string]any, len(vector.data)+1)
	for key, value := range vector.data {
		wantData[key] = value
	}
	if _, ok := wantData["code"]; !ok {
		wantData["code"] = uint64(0)
	}
	if got := fields["data"]; !reflect.DeepEqual(got, wantData) {
		t.Fatalf("%s 数据区往返不一致:\n实际: %#v\n期望: %#v", vector.name, got, wantData)
	}
}

func decodeRSBoardPacket(codec *Codec, packet []byte) (map[string]any, error) {
	// Codec.Scan 的回调在独立协程执行。使用带缓冲通道接收单帧结果，
	// 防止测试在 Scan 返回时错误地认为解码已经完成，从而掩盖异步解码故障。
	resultCh := make(chan *ScanResult, 1)
	err := codec.Stream(bytes.NewReader(packet)).Scan(func(result *ScanResult) error {
		if !result.Abaddon {
			resultCh <- result
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	select {
	case result := <-resultCh:
		if result.ScanError != nil {
			return nil, result.ScanError
		}
		fields, ok := result.Data.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("解码结果类型错误: %T", result.Data)
		}
		return fields, nil
	case <-time.After(time.Second):
		return nil, fmt.Errorf("等待协议解码结果超时")
	}
}

func lowerHex(value string) string {
	decoded, _ := hex.DecodeString(value)
	return hex.EncodeToString(decoded)
}

func TestRSBoardV2RequestAndWriteCommands(t *testing.T) {
	codec := newRSBoardV2Codec(t)

	// 本表覆盖文档中由上位机发起的每一种 BB 查询和 AA 写入。
	// 不仅检查 YAML 能编译，还检查每一个字段都实际进入数据区并能够按相同方向解码回来。
	vectors := []rsBoardVector{
		{"读取设备型号", "01", false, false, rsData()},
		{"读取最大通信帧长度", "02", false, false, rsData()},
		{"读取控制接口组数", "03", false, false, rsData()},
		{"读取接口电平", "04", false, false, rsData("groupNo", uint64(2))},
		{"读取开门按钮设置", "05", false, false, rsData()},
		{"设置开门按钮", "05", true, false, rsData("enabled", uint64(1), "openLevel", uint64(0))},
		{"读取门磁设置", "06", false, false, rsData()},
		{"设置门磁", "06", true, false, rsData("enabled", uint64(1), "openLevel", uint64(1))},
		{"读取继电器闭合时长", "07", false, false, rsData()},
		{"设置继电器闭合时长", "07", true, false, rsData("relayCloseDurationMs", uint64(500))},
		{"设置继电器动作", "08", true, false, rsData("groupNo", uint64(2), "relayEnabled", uint64(1))},
		{"读取 FLASH 大小", "09", false, false, rsData()},
		{"读取 FLASH 数据", "0A", false, false, rsData("flashAddress", uint64(0x1000), "length", uint64(3))},
		{"写入 FLASH 数据", "0A", true, false, rsData("flashAddress", uint64(0x2000), "length", uint64(3), "flashData", "A1B2C3")},
		{"读取 GBK16 字库信息", "0B", false, false, rsData()},
		{"读取 ASCII16 字库信息", "0C", false, false, rsData()},
		{"读取 OTA 固件信息", "0D", false, false, rsData()},
		{"读取音频下载信息", "0E", false, false, rsData()},
		{"读取音频参数", "0F", false, false, rsData("audioNo", uint64(6))},
		{"设置音频参数", "0F", true, false, rsData("audioNo", uint64(6), "sampleRate", uint64(8), "audioLength", uint64(4096))},
		{"读取提示音音量", "10", false, false, rsData()},
		{"设置提示音音量", "10", true, false, rsData("volume", uint64(5))},
		{"播放提示音", "11", true, false, rsData("promptNo", uint64(6))},
		{"读取 RTC", "12", false, false, rsData()},
		{"设置 RTC", "12", true, false, rsData("timestamp", uint64(1710000000))},
		{"发起 OTA", "13", true, false, rsData("firmwareLength", uint64(8192), "firmwareChecksum", uint64(0x12345678))},
		{"读取 HUB 参数", "14", false, false, rsData()},
		{"设置 HUB 参数", "14", true, false, rsData("hubCount", uint64(2), "displayDirection", uint64(1))},
		{"立即显示内容", "15", true, false, rsData("durationSeconds", uint64(5), "line1Length", uint64(2), "line1Speed", uint64(20), "line2Length", uint64(0), "line2Speed", uint64(0), "line3Length", uint64(0), "line3Speed", uint64(0), "line4Length", uint64(0), "line4Speed", uint64(0), "content", "C4E3BAC3")},
		{"读取显示内容", "16", false, false, rsData("displayNo", uint64(3))},
		{"设置显示内容", "16", true, false, rsData("displayNo", uint64(3), "line1Length", uint64(2), "line1Speed", uint64(30), "line2Length", uint64(0), "line2Speed", uint64(0), "line3Length", uint64(0), "line3Speed", uint64(0), "line4Length", uint64(0), "line4Speed", uint64(0), "content", "B2E2CAD4")},
		{"读取刷卡去重间隔", "17", false, false, rsData()},
		{"设置刷卡去重间隔", "17", true, false, rsData("cardReportIntervalSeconds", uint64(3))},
		{"读取离在线模式", "19", false, false, rsData()},
		{"设置离在线模式", "19", true, false, rsData("onlineMode", uint64(1))},
		{"读取心跳周期", "1A", false, false, rsData()},
		{"设置心跳周期", "1A", true, false, rsData("heartbeatIntervalMs", uint64(30000))},
		{"读取上报重试超时", "1B", false, false, rsData()},
		{"设置上报重试超时", "1B", true, false, rsData("reportRetryTimeoutMs", uint64(800))},
		{"读取韦根去重间隔", "1C", false, false, rsData()},
		{"设置韦根去重间隔", "1C", true, false, rsData("sameWgIdFlashReadIntervalSeconds", uint64(4))},
		{"按卡号读取用户", "1D", false, false, rsData("hid", uint64(12), "pid", uint64(34567))},
		{"设置用户", "1D", true, false, rsData("user", rsUserInfo())},
		{"删除用户", "1E", true, false, rsData("hid", uint64(12), "pid", uint64(34567))},
		{"删除全部用户", "1F", true, false, rsData()},
		{"读取用户数量", "20", false, false, rsData()},
		{"按编号读取用户", "21", false, false, rsData("userIndex", uint64(7))},
		{"读取用户表地址", "22", false, false, rsData()},
		{"读取上报 ID 位数", "23", false, false, rsData()},
		{"设置上报 ID 位数", "23", true, false, rsData("userIdDigits", uint64(8))},
		{"读取 TCP 客户端参数", "24", false, false, rsData()},
		{"设置 TCP 客户端参数", "24", true, false, rsData("dhcpEnabled", uint64(0), "port", uint64(9000), "ip", "C0A8010A", "gateway", "C0A80101", "subnetMask", "FFFFFF00")},
		{"读取 TCP 服务器参数", "25", false, false, rsData()},
		{"设置 TCP 服务器参数", "25", true, false, rsData("port", uint64(9010), "host", "server.example")},
		{"设置打印开关", "F1", true, false, rsData("printEnabled", uint64(1))},
		{"恢复默认值", "F2", true, false, rsData()},
		{"重启控制板", "F3", true, false, rsData()},
	}

	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
			packet := encodeRSBoardVector(t, codec, vector)
			assertRSBoardFrame(t, packet, vector)
			assertRSBoardRoundTrip(t, codec, packet, vector)
		})
	}
}

func TestRSBoardV2DeviceReportsAndReadResponses(t *testing.T) {
	codec := newRSBoardV2Codec(t)

	// CC 是设备返回查询结果或主动上报时的方向。本表逐一覆盖所有指令的返回数据，
	// 特别包含变长 FLASH、变长域名、显示内容和两种卡号长度，避免只测固定长度字段。
	vectors := []rsBoardVector{
		{"设备型号上报", "01", false, true, rsData("deviceType", uint64(4), "version", "01020304")},
		{"最大帧长度返回", "02", false, true, rsData("maxFrameLength", uint64(1024))},
		{"接口组数返回", "03", false, true, rsData("controlGroupCount", uint64(2))},
		{"接口电平上报", "04", false, true, rsData("groupNo", uint64(1), "relayLevel", uint64(1), "openButtonLevel", uint64(0), "doorSensorLevel", uint64(1))},
		{"开门按钮返回", "05", false, true, rsData("enabled", uint64(1), "openLevel", uint64(0))},
		{"门磁返回", "06", false, true, rsData("enabled", uint64(1), "openLevel", uint64(1))},
		{"继电器时长返回", "07", false, true, rsData("relayCloseDurationMs", uint64(500))},
		{"继电器动作应答", "08", false, true, rsData()},
		{"FLASH 大小返回", "09", false, true, rsData("flashSize", uint64(1048576))},
		{"FLASH 数据返回", "0A", false, true, rsData("flashData", "A1B2C3")},
		{"GBK16 字库返回", "0B", false, true, rsData("fontAddress", uint64(0x10000), "fontSize", uint64(0x20000))},
		{"ASCII16 字库返回", "0C", false, true, rsData("fontAddress", uint64(0x30000), "fontSize", uint64(0x40000))},
		{"OTA 地址返回", "0D", false, true, rsData("firmwareAddress", uint64(0x50000))},
		{"音频下载信息返回", "0E", false, true, rsData("audioAddress", uint64(0x60000), "audioDataSize", uint64(32000), "audioCount", uint64(10))},
		{"音频参数返回", "0F", false, true, rsData("sampleRate", uint64(8), "audioLength", uint64(4096))},
		{"音量返回", "10", false, true, rsData("volume", uint64(4))},
		{"播放提示音应答", "11", false, true, rsData()},
		{"RTC 返回", "12", false, true, rsData("timestamp", uint64(1710000000))},
		{"OTA 应答", "13", false, true, rsData()},
		{"HUB 参数返回", "14", false, true, rsData("hubCount", uint64(2), "displayDirection", uint64(1))},
		{"立即显示应答", "15", false, true, rsData()},
		{"显示内容返回", "16", false, true, rsData("line1Length", uint64(2), "line1Speed", uint64(30), "line2Length", uint64(0), "line2Speed", uint64(0), "line3Length", uint64(0), "line3Speed", uint64(0), "line4Length", uint64(0), "line4Speed", uint64(0), "content", "B2E2CAD4")},
		{"刷卡去重间隔返回", "17", false, true, rsData("cardReportIntervalSeconds", uint64(3))},
		{"八位卡号上报", "18", false, true, rsData("readerNo", uint64(1), "hid", uint64(123), "pid", uint64(45678))},
		{"五位卡号上报", "18", false, true, rsData("readerNo", uint64(1), "pid", uint64(45678))},
		{"离在线模式返回", "19", false, true, rsData("onlineMode", uint64(1))},
		{"心跳周期返回", "1A", false, true, rsData("heartbeatIntervalMs", uint64(30000))},
		{"上报重试超时返回", "1B", false, true, rsData("reportRetryTimeoutMs", uint64(800))},
		{"韦根去重间隔返回", "1C", false, true, rsData("sameWgIdFlashReadIntervalSeconds", uint64(4))},
		{"用户信息返回", "1D", false, true, rsData("user", rsUserInfo())},
		{"删除用户应答", "1E", false, true, rsData()},
		{"删除全部用户应答", "1F", false, true, rsData()},
		{"用户数量返回", "20", false, true, rsData("userCount", uint64(7))},
		{"按编号用户信息返回", "21", false, true, rsData("user", rsUserInfo())},
		{"用户地址返回", "22", false, true, rsData("userOffsetTableAddress", uint64(0x70000), "userInfoTableAddress", uint64(0x71000))},
		{"上报 ID 位数返回", "23", false, true, rsData("userIdDigits", uint64(8))},
		{"TCP 客户端参数返回", "24", false, true, rsData("dhcpEnabled", uint64(1), "port", uint64(9000), "ip", "C0A8010A", "gateway", "C0A80101", "subnetMask", "FFFFFF00")},
		{"TCP 服务器参数返回", "25", false, true, rsData("port", uint64(9010), "host", "server.example")},
		{"打印开关应答", "F1", false, true, rsData()},
		{"恢复默认值应答", "F2", false, true, rsData()},
		{"重启应答", "F3", false, true, rsData()},
	}

	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
			packet := encodeRSBoardVector(t, codec, vector)
			assertRSBoardFrame(t, packet, vector)
			assertRSBoardRoundTrip(t, codec, packet, vector)
		})
	}
}

func TestRSBoardV2RejectsInvalidDataCRC(t *testing.T) {
	codec := newRSBoardV2Codec(t)
	vector := rsBoardVector{"CRC 篡改样本", "0A", false, true, rsData("flashData", "A1B2C3")}
	packet := encodeRSBoardVector(t, codec, vector)

	// 故意篡改数据区而不重算 CRC。若这里能通过，线上串口噪声或篡改数据会被当成有效开闸指令。
	packet[len(packet)-1] ^= 0xFF
	if _, err := decodeRSBoardPacket(codec, packet); err == nil {
		t.Fatal("篡改数据区后仍通过 CRC 校验")
	}
}

func TestRSBoardV2StreamFraming(t *testing.T) {
	codec := newRSBoardV2Codec(t)
	first := encodeRSBoardVector(t, codec, rsBoardVector{"第一帧", "04", false, true, rsData("groupNo", uint64(1), "relayLevel", uint64(1), "openButtonLevel", uint64(0), "doorSensorLevel", uint64(1))})
	second := encodeRSBoardVector(t, codec, rsBoardVector{"第二帧", "18", false, true, rsData("readerNo", uint64(1), "hid", uint64(123), "pid", uint64(45678))})

	// 在两帧之间插入垃圾字节，验证 framing_rule 会按 dataLen 找到下一帧，
	// 而不是因为串口粘包/噪声把两条独立事件合并成一条错误记录。
	stream := bytes.NewBuffer(append(append(append([]byte{}, first...), 0x99), second...))
	var wg sync.WaitGroup
	wg.Add(2)
	var commands []string
	var lock sync.Mutex
	err := codec.Stream(stream).Scan(func(result *ScanResult) error {
		if result.Abaddon || result.ScanError != nil {
			return nil
		}
		lock.Lock()
		commands = append(commands, result.Data.(map[string]any)["command"].(string))
		lock.Unlock()
		wg.Done()
		return nil
	})
	if err != nil {
		t.Fatalf("流式分帧失败: %v", err)
	}
	wg.Wait()
	// ScanResult 回调由 Codec 异步调度，回调完成顺序不属于分帧协议的保证范围，
	// 因此按指令码排序后只校验两帧都被正确识别，避免测试因调度时序偶发失败。
	sort.Strings(commands)
	if !reflect.DeepEqual(commands, []string{"04", "18"}) {
		t.Fatalf("流式分帧结果错误: %#v", commands)
	}
}

func rsUserInfo() map[string]any {
	return rsData(
		"hid", uint64(12),
		"pid", uint64(34567),
		"permission", uint64(1),
		"startTimestamp", uint64(1710000000),
		"endTimestamp", uint64(1810000000),
		"reserved", "0000000000000000",
	)
}
