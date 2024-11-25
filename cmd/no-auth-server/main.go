package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
)

func main() {
	// Backend service name and address
	serviceName := "example.com"
	serviceAddress := "127.0.0.1:8080"
	proxyRegistrationEndpoint := "http://localhost:8081/register"

	// Register the backend with the proxy
	err := registerWithProxy(proxyRegistrationEndpoint, serviceName, serviceAddress)
	if err != nil {
		log.Fatalf("Failed to register with proxy: %v", err)
	}
	log.Printf("Backend registered: %s -> %s", serviceName, serviceAddress)

	// Start listening for plaintext connections
	listener, err := net.Listen("tcp", serviceAddress)
	if err != nil {
		log.Fatalf("Failed to start backend server: %v", err)
	}
	defer listener.Close()

	log.Printf("Backend server listening on %s", serviceAddress)

	for {
		// Accept a new connection
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		// Handle the connection
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read the client's message
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Printf("Failed to read message: %v", err)
		return
	}
	message := string(buffer[:n])
	log.Printf("Received message: %s", message)

	// Send a response to the client
	response := "Hello from plaintext backend!"
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
