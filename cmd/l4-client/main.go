package main

import (
	"crypto/tls"
	"fmt"
	"log"
)

func main() {
	// Proxy server address
	proxyAddress := "localhost:443"

	// Target service name to include in SNI
	targetService := "example.com"

	// TLS configuration
	tlsConfig := &tls.Config{
		ServerName:         targetService, // SNI field
		InsecureSkipVerify: true,          // Skip verification for self-signed certificates
	}

	// Establish a TLS connection to the proxy
	log.Println("Dialing connection to proxy")
	conn, err := tls.Dial("tcp", proxyAddress, tlsConfig)
	if err != nil {
		log.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer func(conn *tls.Conn) {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Failed to close connection: %v", err)
		}
	}(conn)

	// Send a test message
	message := "Hello from client!"
	log.Println("Sending message to proxy")
	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}
	fmt.Println("Message sent to proxy:", message)

	// Read the response from the backend
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}
	fmt.Println("Response from backend:", string(buffer[:n]))
}
