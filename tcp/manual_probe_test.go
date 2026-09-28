package tcp

import (
	"io"
	"net"
	"testing"
	"time"
)

// TestManualTCPProbe3001 用于观察设备收到固定探测帧后的原始响应。
// 只有显式设置环境变量才监听固定端口，避免普通测试运行时等待外部设备。
// 手动运行：VPACKET_TCP_PROBE_3001=1 go test ./tcp -run '^TestManualTCPProbe3001$' -v -count=1 -timeout 3m
func TestManualTCPProbe3001(t *testing.T) {
	// if os.Getenv("VPACKET_TCP_PROBE_3001") != "1" {
	// 	t.Skip("设置 VPACKET_TCP_PROBE_3001=1 后运行手动 TCP 探针")
	// }

	listener, err := net.Listen("tcp", ":3001")
	if err != nil {
		t.Fatalf("监听本地 3001 端口失败: %v", err)
	}
	defer listener.Close()
	t.Logf("等待设备连接 %s", listener.Addr())

	conn, err := listener.Accept()
	if err != nil {
		t.Fatalf("等待设备连接失败: %v", err)
	}
	defer conn.Close()
	t.Logf("设备已连接: %s", conn.RemoteAddr())

	packet := []byte{0x72, 0x73, 0xfe, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0xfe, 0xbb}
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("设置发送超时失败: %v", err)
	}
	n, err := conn.Write(packet)
	if err != nil {
		t.Fatalf("发送探测报文失败: %v", err)
	}
	if n != len(packet) {
		t.Fatalf("探测报文未完整发送: %v，已发送 %d/%d 字节", io.ErrShortWrite, n, len(packet))
	}
	t.Logf("已发送 %d 字节: % X", n, packet)

	// 设备响应可能无法通过现有协议解码，因此直接读取并打印原始字节。
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Minute)); err != nil {
		t.Fatalf("设置接收超时失败: %v", err)
	}
	response := make([]byte, 4096)
	n, err = conn.Read(response)
	if n > 0 {
		t.Logf("收到 %d 字节: % X", n, response[:n])
		return
	}
	if err != nil {
		t.Fatalf("等待设备响应失败: %v", err)
	}
	t.Fatal("设备未返回报文")
}
