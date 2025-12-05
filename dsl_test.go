package vpacket

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
)

func TestDsl(t *testing.T) {
	Setup()
	// 1. 加载协议配置
	yamlBytes, err := os.ReadFile("./resources/protocols.yaml")
	if err != nil {
		log.Fatal("Error reading protocols.yaml:", err)
	}

	// 模拟数据
	mockStream := new(bytes.Buffer)
	mockStream.WriteString("[keep_alive]") // Packet 1: Text Heartbeat
	//writePacket(mockStream, 1, 4, []byte{0x00, 0xAA, 0xBB, 0xCC}) // Packet 2: Cmd 0x01 (Len 4)
	//mockStream.WriteString("junk")
	//writePacket(mockStream, 2, 5, []byte{0x00, 0x01, 0x02, 0x03, 0x04}) // Packet 3: Cmd 0x02 (Len 5)
	//writePacket(mockStream, 99, 10, bytes.Repeat([]byte{0xEF}, 10))     // Packet 4: Cmd 0x63 (Default, Len 10)
	//
	mockStream.Write(
		[]byte{0x72, 0x73, 0xbb, 0xbb, 0x8c, 0xab, 0xcd, 0x23, 0x9e, 0xbc, 0x45, 0xe3, 0x39, 0xe3, 0x39, 0x00, 0x00, 0x00, 0x00, 0xc3, 0x00, 0x13, 0x0e, 0x0b, 0x01, 0x05, 0x30, 0x35, 0x32, 0x34, 0x32, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	)

	fmt.Printf(">>> 模拟混合流总长度: %d bytes\n", mockStream.Len())

	// 预处理和编译 DSL
	scanner, err := NewCodecFromBytes(yamlBytes)
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}

	err = scanner.Stream(mockStream).Scan(func(result *ScanResult) error {
		printJson(result.Data)
		if result.HandleError != nil {
			fmt.Println(result.HandleError)
		}
		if result.ScanError != nil {
			fmt.Println(result.ScanError)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}

	text := `
{
  "board_id": "BB8CABCD239EBC45E339E339",
  "command": "C3",
  "data": {
    "card_no": "05242",
    "channel": 1,
    "idLen": 5
  },
  "dataCrc": 3595
}
`
	fields := map[string]any{}
	err = json.Unmarshal([]byte(text), &fields)
	if err != nil {
		log.Fatal(err)
	}

	bs, err := scanner.Encode(fields)

	if err != nil {
		fmt.Printf("%+v", err)
		log.Fatal(err)
	}

	fmt.Printf(">>> 编码结果: %x\n", bs)

	//var binaryNodes []Node
	//var allProtocols []*Protocol
	//var binaryProtocolName string
	//
	//for _, p := range rootConfig.Protocols {
	//	if err := p.Setup(); err != nil {
	//		log.Fatalf("Error processing rules for %s: %v", p.Name, err)
	//	}
	//	allProtocols = append(allProtocols, p)
	//
	//	if p.Type == "binary" {
	//		// 关键：将 DataStructures 传入 Compile
	//		nodes, err := Compile(p.Fields, rootConfig.DataStructures)
	//		if err != nil {
	//			log.Fatalf("Error compiling binary fields: %v", err)
	//		}
	//		binaryNodes = nodes
	//		binaryProtocolName = p.Name
	//	}
	//}
	//fmt.Println(">>> 混合模式 DSL 编译成功")
	//
	//// 3. 构造混合数据流，测试 Command Switch 逻辑
	//// Command 0x01 (外部引用) 和 Command 0x02 (内联定义)
	//
	//// 4. 创建模块化分发器 (Codec)
	//scanner := bufio.NewCodec(mockStream)
	//scanner.Split(Splitter(allProtocols))
	//
	//// 5. 循环读取完整包并分发
	//packetCount := 0
	//for scanner.Scan() {
	//	packetCount++
	//	packet := scanner.Bytes()
	//
	//	if len(packet) == 1 {
	//		// 丢弃的数据
	//		fmt.Printf("\n=== 丢弃数据 %x ===\n", packet[0])
	//		continue
	//	}
	//
	//	if len(packet) > 0 && packet[0] == '[' {
	//		fmt.Printf("\n=== 收到第 %d 个包 (TEXT) ===\n", packetCount)
	//		fmt.Printf(">>> 协议: DTUHeartbeat, 内容: %s\n", string(packet))
	//	} else {
	//		fmt.Printf("\n=== 收到第 %d 个包 (BINARY, Len: %d) ===\n", packetCount, len(packet))
	//
	//		ctx := NewContext(packet)
	//		for _, node := range binaryNodes {
	//			if err := node.Decode(ctx); err != nil {
	//				log.Printf("DSL 解析失败: %v", err)
	//				continue
	//			}
	//		}
	//		fmt.Printf(">>> 协议: %s, 解析结果:\n", binaryProtocolName)
	//		printJson(ctx.Vars)
	//	}
	//}
	//
	//if err := scanner.Err(); err != nil {
	//	log.Printf("Codec Error: %v", err)
	//}
}

// 辅助函数：构造二进制包 (command: 1-byte, dataLen: 2-byte)
func writePacket(buf *bytes.Buffer, command uint8, dataLen uint16, payload []byte) {
	// 1-2: Magic 0x7273
	binary.Write(buf, binary.BigEndian, uint16(0x7273))
	// 3: Direction
	buf.WriteByte(0xAA)
	// 4-15: Board ID (12 bytes)
	buf.Write(bytes.Repeat([]byte{0xEE}, 12))
	// 16-17: Product Code
	binary.Write(buf, binary.BigEndian, uint16(0x1001))
	// 18-19: Version (1.5)
	binary.Write(buf, binary.BigEndian, uint16(0x0105))
	// 20: Command
	buf.WriteByte(command)
	// 21-22: Data Len
	binary.Write(buf, binary.BigEndian, dataLen)
	// 23-24: Checksum (Mock)
	binary.Write(buf, binary.BigEndian, uint16(0xABCD))
	// 25-n: Payload
	buf.Write(payload)
}

func printJson(v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
