package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
)

// Client represents a single client that connects to the proxy and sends messages.
type Client struct {
	id        int
	stopChan  chan struct{}
	waitGroup *sync.WaitGroup
}

// NewClient initializes a new Client instance.
func NewClient(id int, waitGroup *sync.WaitGroup) *Client {
	return &Client{
		id:        id,
		stopChan:  make(chan struct{}),
		waitGroup: waitGroup,
	}
}

// Start connects the client to the proxy and sends periodic messages until stopped.
func (c *Client) Start(proxyAddress, targetService string) {
	defer c.waitGroup.Done()

	// TLS configuration
	tlsConfig := &tls.Config{
		ServerName:         targetService, // SNI field
		InsecureSkipVerify: true,          // Skip verification for self-signed certificates
	}

	for {
		select {
		case <-c.stopChan:
			log.Printf("Client %d stopping", c.id)
			return
		default:
			conn, err := tls.Dial("tcp", proxyAddress, tlsConfig)
			if err != nil {
				log.Printf("Client %d failed to connect: %v", c.id, err)
				time.Sleep(1 * time.Second) // Retry after a second
				continue
			}

			message := fmt.Sprintf("Hello from client %d", c.id)
			_, err = conn.Write([]byte(message))
			if err != nil {
				log.Printf("Client %d failed to send message: %v", c.id, err)
			} else {
				log.Printf("Client %d sent message: %s", c.id, message)
			}

			// Read the response from the backend
			buffer := make([]byte, 1024)
			n, err := conn.Read(buffer)
			if err != nil {
				log.Printf("Failed to read response: %v", err)
			}
			log.Println("Response from backend:", string(buffer[:n]))

			conn.Close()
			time.Sleep(1 * time.Second) // Wait before sending the next message
		}
	}
}

// Stop signals the client to stop running.
func (c *Client) Stop() {
	close(c.stopChan)
}

func main() {
	const numClients = 1000
	const proxyAddress = "127.0.0.1:443"

	var wg sync.WaitGroup
	clients := make([]*Client, numClients)

	// Start all clients
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		client := NewClient(i, &wg)
		clients[i] = client
		go client.Start(proxyAddress, "service"+strconv.FormatInt(int64(i%50+1), 10)+".com")
	}

	// Wait for some time (e.g., 10 seconds) to simulate workload
	time.Sleep(600 * time.Second)

	// Stop all clients
	for _, client := range clients {
		client.Stop()
	}

	// Wait for all clients to finish
	wg.Wait()

	log.Println("All clients stopped")
}
