package tcp

import (
	"context"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/log"
	"github.com/vuuvv/vpacket/utils"
	"go.uber.org/zap"
	"net"
	"sync"
	"time"
)

type DeviceConnection struct {
	server         *Server
	conn           net.Conn
	key            string
	lastActiveTime time.Time
	heartbeatTime  time.Time
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	sn             string // 实际的连接设备，可能是设备,dtu,网关等
	deviceType     string
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

//func (this *DeviceConnection) DeviceKey(sn string) string {
//	return fmt.Sprintf("%s@%s", sn, this.key)
//}

func (this *DeviceConnection) RemoteAddr() string {
	return this.conn.RemoteAddr().String()
}

func (this *DeviceConnection) Write(data []byte) (int, error) {
	this.mu.Lock()
	defer this.mu.Unlock()
	log.Info("发送报文", this.zapFields(zap.String("data", utils.Bytes2Hex(data)))...)
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

func (this *DeviceConnection) Encode(data map[string]any) ([]byte, error) {
	return core.NewCodec().Config(this.server.scheme).Encode(data)
}

func (this *DeviceConnection) SendCommand(cmd map[string]any) error {
	bs, err := this.Encode(cmd)
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = this.Write(bs)
	return err
}

func (this *DeviceConnection) Handle(result *core.ScanResult) error {
	log.Info("接收报文", this.zapFields(zap.String("data", utils.Bytes2Hex(result.Packet)))...)

	/// 检查是否是连接设备
	this.setupDeviceSn(result)
	this.UpdateActiveTime()
	if this.server.messageHandle != nil {
		err := this.server.messageHandle(result)
		if err != nil {
			log.Error(err, this.zapFields()...)
		}
	}
	return nil
}

func (this *DeviceConnection) setupDeviceSn(result *core.ScanResult) {
	sn, deviceType := this.getConnectionDevice(result)
	if sn == "" || deviceType == "" {
		return
	}
	this.sn = sn
	this.deviceType = deviceType
	this.server.AddDevice(this, sn)
}

func (this *DeviceConnection) getConnectionDevice(result *core.ScanResult) (string, string) {
	if result == nil {
		return "", ""
	}

	if result.Data == nil {
		return "", ""
	}

	dict, ok := result.Data.(map[string]any)
	if !ok {
		return "", ""
	}

	isConnectionDevice, ok := dict["connectionDevice"].(bool)
	if !ok {
		return "", ""
	}

	if !isConnectionDevice {
		return "", ""
	}

	sn, ok := dict["sn"].(string)
	if !ok {
		return "", ""
	}

	deviceType, ok := dict["deviceType"].(string)
	if !ok {
		return "", ""
	}

	return sn, deviceType
}

func (this *DeviceConnection) Heartbeat(duration int, discoveryFunc DeviceDiscoveryFunc, command map[string]any) {
	if this.sn == "" {
		return
	}

	if discoveryFunc == nil {
		log.Warn("未设置子设备发现函数", this.zapFields()...)
		return
	}

	if command == nil {
		log.Warn("未设置子设备发现命令", this.zapFields()...)
		return
	}

	if time.Since(this.heartbeatTime) < time.Duration(duration)*time.Second {
		return
	}

	subDevices, err := discoveryFunc(this.sn, this.deviceType)
	if err != nil {
		log.Warn(errors.Wrapf(err, "查询子设备失败: %s, %s", this.sn, err.Error()), this.zapFields()...)
		return
	}
	onlyOld, onlyNew, _ := utils.DifferenceBy(this.subDevices, subDevices, func(item string) string { return item })

	this.server.RemoveDeviceSn(this, onlyOld...)
	this.server.AddDevice(this, onlyNew...)
	this.subDevices = subDevices

	for _, sn := range subDevices {
		heartbeatCommand := map[string]any{}
		for k, v := range command {
			heartbeatCommand[k] = v
		}
		if _, ok := heartbeatCommand["sn"]; !ok {
			heartbeatCommand["sn"] = sn
		}

		data, err := this.Encode(heartbeatCommand)
		if err != nil {
			log.Warn(errors.Wrapf(err, "编码子设备心跳命令失败: %s, %s", sn, err.Error()), this.zapFields()...)
			continue
		}
		_, err = this.Write(data)
		if err != nil {
			log.Warn(errors.Wrapf(err, "发送子设备心跳命令失败: %s, %s", sn, err.Error()), this.zapFields()...)
			continue
		}
	}

	this.heartbeatTime = time.Now()
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
