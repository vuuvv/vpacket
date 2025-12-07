package vpacket

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/vuuvv/vpacket/core"
	"log"
	"os"
	"strings"
	"testing"
)

func setupTestScanner(file ...string) *core.Codec {
	filePath := "./resources/protocols.yaml"
	if len(file) > 0 {
		filePath = file[0]
	}
	Setup()
	// 1. 加载协议配置
	yamlBytes, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("%+v\n", err)
		panic(err)
	}
	scanner, err := NewCodecFromBytes(yamlBytes)
	if err != nil {
		fmt.Printf("%+v\n", err)
		panic(err)
	}
	return scanner
}

func TestDsl(t *testing.T) {
	scanner := setupTestScanner()

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

	err := scanner.Stream(mockStream).Scan(func(result *ScanResult) error {
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

	//	text := `
	//{
	//  "board_id": "BB8CABCD239EBC45E339E339",
	//  "command": "C3",
	//  "data": {
	//    "card_no": "05242",
	//    "channel": 1,
	//    "idLen": 5
	//  },
	//  "dataCrc": 3595
	//}
	//`
	text := `
{"command":"04","sn":"BB8CABCD239EBC45E339E339","timestamp":"1764998575"}
`
	bs, err := scanner.EncodeFromJson(text)

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

func TestEncodeFE(t *testing.T) {
	scanner := setupTestScanner()
	text := `
{
  "sn": "BB8CABCD239EBC45E339E339",
  "command": "FE"
}
`
	bs, err := scanner.EncodeFromJson(text)

	if err != nil {
		fmt.Printf("%+v", err)
		log.Fatal(err)
	}

	fmt.Printf(">>> 编码结果: %x\n", bs)
}

func TestDecodeFE(t *testing.T) {
	scanner := setupTestScanner()
	bs, err := hex.DecodeString("7273bbbb8cabcd239ebc45e339e33900000000fe000140BF00")
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	mockStream := new(bytes.Buffer)
	mockStream.Write(bs)

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
}

func TestDecodeSetTime(t *testing.T) {
	scanner := setupTestScanner()
	bs, err := hex.DecodeString(strings.ReplaceAll("72 73 bb bb 8c ab cd 23 9e bc 45 e3 39 e3 39 00 00 00 00 04 00 01 40 bf 00", " ", ""))
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	mockStream := new(bytes.Buffer)
	mockStream.Write(bs)

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
