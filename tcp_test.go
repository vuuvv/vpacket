package vpacket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

	tcpServer := NewTcpServer(&TcpServerConfig{
		Address:         "0.0.0.0:30001",
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		MaxConnections:  10000,
	}, scheme)

	http.HandleFunc("/command", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, fmt.Sprintf("JSON解析错误: %v", err), http.StatusBadRequest)
			return
		}

		sn, ok := payload["sn"].(string)
		if !ok {
			http.Error(w, fmt.Sprintf("序列号缺失或序列号格式错误: %v", sn), http.StatusBadRequest)
			return
		}
		// TODO: handle params
		err = tcpServer.SendCommand(sn, payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	go func() {
		log.Println("Starting http server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start http server: %v", err)
		}
	}()

	err = tcpServer.Start()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
}
