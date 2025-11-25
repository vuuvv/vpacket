package utils

import (
	"github.com/vuuvv/errors"
	"go.uber.org/zap"
	"net"
	"time"
)

func SafeCloseConn(conn net.Conn) {
	if conn != nil {
		zap.L().Info("Closing connections", zap.String("addr", conn.RemoteAddr().String()))
		if err := conn.Close(); err != nil {
			zap.L().Warn("close error: %v", zap.Error(err))
		}
	}
}

func OptimalTcpConn(conn net.Conn, readBufferSize, writeBufferSize int) error {
	// 转换为TCP连接
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return errors.New("not a tcp connection")
	}

	// 启用TCP保活机制
	// 系统会定期检查连接是否仍然有效
	// 如果对端无响应，系统会关闭连接并返回错误
	// 防止"半开连接"（对端异常断开但本端不知情）
	err := tcpConn.SetKeepAlive(true)
	if err != nil {
		return errors.WithStack(err)
	}

	// 设置保活探测间隔
	// 每3分钟发送一次保活探测包
	// 默认值通常为15分钟，这里设置为更敏感的3分钟
	// 更快的连接失效检测
	err = tcpConn.SetKeepAlivePeriod(3 * time.Minute)
	if err != nil {
		return errors.WithStack(err)
	}

	// 禁用Nagle算法
	// Nagle算法会缓冲小数据包，合并发送以减少网络开销
	// 设置true表示立即发送数据，降低延迟
	// 适合实时性要求高的应用（如游戏、实时通信）
	err = tcpConn.SetNoDelay(true)
	if err != nil {
		return errors.WithStack(err)
	}

	// 设置内核读缓冲区大小
	// 影响TCP接收窗口大小
	// 较大的缓冲区可以提高吞吐量，减少系统调用次数
	// 但会增加内存占用
	err = tcpConn.SetReadBuffer(readBufferSize)
	if err != nil {
		return errors.WithStack(err)
	}

	// 设置内核写缓冲区大小
	//
	// 影响TCP发送窗口大小
	// 较大的缓冲区有助于应对突发流量
	// 需要根据网络条件调整
	err = tcpConn.SetWriteBuffer(writeBufferSize)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
