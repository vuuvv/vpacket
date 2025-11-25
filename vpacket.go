package vpacket

import (
	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/framing"
	"github.com/vuuvv/vpacket/log"
	"github.com/vuuvv/vpacket/node"
	"github.com/vuuvv/vpacket/tcp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Scheme = core.Scheme

var NewScheme = core.NewScheme
var NewSchemeFromFile = core.NewSchemeFromFile

type Protocol = core.Protocol
type Node = core.Node

type FramingRule = core.FramingRule

type Context = core.Context

var NewContext = core.NewContext

type Codec = core.Codec
type ScanResult = core.ScanResult

var NewCodec = core.NewCodec
var NewCodecFromBytes = core.NewCodecFromBytes
var NewCodecFromFile = core.NewCodecFromFile

type TcpServer = tcp.Server
type TcpServerConfig = tcp.ServerConfig

var NewTcpServer = tcp.NewTCPServer

func Setup() {
	var logger *zap.Logger
	var err error
	if !zap.L().Core().Enabled(zapcore.PanicLevel) {
		logger, err = zap.NewDevelopment()
		if err != nil {
			panic(err)
		}
	} else {
		logger = zap.L()
	}
	log.SetLogger(logger)
	log.SetDefaultLogger(logger)
	log.SetHttpErrorLogger(logger)

	framing.Register()
	node.Register()
}
