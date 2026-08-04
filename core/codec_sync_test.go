package core

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// syncTestFramingRule 用固定两字节帧模拟协议分帧，便于在不依赖外部设备
// 配置的情况下比较 Scan 和 ScanSync 的协议结果。
type syncTestFramingRule struct{}

func (syncTestFramingRule) Split(data []byte) *FramingRuleMatchResult {
	if len(data) < 2 {
		return WaitFramingRuleMatchResult()
	}
	if data[0] != 0xAA {
		return AbandonFramingRuleMatchResult(1, data)
	}
	return NewFramingRuleMatchResult(2, data[:2])
}

func (syncTestFramingRule) GetHeaderMarker() []byte { return []byte{0xAA} }
func (syncTestFramingRule) Setup() error            { return nil }

func newSyncTestCodec(packet []byte) *Codec {
	protocol := &Protocol{ParsedFramingRule: syncTestFramingRule{}}
	scheme := &Scheme{Protocols: []*Protocol{protocol}}
	return NewCodec().Config(scheme).Stream(bytes.NewReader(packet))
}

func TestScanSyncUsesSameFramingAndDecodeAsScan(t *testing.T) {
	packet := []byte{0xAA, 0x01, 0xAA, 0x02}

	var syncPackets [][]byte
	err := newSyncTestCodec(packet).ScanSync(func(result *ScanResult) error {
		syncPackets = append(syncPackets, append([]byte(nil), result.Packet...))
		return nil
	})
	if err != nil {
		t.Fatalf("同步扫描失败: %v", err)
	}
	if len(syncPackets) != 2 || !bytes.Equal(syncPackets[0], []byte{0xAA, 0x01}) || !bytes.Equal(syncPackets[1], []byte{0xAA, 0x02}) {
		t.Fatalf("同步扫描分帧结果错误: %#v", syncPackets)
	}

	asyncPackets := make(chan []byte, 2)
	err = newSyncTestCodec(packet).Scan(func(result *ScanResult) error {
		asyncPackets <- append([]byte(nil), result.Packet...)
		return nil
	})
	if err != nil {
		t.Fatalf("异步扫描失败: %v", err)
	}
	gotPackets := make([][]byte, 0, len(syncPackets))
	for i := 0; i < len(syncPackets); i++ {
		select {
		case got := <-asyncPackets:
			gotPackets = append(gotPackets, got)
		case <-time.After(time.Second):
			t.Fatalf("Scan 回调在等待时间内未完成，第 %d 帧缺失", i)
		}
	}
	// Scan 的异步回调不保证完成顺序，只比较完整帧集合。
	packetKey := func(packet []byte) string { return string(packet) }
	sort.Slice(syncPackets, func(i, j int) bool { return packetKey(syncPackets[i]) < packetKey(syncPackets[j]) })
	sort.Slice(gotPackets, func(i, j int) bool { return packetKey(gotPackets[i]) < packetKey(gotPackets[j]) })
	for i := range syncPackets {
		if !bytes.Equal(gotPackets[i], syncPackets[i]) {
			t.Fatalf("Scan 与 ScanSync 第 %d 帧不一致: got=%x want=%x", i, gotPackets[i], syncPackets[i])
		}
	}
}

func TestScanSyncReturnsCallbackError(t *testing.T) {
	want := fmt.Errorf("同步处理失败")
	err := newSyncTestCodec([]byte{0xAA, 0x01}).ScanSync(func(_ *ScanResult) error {
		return want
	})
	if err == nil || !strings.Contains(err.Error(), want.Error()) {
		t.Fatalf("同步回调错误未返回: %v", err)
	}
}
