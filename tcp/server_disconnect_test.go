package tcp

import (
	"net"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/vuuvv/vpacket/core"
)

func TestExpiredMainDeviceClosesConnection(t *testing.T) {
	server := NewTCPServer(&ServerConfig{HeartbeatTimeout: 60}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()
	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{"sn": "board-1"}})
	server.devices.Store("board-1", lo.Tuple2[string, time.Time]{A: conn.key, B: time.Now().Add(-time.Minute)})

	server.expireDevices(time.Now())
	if server.GetDeviceConnection("board-1") != nil {
		t.Fatal("过期主设备仍保留在线索引")
	}
	if _, exists := server.connections.Load(conn.key); exists {
		t.Fatal("过期主设备的连接未清理")
	}
	select {
	case <-conn.ctx.Done():
	default:
		t.Fatal("过期主设备的连接未关闭")
	}
}

func TestExpiredSubDeviceKeepsMainConnection(t *testing.T) {
	server := NewTCPServer(&ServerConfig{HeartbeatTimeout: 60}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()
	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{"sn": "board-1"}})
	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{"sn": "child-1", "subDevice": true}})
	server.devices.Store("child-1", lo.Tuple2[string, time.Time]{A: conn.key, B: time.Now().Add(-time.Minute)})

	server.expireDevices(time.Now())
	if server.GetDeviceConnection("child-1") != nil {
		t.Fatal("过期子设备仍保留在线索引")
	}
	if server.GetDeviceConnection("board-1") != conn {
		t.Fatal("子设备过期错误地关闭了主连接")
	}
}

func TestRefreshedDeviceSurvivesStaleExpirySnapshot(t *testing.T) {
	server := NewTCPServer(&ServerConfig{HeartbeatTimeout: 60}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()
	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{"sn": "board-1"}})
	stale := lo.Tuple2[string, time.Time]{A: conn.key, B: time.Now().Add(-time.Minute)}
	server.devices.Store("board-1", stale)
	server.AddDevice(conn, "board-1")

	// 清理器扫描后设备可能已收到新报文，旧快照不能删除已刷新的设备索引。
	server.expireDevice("board-1", stale)
	if server.GetDeviceConnection("board-1") != conn {
		t.Fatal("过期快照误删已刷新的设备")
	}
}

func TestLateDecodeCallbackCannotRestoreDisconnectedDevice(t *testing.T) {
	server := NewTCPServer(&ServerConfig{}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()
	server.RemoveConnection(conn, false)

	// Scan 回调异步执行，断线清理后才完成的回调不能把已移除的设备重新标为在线。
	conn.setupDeviceSn(&core.ScanResult{Data: map[string]any{"sn": "board-1"}})
	if _, exists := server.devices.Load("board-1"); exists {
		t.Fatal("断线后的解码回调重新建立了设备在线索引")
	}
}

func TestUnidentifiedIdleConnectionExpires(t *testing.T) {
	server := NewTCPServer(&ServerConfig{HeartbeatTimeout: 60}, nil)
	serverConn, deviceConn := net.Pipe()
	defer deviceConn.Close()
	conn := NewDeviceConnection(server, serverConn)
	defer conn.Close()
	conn.lastActiveTime = time.Now().Add(-time.Minute)

	// 未上报 SN 的半开连接没有设备索引，仍须按空闲时间释放连接名额。
	server.expireUnidentifiedConnections(time.Now())
	if _, exists := server.connections.Load(conn.key); exists {
		t.Fatal("未识别的空闲连接仍占用连接名额")
	}
	select {
	case <-conn.ctx.Done():
	default:
		t.Fatal("未识别的空闲连接未关闭")
	}
}
