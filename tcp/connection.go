package tcp

import (
	"context"
	"encoding/hex"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cast"
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/log"
	"github.com/vuuvv/vpacket/utils"
	"go.uber.org/zap"
)

type DeviceConnection struct {
	server         *Server
	conn           net.Conn
	key            string
	msgQueue       chan []byte
	lastActiveTime time.Time
	heartbeatTime  time.Time
	mu             sync.Mutex
	identityMu     sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	identified     chan struct{} // 首次取得连接 SN 后关闭，用于停止建连阶段的发现轮询
	sn             string        // 连接身份，优先使用主设备 SN；主设备未上报时可暂用子设备 SN
	deviceType     string
	subDevices     []string // 子设备序列号，可来自本连接上报或服务器查询；访问时须持有 identityMu
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
		identified:     make(chan struct{}),
	}
	server.AddConnection(deviceConn)
	if deviceConn.server.config.MessageDelayTime > 0 {
		deviceConn.msgQueue = make(chan []byte, 100)
		go deviceConn.writeLoop()
	}

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

// func (this *)
func (this *DeviceConnection) Write(data []byte) (int, error) {
	this.mu.Lock()
	defer this.mu.Unlock()

	if this.server.config.MessageDelayTime == 0 {
		log.Info("发送报文", this.zapFields(zap.String("data", utils.Bytes2Hex(data)))...)
		return this.conn.Write(data)
	}

	// 加入队列
	select {
	case this.msgQueue <- data:
		// 成功加入队列
	default:
		// 队列满了,丢弃最旧的消息
		select {
		case <-this.msgQueue:
			// 队列已满,丢弃最旧的消息
		default:
		}
		// 再次尝试加入
		select {
		case this.msgQueue <- data:
		default:
			return 0, errors.New("无法加入连接的写入消息到队列")
		}
	}
	return len(data), nil
}

func (this *DeviceConnection) writeLoop() {
	for msg := range this.msgQueue {
		// 确保距离上次发送至少 200ms
		this.mu.Lock()
		elapsed := time.Since(this.lastActiveTime)
		delayTime := time.Duration(this.server.config.MessageDelayTime) * time.Millisecond
		if elapsed < delayTime {
			waitTime := delayTime - elapsed
			this.mu.Unlock()
			time.Sleep(waitTime)
			this.mu.Lock()
		}

		// 发送消息
		log.Info("发送报文", this.zapFields(zap.String("data", utils.Bytes2Hex(msg)))...)
		_, err := this.conn.Write(msg)
		if err != nil {
			log.Error(errors.Wrapf(err, "发送报文失败: %s", err.Error()), this.zapFields(zap.String("data", utils.Bytes2Hex(msg)))...)
			this.mu.Unlock()
			return
		}

		this.lastActiveTime = time.Now()
		this.mu.Unlock()
	}
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
	sn, deviceType, subDevice := this.getConnectionDevice(result)
	if sn == "" {
		return
	}

	if subDevice {
		// 主动探测模式从本连接的子设备上报维护列表，心跳不能再依赖服务器查询。
		this.addSubDevice(sn, deviceType)
		this.server.AddDevice(this, sn)
		return
	}

	// 主连接设备
	this.identityMu.Lock()
	firstIdentified := this.sn == ""
	this.sn = sn
	this.deviceType = deviceType
	if firstIdentified {
		// 发现轮询通过此信号立即退出，避免拿到 SN 后仍等待下一次定时器。
		close(this.identified)
	}
	this.identityMu.Unlock()
	this.server.AddDevice(this, sn)
}

func (this *DeviceConnection) identity() (string, string) {
	this.identityMu.RLock()
	defer this.identityMu.RUnlock()
	return this.sn, this.deviceType
}

func (this *DeviceConnection) addSubDevice(sn, deviceType string) {
	this.identityMu.Lock()
	defer this.identityMu.Unlock()
	if this.sn == "" {
		// 尚未收到主设备身份时，先用子设备 SN 标识连接，避免后续发现或心跳一直等待空 SN。
		this.sn = sn
		this.deviceType = deviceType
		close(this.identified)
	}
	for _, existing := range this.subDevices {
		if existing == sn {
			return
		}
	}
	this.subDevices = append(this.subDevices, sn)
}

func (this *DeviceConnection) subDeviceSnapshot() []string {
	this.identityMu.RLock()
	defer this.identityMu.RUnlock()
	return append([]string(nil), this.subDevices...)
}

func (this *DeviceConnection) setSubDevices(subDevices []string) {
	this.identityMu.Lock()
	this.subDevices = append([]string(nil), subDevices...)
	this.identityMu.Unlock()
}

func (this *DeviceConnection) removeSubDevice(sn string) {
	this.identityMu.Lock()
	defer this.identityMu.Unlock()
	remaining := make([]string, 0, len(this.subDevices))
	for _, existing := range this.subDevices {
		if existing != sn {
			remaining = append(remaining, existing)
		}
	}
	this.subDevices = remaining
}

// discoverDevice 在建连时先发一次探测命令；设备持续静默时按周期重发，取得连接 SN 或断线后停止。
func (this *DeviceConnection) discoverDevice(command map[string]any, interval time.Duration) {
	if len(command) == 0 {
		// 未配置发现命令时保持设备主动上报模式，不发送空报文。
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-this.ctx.Done():
			return
		case <-this.identified:
			return
		default:
		}

		if err := this.sendConfiguredCommand(command); err != nil {
			log.Warn(errors.Wrapf(err, "发送设备发现命令失败: %s", err.Error()), this.zapFields()...)
		}

		select {
		case <-this.ctx.Done():
			return
		case <-this.identified:
			return
		case <-ticker.C:
		}
	}
}

