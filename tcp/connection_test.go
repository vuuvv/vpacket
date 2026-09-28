package tcp

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/log"
	"go.uber.org/zap"
)

func init() {
	log.SetLogger(zap.NewNop())
}

func TestDeviceDiscoveryRetriesUntilIdentified(t *testing.T) {
	server := NewTCPServer(&ServerConfig{}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()

	done := make(chan struct{})
	go func() {
		conn.discoverDevice(map[string]any{"hex": "A1"}, 40*time.Millisecond)
		close(done)
	}()

	assertPacket := func() {
		t.Helper()
		if err := deviceConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		packet := make([]byte, 1)
		if _, err := deviceConn.Read(packet); err != nil {
			t.Fatal(err)
		}
		if packet[0] != 0xA1 {
			t.Fatalf("发现命令 = %x, 期望 a1", packet)
		}
	}

	// 静默设备应在建连后立即收到探测帧，超时未返回 SN 时还应再次收到。
	assertPacket()
	assertPacket()
	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{"sn": "gateway-1"}})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("获取 SN 后发现轮询未退出")
	}

	if err := deviceConn.SetReadDeadline(time.Now().Add(120 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	packet := make([]byte, 1)
	if _, err := deviceConn.Read(packet); err == nil {
		t.Fatalf("获取 SN 后仍发送发现命令: %x", packet)
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatalf("期望读超时，实际为: %v", err)
	}
}

func TestDeviceDiscoveryStopsOnDisconnect(t *testing.T) {
	server := NewTCPServer(&ServerConfig{}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)

	done := make(chan struct{})
	go func() {
		conn.discoverDevice(map[string]any{"hex": "A1"}, time.Second)
		close(done)
	}()
	packet := make([]byte, 1)
	if err := deviceConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := deviceConn.Read(packet); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("连接关闭后发现轮询未退出")
	}
}

