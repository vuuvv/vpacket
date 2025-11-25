package vpacket

import (
	"fmt"
	"testing"
)

func TestTcp(t *testing.T) {
	Setup()
	scheme, err := NewSchemeFromFile("./resources/protocols.yaml")
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}

	err = scheme.Setup()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}

	server := NewTcpServer(&TcpServerConfig{
		Address:         "0.0.0.0:3001",
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		MaxConnections:  10000,
	}, scheme)

	err = server.Start()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
}