func (this *DeviceConnection) sendConfiguredCommand(command map[string]any) error {
	// 厂商探测帧可能不是协议编码器支持的完整报文，hex 配置需要原样下发。
	if hexRaw, ok := command["hex"]; ok {
		hexText, err := cast.ToStringE(hexRaw)
		if err == nil {
			// 配置中的十六进制报文可能按字节分组，解码前移除全部空格以免误判为无效报文。
			hexText = strings.ReplaceAll(hexText, " ", "")
			data, err := hex.DecodeString(hexText)
			if err == nil {
				_, err = this.Write(data)
				return err
			}
		}
	}
	return this.SendCommand(command)
}

func (this *DeviceConnection) getConnectionDevice(result *core.ScanResult) (string, string, bool) {
	if result == nil {
		return "", "", true
	}

	if result.Data == nil {
		return "", "", true
	}

	dict, ok := result.Data.(map[string]any)
	if !ok {
		return "", "", true
	}

	sn, ok := dict["sn"].(string)
	if !ok {
		return "", "", true
	}

	deviceType, _ := dict["deviceType"].(string)

	subDevice, ok := dict["subDevice"].(bool)
	if !ok {
		subDevice = false
	}

	return sn, deviceType, subDevice
}

// Heartbeat 按同一周期执行子设备探测或主动心跳。
// 配置发现命令时只使用本连接上报的子设备，主设备 SN 尚未取得也可继续探测。
func (this *DeviceConnection) Heartbeat(duration int, discoveryFunc DeviceDiscoveryFunc, command map[string]any) {
	discoveryCmd := this.server.config.DeviceDiscoveryCmd
	sn, deviceType := this.identity()
	// 无可发送命令或尚未到周期时跳过，避免空报文和过密轮询。
	if (len(command) == 0 && len(discoveryCmd) == 0) ||
		time.Since(this.heartbeatTime) < time.Duration(duration)*time.Second {
		return
	}

	subDevices, ok := this.heartbeatSubDevices(sn, deviceType, discoveryFunc, len(discoveryCmd) > 0)
	if !ok {
		return
	}

	switch {
	case len(subDevices) == 0 && len(discoveryCmd) > 0:
		// 当前连接尚无子设备时继续探测，以便后续子设备上报进入列表。
		if err := this.sendConfiguredCommand(discoveryCmd); err != nil {
			log.Warn(errors.Wrapf(err, "发送子设备发现命令失败: %s", err.Error()), this.zapFields()...)
		}
	case len(subDevices) == 0 || len(command) == 0:
		// 无子设备且没有发现命令，或有子设备但没有主动心跳包时，不发送空报文。
		return
	default:
		this.sendSubDeviceHeartbeats(subDevices, command)
	}
	// 发送失败也记录本次尝试，避免下一次连接巡检立即重复发送和报错。
	this.heartbeatTime = time.Now()
}

// heartbeatSubDevices 返回本轮可用的子设备；第二个返回值表示是否成功取得列表。
// 查询成功但列表为空仍要交给 Heartbeat 判断是否发送发现命令。
func (this *DeviceConnection) heartbeatSubDevices(sn, deviceType string, discoveryFunc DeviceDiscoveryFunc, useReported bool) ([]string, bool) {
	if useReported {
		// 已配置连接探测命令时，子设备由当前连接的上报维护，不再请求设备发现接口。
		return this.subDeviceSnapshot(), true
	}
	if sn == "" {
		// 接口查询需要主设备 SN，空 SN 不能作为查询条件；连接探测路径不受此限制。
		return nil, false
	}
	if discoveryFunc == nil {
		log.Warn("未设置子设备发现函数", this.zapFields()...)
		return nil, false
	}
	queried, err := discoveryFunc(sn, deviceType)
	if err != nil {
		log.Warn(errors.Wrapf(err, "查询子设备失败: %s, %s", sn, err.Error()), this.zapFields()...)
		return nil, false
	}
	// 查询结果会变化，先同步设备索引和连接列表，避免继续向已移除的子设备发心跳。
	old := this.subDeviceSnapshot()
	onlyOld, onlyNew, _ := utils.DifferenceBy(old, queried, func(item string) string { return item })
	this.server.RemoveDeviceSn(this, onlyOld...)
	this.server.AddDevice(this, onlyNew...)
	this.setSubDevices(queried)
	return queried, true
}

// sendSubDeviceHeartbeats 按子设备逐个发送；原始 hex 帧保持不变，普通命令再补目标 SN。
func (this *DeviceConnection) sendSubDeviceHeartbeats(subDevices []string, command map[string]any) {
	var hexBs []byte
	useRawHex := false

	if hexRaw, ok := command["hex"]; ok {
		hexText, err := cast.ToStringE(hexRaw)
		if err == nil {
			hexBs, err = hex.DecodeString(hexText)
			if err == nil {
				useRawHex = true
			}
		}
	}

	for _, sn := range subDevices {
		data := hexBs
		if !useRawHex {
			// 每个子设备都复制一份命令，防止补 SN 时改写共享配置并影响后续发送。
			heartbeatCommand := map[string]any{}
			for k, v := range command {
				heartbeatCommand[k] = v
			}
			if _, ok := heartbeatCommand["sn"]; !ok {
				heartbeatCommand["sn"] = sn
			}

			encoded, err := this.Encode(heartbeatCommand)
			if err != nil {
				log.Warn(errors.Wrapf(err, "编码子设备心跳命令失败: %s, %s", sn, err.Error()), this.zapFields()...)
				continue
			}
			data = encoded
		}
		_, err := this.Write(data)
		if err != nil {
			log.Warn(errors.Wrapf(err, "发送子设备心跳命令失败: %s, %s", sn, err.Error()), this.zapFields()...)
			continue
		}
	}
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