func TestDeviceDiscoveryIgnoresEmptyCommand(t *testing.T) {
	server := NewTCPServer(&ServerConfig{}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()

	for _, command := range []map[string]any{nil, {}} {
		done := make(chan struct{})
		go func() {
			conn.discoverDevice(command, 20*time.Millisecond)
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("未配置设备发现命令时仍启动了轮询")
		}
	}

	if err := deviceConn.SetReadDeadline(time.Now().Add(80 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	packet := make([]byte, 1)
	if _, err := deviceConn.Read(packet); err == nil {
		t.Fatalf("未配置设备发现命令时仍发送报文: %x", packet)
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatalf("期望读超时，实际为: %v", err)
	}
}

func TestSendConfiguredCommandIgnoresHexSpaces(t *testing.T) {
	server := NewTCPServer(&ServerConfig{}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()

	done := make(chan error, 1)
	go func() {
		done <- conn.sendConfiguredCommand(map[string]any{"hex": "  A1  B2  "})
	}()
	if err := deviceConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	packet := make([]byte, 2)
	if _, err := io.ReadFull(deviceConn, packet); err != nil {
		t.Fatal(err)
	}
	if packet[0] != 0xA1 || packet[1] != 0xB2 {
		t.Fatalf("发送报文=%x，期望 a1b2", packet)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServerHeartbeatWaitsForSn(t *testing.T) {
	server := NewTCPServer(&ServerConfig{}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()

	queries := 0
	discover := func(sn, deviceType string) ([]string, error) {
		queries++
		if sn != "gateway-1" || deviceType != "gateway" {
			t.Fatalf("查询参数错误: sn=%q, deviceType=%q", sn, deviceType)
		}
		return []string{"child-1"}, nil
	}
	conn.Heartbeat(0, discover, map[string]any{"hex": "B2"})
	if queries != 0 || !conn.heartbeatTime.IsZero() {
		t.Fatal("获得主设备 SN 前不应查询子设备或发送心跳")
	}

	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{"sn": "gateway-1", "deviceType": "gateway"}})
	done := make(chan struct{})
	go func() {
		conn.Heartbeat(0, discover, map[string]any{"hex": "B2"})
		close(done)
	}()
	if err := deviceConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	packet := make([]byte, 1)
	if _, err := deviceConn.Read(packet); err != nil {
		t.Fatal(err)
	}
	<-done
	if packet[0] != 0xB2 || queries != 1 {
		t.Fatalf("主动心跳报文=%x, 子设备查询次数=%d", packet, queries)
	}
}

func TestSubDeviceIdentifiesConnectionWhenSnMissing(t *testing.T) {
	server := NewTCPServer(&ServerConfig{}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()

	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{
		"sn": "child-1", "deviceType": "child", "subDevice": true,
	}})
	if sn, deviceType := conn.identity(); sn != "child-1" || deviceType != "child" {
		t.Fatalf("子设备未填充连接身份: sn=%q, deviceType=%q", sn, deviceType)
	}
	select {
	case <-conn.identified:
	default:
		t.Fatal("子设备提供 SN 后发现轮询未收到已识别信号")
	}
	if got := conn.subDeviceSnapshot(); len(got) != 1 || got[0] != "child-1" {
		t.Fatalf("子设备未加入连接列表: %v", got)
	}

	// 后续主设备报文仍应覆盖临时身份，子设备列表继续保留用于心跳。
	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{
		"sn": "gateway-1", "deviceType": "gateway",
	}})
	if sn, deviceType := conn.identity(); sn != "gateway-1" || deviceType != "gateway" {
		t.Fatalf("主设备未覆盖连接身份: sn=%q, deviceType=%q", sn, deviceType)
	}
	if got := conn.subDeviceSnapshot(); len(got) != 1 || got[0] != "child-1" {
		t.Fatalf("覆盖连接身份后子设备列表丢失: %v", got)
	}
}

func TestServerHeartbeatUsesReportedSubDevicesWithDiscoveryCommand(t *testing.T) {
	server := NewTCPServer(&ServerConfig{DeviceDiscoveryCmd: map[string]any{"hex": "A1"}}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()

	queries := 0
	discover := func(sn, deviceType string) ([]string, error) {
		queries++
		return []string{"queried-child"}, nil
	}
	assertPacket := func(command map[string]any, want byte) {
		t.Helper()
		done := make(chan struct{})
		go func() {
			conn.Heartbeat(0, discover, command)
			close(done)
		}()
		if err := deviceConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		packet := make([]byte, 1)
		if _, err := deviceConn.Read(packet); err != nil {
			t.Fatal(err)
		}
		if packet[0] != want {
			t.Fatalf("发送报文=%x，期望=%x", packet, want)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			conn.Close()
			t.Fatal("发送报文后 Heartbeat 未结束")
		}
	}

	// 连接尚未上报主设备 SN 时，已配置的发现命令仍应由 Heartbeat 下发。
	assertPacket(nil, 0xA1)
	if queries != 0 {
		t.Fatal("配置设备发现命令后不应调用设备发现接口")
	}
	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{"sn": "gateway-1", "deviceType": "gateway"}})

	// 主设备已识别但尚无子设备时，继续发送发现命令；即使未配置主动心跳包也要执行。
	assertPacket(nil, 0xA1)
	if queries != 0 || conn.heartbeatTime.IsZero() {
		t.Fatalf("空子设备列表未执行发现: 查询次数=%d, 发现时间=%v", queries, conn.heartbeatTime)
	}

	child := &core.ScanResult{Data: map[string]any{"sn": "child-1", "subDevice": true}}
	conn.setupDeviceSn(child)
	conn.setupDeviceSn(child)
	if got := conn.subDeviceSnapshot(); len(got) != 1 || got[0] != "child-1" {
		t.Fatalf("连接内子设备列表错误: %v", got)
	}

	assertPacket(map[string]any{"hex": "B2"}, 0xB2)
	if queries != 0 {
		t.Fatalf("配置设备发现命令后仍调用接口: %d", queries)
	}

	server.RemoveSn("child-1", false)
	if got := conn.subDeviceSnapshot(); len(got) != 0 {
		t.Fatalf("已移除的子设备仍留在连接中: %v", got)
	}
	if server.GetDeviceConnection("child-1") != nil {
		t.Fatal("已移除的子设备仍保留连接索引")
	}
	lastHeartbeat := conn.heartbeatTime
	assertPacket(nil, 0xA1)
	if queries != 0 || !conn.heartbeatTime.After(lastHeartbeat) {
		t.Fatalf("移除全部子设备后未恢复发现: 查询次数=%d, 发现时间=%v", queries, conn.heartbeatTime)
	}
}

func TestServerHeartbeatSkipsEmptyQueriedSubDevices(t *testing.T) {
	server := NewTCPServer(&ServerConfig{}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()
	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{"sn": "gateway-1", "deviceType": "gateway"}})

	queries := 0
	done := make(chan struct{})
	go func() {
		conn.Heartbeat(0, func(sn, deviceType string) ([]string, error) {
			queries++
			return nil, nil
		}, map[string]any{"hex": "B2"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		conn.Close()
		t.Fatal("查询未返回子设备时，心跳发送未及时结束")
	}
	if queries != 1 || !conn.heartbeatTime.IsZero() {
		t.Fatalf("空子设备查询仍执行心跳: 查询次数=%d, 心跳时间=%v", queries, conn.heartbeatTime)
	}
}
