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
	"sync/atomic"
	"time"
)

const (
	DeviceDiscoveryModeHeartbeat = "heartbeat" // 子设备发现模式,通过子设备发送心跳来发现
	DeviceDiscoveryModeSync      = "sync"      // 子设备发现模式,通过服务器查询子设备,然后向子设备发送心跳
)

type DeviceDiscoveryFunc func(sn string, deviceType string) (subDevices []string, err error)

type ServerConfig struct {
	Address             string              `json:"address"`
	ReadBufferSize      int                 `json:"readBufferSize"`
	WriteBufferSize     int                 `json:"writeBufferSize"`
	MaxConnections      int                 `json:"maxConnections"`
	DeviceDiscoveryMode string              `json:"deviceDiscoveryMode"` // 子设备发现模式,默认是通过子设备发送心跳来发现
	DeviceSyncDuration  int                 `json:"deviceSyncDuration"`  // 查询子设备的时间间隔,并向子设备发送心跳,默认是30秒
	DeviceDiscoveryFunc DeviceDiscoveryFunc `json:"-"`
	DeviceDiscoveryCmd  map[string]any      `json:"deviceDiscoveryCmd"`
}

type Server struct {
	config           *ServerConfig
	listener         net.Listener
	connections      sync.Map // 实际的连接数量
	devices          sync.Map // 用于使用设备ID或子设备ID查询设备连接,发送指令
	wg               sync.WaitGroup
	ctx              context.Context
	cancel           context.CancelFunc
	scheme           *core.Scheme
	connectionCounts int32
	messageHandle    func(result *core.ScanResult) error
}

func NewTCPServer(config *ServerConfig, scheme *core.Scheme) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		config: config,
		scheme: scheme,
		ctx:    ctx,
		cancel: cancel,
		//readBufferSize:  4096,
		//writeBufferSize: 4096,
		//maxConnections:  10000,
	}
}

// RegisterProtocol 注册协议
//func (s *TCPServer) RegisterProtocol(meta *ProtocolMeta) error {
//	if _, exists := s.protocols[meta.Name]; exists {
//		return fmt.Errorf("scheme %s already registered", meta.Name)
//	}
//	s.protocols[meta.Name] = meta
//	log.Printf("Registered scheme: %s (type: %s, delimiter: %s)",
//		meta.Name, meta.Type, meta.Delimiter)
//	return nil
//}
//
//// RegisterHandler 注册处理器
//func (s *TCPServer) RegisterHandler(handler MessageHandler) {
//	s.handlers[handler.GetName()] = handler
//	log.Info("Registered handler", zap.String("name", handler.GetName()))
//}

// Start 启动服务器
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return errors.Errorf("failed to start listener: %v", err)
	}
	s.listener = listener

	log.Info("TCP server start", zap.String("addr", s.config.Address))

	// 启动连接清理协程
	s.wg.Add(1)
	go s.connectionCleaner()

	// 接受连接
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				log.Warn(errors.Wrap(err, "Accept error"))
				continue
			}
		}

		// 检查连接数限制
		if !s.acceptConnection() {
			log.Warn("Max connections reached, rejecting", zap.String("addr", conn.RemoteAddr().String()))
			utils.SafeCloseConn(conn)
			continue
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) MessageHandle(fn func(result *core.ScanResult) error) {
	s.messageHandle = fn
}

func (s *Server) acceptConnection() bool {
	current := atomic.LoadInt32(&s.connectionCounts)
	if current >= int32(s.config.MaxConnections) {
		return false
	}
	return atomic.CompareAndSwapInt32(&s.connectionCounts, current, current+1)
}

func (s *Server) releaseConnection() {
	atomic.AddInt32(&s.connectionCounts, -1)
}

// handleConnection 处理单个连接
func (s *Server) handleConnection(conn net.Conn) {
	defer utils.NormalRecover()
	defer utils.SafeCloseConn(conn)
	defer s.wg.Done()
	defer s.releaseConnection()

	// 连接优化
	err := utils.OptimalTcpConn(conn, s.config.ReadBufferSize, s.config.WriteBufferSize)
	if err != nil {
		log.Warn(errors.Wrap(err, "OptimalTcpConn fail"))
		return
	}

	deviceConn := NewDeviceConnection(s, conn)
	defer s.RemoveConnection(deviceConn)
	err = deviceConn.Scan(s.scheme)
	if err != nil {
		log.Warn(errors.Wrap(err, "Scan fail"), deviceConn.zapFields()...)
		return
	}
}

func (s *Server) AddConnection(conn *DeviceConnection) {
	s.connections.Store(conn.key, conn)
}

func (s *Server) RemoveConnection(conn *DeviceConnection) {
	s.connections.Delete(conn.key)
	s.RemoveDevice(conn)
}

func (s *Server) AddDevice(sn string, conn *DeviceConnection) {
	s.devices.Store(sn, conn.DeviceKey(sn))
}

func (s *Server) RemoveDevice(conn *DeviceConnection) {
	if conn.sn != "" {
		s.removeDevice(conn.sn, conn)
	}
	for _, subDevice := range conn.subDevices {
		s.removeDevice(subDevice, conn)
	}
}

func (s *Server) removeDevice(sn string, conn *DeviceConnection) {
	if key, ok := s.devices.Load(sn); ok {
		if key == conn.DeviceKey(sn) {
			s.devices.Delete(sn)
		}
	}
}

func (s *Server) GetDevice(sn string) *DeviceConnection {
	connKey, ok := s.devices.Load(sn)
	if !ok {
		return nil
	}
	conn, ok := s.connections.Load(connKey.(string))
	if !ok {
		return nil
	}
	return conn.(*DeviceConnection)
}

// heartbeatMonitor 心跳监控
//func (s *Server) heartbeatMonitor(conn *DeviceConnection, hbCfg *HeartbeatConfig) {
//ticker := time.NewTicker(time.Duration(hbCfg.Interval) * time.Second)
//defer ticker.Stop()
//
//timeout := time.Duration(hbCfg.Timeout) * time.Second
//
//for {
//	select {
//	case <-conn.ctx.Done():
//		return
//	case <-ticker.C:
//		lastActive := conn.GetLastActiveTime()
//		if time.Since(lastActive) > timeout {
//			log.Warn("Heartbeat timeout, closing connection", zap.String("addr", conn.RemoteAddr()))
//			conn.Close()
//			return
//		}
//	}
//}
//}

// connectionCleaner 连接清理器
func (s *Server) connectionCleaner() {
	defer utils.Catch(func(reason any) {
		go s.releaseConnection()
	})
	defer s.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			count := 0
			s.connections.Range(func(key, value any) bool {
				conn, ok := value.(*DeviceConnection)
				if !ok {
					return true
				}
				if s.config.DeviceDiscoveryMode == DeviceDiscoveryModeSync {
					conn.Heartbeat(s.config.DeviceSyncDuration, s.config.DeviceDiscoveryFunc, s.config.DeviceDiscoveryCmd)
				}
				count++
				return true
			})
			log.Info("Active connections", zap.Int("count", count))
		}
	}
}

// Stop 停止服务器
func (s *Server) Stop() error {
	zap.L().Info("Stopping TCP server")
	s.cancel()

	if s.listener != nil {
		err := s.listener.Close()
		if err != nil {
			log.Warn(errors.Wrap(err, "Error closing listener"))
		}
	}

	// 关闭所有连接
	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*DeviceConnection); ok {
			conn.Close()
		}
		return true
	})

	// 等待所有goroutine结束
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("Server shutdown complete")
		return nil
	case <-time.After(30 * time.Second):
		return errors.Errorf("shutdown timeout")
	}
}
