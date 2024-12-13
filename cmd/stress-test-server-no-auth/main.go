package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// Server represents a backend server instance.
type Server struct {
	serviceName string
	address     string
	stopChan    chan struct{}
	waitGroup   *sync.WaitGroup
	listener    net.Listener
}

// NewServer initializes a new Server instance.
func NewServer(serviceName, address string, waitGroup *sync.WaitGroup) *Server {
	listener, err := net.Listen("tcp", address)

	if err != nil {
		log.Printf("Server %s failed to start listener: %v", serviceName, err)
		return nil
	}

	return &Server{
		serviceName: serviceName,
		address:     address,
		stopChan:    make(chan struct{}),
		waitGroup:   waitGroup,
		listener:    listener,
	}
}

// Start launches the server to listen for connections and process messages.
func (s *Server) Start(certFile, keyFile, proxyRegistrationEndpoint string) {
	defer s.waitGroup.Done()

	// Register the server with the proxy
	err := registerWithProxy(proxyRegistrationEndpoint, s.serviceName, s.address)
	if err != nil {
		log.Printf("Server %s failed to register with proxy: %v", s.serviceName, err)
		return
	}
	log.Printf("Server %s registered with proxy", s.serviceName)

	defer s.listener.Close()

	log.Printf("Server %s listening on %s", s.serviceName, s.address)

	for {
		select {
		case <-s.stopChan:
			log.Printf("Server %s stopping", s.serviceName)
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				if opErr, ok := err.(net.Error); ok && opErr.Timeout() {
					continue
				}
				log.Printf("Server %s failed to accept connection: %v", s.serviceName, err)
				continue
			}

			go s.handleConnection(conn)
		}
	}
}

// Stop signals the server to stop listening for connections.
func (s *Server) Stop() {
	close(s.stopChan)
	s.listener.Close()
}

// handleConnection processes an incoming client connection.
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read client message
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Printf("Server %s failed to read message: %v", s.serviceName, err)
		return
	}
	message := string(buffer[:n])
	log.Printf("Server %s received message: %s", s.serviceName, message)

	// Send a response to the client
	response := fmt.Sprintf("Hello from server %s!", s.serviceName)
	_, err = conn.Write([]byte(response))
	if err != nil {
		log.Printf("Server %s failed to send response: %v", s.serviceName, err)
	}
}

// registerWithProxy registers the server with the proxy.
func registerWithProxy(endpoint, name, address string) error {
	payload := map[string]string{
		"name":    name,
		"address": address,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal registration payload: %w", err)
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("registration failed: %s", string(body))
	}
	return nil
}

func main() {
	const (
		numServers                = 100
		proxyRegistrationEndpoint = "http://localhost:8081/register"
		certFile                  = "cert.pem"
		keyFile                   = "key.pem"
		startPort                 = 8082
	)

	var wg sync.WaitGroup
	servers := make([]*Server, numServers)

	// Start all servers
	for i := 0; i < numServers; i++ {
		serviceName := fmt.Sprintf("service%d.com", i+1)
		address := fmt.Sprintf("127.0.0.1:%d", startPort+i)
		wg.Add(1)

		server := NewServer(serviceName, address, &wg)
		servers[i] = server
		go server.Start(certFile, keyFile, proxyRegistrationEndpoint)
	}

	// Wait for some time (e.g., 10 seconds) to simulate workload
	time.Sleep(120 * time.Second)

	// Stop all servers
	for _, server := range servers {
		server.Stop()
	}

	// Wait for all servers to finish
	wg.Wait()

	log.Println("All servers stopped")
}
