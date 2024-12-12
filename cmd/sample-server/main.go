package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
)

func main() {
	// Backend service name and address
	serviceName := "exampleexampleexample.com"
	serviceAddress := "127.0.0.1:8080"
	proxyRegistrationEndpoint := "http://localhost:8081/register"

	// Load TLS certificate and key
	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Fatalf("Failed to load TLS certificate: %v", err)
	}

	// Register the backend with the proxy
	err = registerWithProxy(proxyRegistrationEndpoint, serviceName, serviceAddress)
	if err != nil {
		log.Fatalf("Failed to register with proxy: %v", err)
	}
	log.Printf("Backend registered: %s -> %s", serviceName, serviceAddress)

	// Start the backend server
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

	listener, err := tls.Listen("tcp", serviceAddress, tlsConfig)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	log.Printf("Backend server listening on %s", serviceAddress)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Printf("Failed to close connection: %v", err)
		}
	}(conn)

	// Read client message
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Printf("Failed to read message: %v", err)
		return
	}
	message := string(buffer[:n])
	log.Printf("Received message: %s", message)

	// Send a response to the client
	response := "Hello from backend!"
	_, err = conn.Write([]byte(response))
	if err != nil {
		log.Printf("Failed to send response: %v", err)
	}
}

func registerWithProxy(endpoint, name, address string) error {
	// Prepare the registration payload
	payload := map[string]string{
		"name":    name,
		"address": address,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal registration payload: %w", err)
	}

	// Make the HTTP POST request to the proxy
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("registration failed: %s", string(body))
	}

	return nil
}
