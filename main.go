package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	hook "github.com/robotn/gohook"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Bindings map[string]string `yaml:"bindings"`
}

func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

type ConnectionManager struct {
	tcpClients map[string]net.Conn
	mu         sync.Mutex
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		tcpClients: make(map[string]net.Conn),
	}
}

func (cm *ConnectionManager) SendMessage(message string, canRetry bool) {
	ipAddress := "127.0.0.1"
	port := 29200

	bHeader := []byte{0x43, 0x4d, 0x4e, 0x44, 0x00, 0xd2, 0x00, 0x00} // CMND 0x00d20000
	bCommand := append([]byte(message), '\n')
	bPadding := []byte{0x00, 0x00}
	bLength := make([]byte, 4)
	binary.BigEndian.PutUint32(bLength, uint32(len(bCommand)+13))
	bTerminator := []byte{0x00}

	data := bytes.Join([][]byte{bHeader, bLength, bPadding, bCommand, bTerminator}, nil)

	tcpClientIdentifier := fmt.Sprintf("%s::%d", ipAddress, port)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, exists := cm.tcpClients[tcpClientIdentifier]
	if !exists {
		var err error
		conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", ipAddress, port))
		if err != nil {
			fmt.Println("Error connecting:", err)
			return
		}
		cm.tcpClients[tcpClientIdentifier] = conn
	}

	if _, err := conn.Write(data); err != nil {
		fmt.Println("Error sending data:", err)
		conn.Close()
		delete(cm.tcpClients, tcpClientIdentifier)
		if canRetry {
			cm.SendMessage(message, false)
		}
	}
}

func (cm *ConnectionManager) InitializeClients() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.tcpClients = make(map[string]net.Conn)
}

func (cm *ConnectionManager) Dispose() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for _, client := range cm.tcpClients {
		client.Close()
	}
	cm.tcpClients = make(map[string]net.Conn)
}

func main() {

	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v\n", err)
	}

	fmt.Println("Listening for key presses in the background...")

	for keyStr, message := range config.Bindings {
		// Capture the current value of keyStr and message in a closure
		func(keyStr, message string) {

			fmt.Println("Press", keyStr, message)
			hook.Register(hook.KeyDown, []string{keyStr}, func(e hook.Event) {
				cm := NewConnectionManager()
				fmt.Printf("Key '%s' pressed, sending message: %s\n", keyStr, message)
				cm.SendMessage(message, true)
				cm.Dispose()
			})
		}(keyStr, message)
	}

	s := hook.Start()
	<-hook.Process(s)
}
