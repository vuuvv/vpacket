package tcp

import (
	"context"
	"fmt"
	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/utils"
	"go.uber.org/zap"
	"net"
	"sync"
	"time"
	"vuuvv.cn/unisoftcn/orca/log"
)

type DeviceConnection struct {
	server         *Server
	conn           net.Conn
	key            string
	lastActiveTime time.Time
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	deviceId       string   // 实际的连接设备，可能是dtu,网关等
	subDevices     []string // 子设备的key(一般是序列号),子设备可以查询服务器获取,或者子设备自己发送心跳(哪种形式应该由服务器进行配置)
}

func NewDeviceConnection(server *Server, conn net.Conn) *DeviceConnection {
	ctx, cancel := context.WithCancel(context.Background())
	deviceConn := &DeviceConnection{
		key:            utils.GenId(),
		server:         server,
		conn:           conn,
		lastActiveTime: time.Now(),
		ctx:            ctx,
		cancel:         cancel,
	}
	server.AddConnection(deviceConn)
	return deviceConn
}

func (this *DeviceConnection) Key() string {
	return this.key
}

func (this *DeviceConnection) SetKey(key string) {
	this.key = key
}

func (this *DeviceConnection) DeviceKey(deviceId string) string {
	return fmt.Sprintf("%s@%s", deviceId, this.RemoteAddr())
}

func (this *DeviceConnection) RemoteAddr() string {
	return this.conn.RemoteAddr().String()
}

func (this *DeviceConnection) Write(data []byte) (int, error) {
	this.mu.Lock()
	defer this.mu.Unlock()
	return this.conn.Write(data)
}

func (this *DeviceConnection) UpdateActiveTime() {
	this.mu.Lock()
	this.lastActiveTime = time.Now()
	this.mu.Unlock()
}

func (this *DeviceConnection) GetLastActiveTime() time.Time {
	this.mu.Lock()
	defer this.mu.Unlock()
	return this.lastActiveTime
}

func (this *DeviceConnection) Scan(protocol *core.Scheme) error {
	/// 启动一个 Goroutine 来监听 Context 取消事件
	go this.checkCancel()

	return core.NewCodec().Config(protocol).Stream(this.conn).Scan(this.Handle)
}

func (this *DeviceConnection) Handle(result *core.ScanResult) error {
	fmt.Printf("%02x\n", result.Packet)
	this.UpdateActiveTime()
	return nil
}

func (this *DeviceConnection) checkCancel() {
	<-this.ctx.Done()
	log.Warn("Context cancelled. Setting ReadDeadline to NOW to interrupt scanner.", this.zapFields()...)

	// Context 被取消了，强制中断阻塞的读取
	// 将读限期设置为现在，导致任何阻塞的 Read 调用立即返回超时错误。
	err := this.conn.SetReadDeadline(time.Now())
	if err != nil {
		log.Error(err, this.zapFields()...)
	}
}

func (this *DeviceConnection) zapFields(fields ...zap.Field) []zap.Field {
	return append([]zap.Field{
		zap.String("addr", this.RemoteAddr()),
		zap.String("key", this.key),
	}, fields...)
}

func (this *DeviceConnection) Close() {
	this.cancel()
	utils.SafeCloseConn(this.conn)
}
