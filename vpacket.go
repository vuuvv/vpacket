package vpacket

import (
	"github.com/vuuvv/vpacket/core"
	"github.com/vuuvv/vpacket/framing"
	"github.com/vuuvv/vpacket/node"
)

type Protocol = core.Protocol
type Config = core.Config
type Node = core.Node

type FramingRule = core.FramingRule

var NewContext = core.NewContext

type Scanner = core.Codec
type ScanResult = core.ScanResult

var NewScanner = core.NewCodec
var NewScannerFromBytes = core.NewScannerFromBytes
var NewScannerFromFile = core.NewScannerFromFile

func Setup() {
	framing.Register()
	node.Register()
}
