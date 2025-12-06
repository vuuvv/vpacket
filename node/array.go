package node

import "github.com/vuuvv/vpacket/core"

type ArrayNode struct {
	core.BaseNode
	Items []core.Node
}
