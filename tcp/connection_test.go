package tcp

import (
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
	if queries != 0 {
		t.Fatal("获得主设备 SN 前查询了子设备")
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
