package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/vuuvv/vpacket"
	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/tcp"
)

const (
	// 仅验证用户提供的真实控制板，避免局域网内其他设备上报被误判为测试成功。
	targetSN = "EEBDABCD5783BCF9E339E339"
	listenAt = "0.0.0.0:3001"
	// 厂商文档指定的 UID 探测帧不是标准完整协议帧，必须按原始字节透传，不能套用 v2 的长度和 CRC 规则。
	uidProbeHex = "7273FE0102030405060708090AFEBB"
)

func main() {
	vpacket.Setup()
	scheme, err := vpacket.NewSchemeFromFile("resources/protocols-v2.yaml")
	if err != nil {
		log.Fatalf("加载 RS 协议配置失败: %v", err)
	}
	if err = scheme.Setup(); err != nil {
		log.Fatalf("初始化 RS 协议失败: %v", err)
	}

	resultCh := make(chan map[string]any, 1)
	var modelRequestOnce sync.Once
	server := vpacket.NewTcpServer(&vpacket.TcpServerConfig{
		Address:          listenAt,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		MaxConnections:   10,
		HeartbeatTimeout: 120,
		ConnectionHandler: func(conn *tcp.DeviceConnection) {
			modelRequestOnce.Do(func() {
				probe, decodeErr := hex.DecodeString(uidProbeHex)
				if decodeErr != nil {
					log.Printf("解析厂商 UID 探测报文失败: %v", decodeErr)
					return
				}
				// 板卡可能在 TCP 建连后保持静默，因此先发厂商定义的 UID 探测帧，
				// 再由设备型号回包携带真实 SN，交给正式协议解码链路完成身份校验。
				if _, writeErr := conn.Write(probe); writeErr != nil {
					log.Printf("连接建立后发送 UID 探测报文失败: %v", writeErr)
					return
				}
				log.Printf("连接已建立，已发送厂商 UID 探测报文: %s", uidProbeHex)
			})
		},
	}, scheme)
	defer func() {
		if stopErr := server.Stop(); stopErr != nil {
			log.Printf("停止端到端测试服务器失败: %v", stopErr)
		}
	}()

	server.MessageHandle(func(result *core.ScanResult) error {
		data, ok := result.Data.(map[string]any)
		if !ok {
			return nil
		}
		sn, _ := data["sn"].(string)
		if sn != targetSN {
			// 必须忽略非目标板卡，避免连接到同网段 DTU 或其他门禁板时向错误设备发送命令。
			log.Printf("忽略非目标设备报文：sn=%s，command=%v", sn, data["command"])
			return nil
		}

		log.Printf("收到目标板卡报文：command=%v，direction=%v，data=%v", data["command"], data["direction"], data["data"])
		if data["command"] == "01" && data["direction"] == "CC" {
			select {
			case resultCh <- data:
			default:
			}
		}
		return nil
	})
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.Start()
	}()
	log.Printf("RS 控制板端到端测试已监听 %s，等待 %s 连接（最长 5 分钟）", listenAt, targetSN)

	select {
	case data := <-resultCh:
		payload, _ := data["data"].(map[string]any)
		if code, ok := payload["code"].(uint64); ok && code != 0 {
			log.Fatalf("设备型号查询返回失败码: %d", code)
		}
		log.Printf("端到端测试成功：sn=%s，型号=%v，版本=%v", targetSN, payload["deviceType"], payload["version"])
	case err := <-serverErrCh:
		if err != nil {
			log.Fatalf("TCP 服务器启动或运行失败: %v", err)
		}
		log.Fatal("TCP 服务器意外停止")
	case <-time.After(5 * time.Minute):
		log.Fatal("等待设备连接或设备型号返回超时，请检查板子 TCP 客户端地址、端口 3001 与网络连通性")
	}

	// 让 go run 在测试成功时返回 0，便于现场人员直接据进程退出码判断结果。
	_, _ = fmt.Fprintln(os.Stdout, "RS_BOARD_E2E_OK")
}
