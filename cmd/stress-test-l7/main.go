package main

import (
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Client represents a single client that connects to the proxy.
type Client struct {
	id        int
	stopChan  chan struct{}
	waitGroup *sync.WaitGroup
}

// NewClient initializes a new Client.
func NewClient(id int, waitGroup *sync.WaitGroup) *Client {
	return &Client{
		id:        id,
		stopChan:  make(chan struct{}),
		waitGroup: waitGroup,
	}
}

// Start simulates the client sending periodic requests to the proxy.
func (c *Client) Start(proxyAddress, targetService string) {
	defer c.waitGroup.Done()

	// Configure HTTP client with insecure TLS for testing self-signed certificates
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	for {
		select {
		case <-c.stopChan:
			log.Printf("Client %d stopping", c.id)
			return
		default:
			req, err := http.NewRequest("GET", proxyAddress, nil)
			if err != nil {
				log.Fatalf("Failed to create request: %v", err)
			}

			// Set the Host header for routing
			req.Host = targetService

			// Send a request to the proxy
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Client %d failed to send request: %v", c.id, err)
				time.Sleep(1 * time.Second)
				continue
			}

			log.Printf("Client %d received response: %s", c.id, resp.Status)
			resp.Body.Close()
			// Read and display the response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Failed to read response: %v", err)
			}

			log.Printf("Client %d received response body: %s", c.id, body)

			time.Sleep(100 * time.Millisecond) // Simulate workload
		}
	}
}

// Stop signals the client to stop running.
func (c *Client) Stop() {
	close(c.stopChan)
}

// Main function to spin up 1,000 clients
func main() {
	const numClients = 1000
	const proxyAddress = "https://localhost:443" // Update to your proxy's actual address

	var wg sync.WaitGroup
	clients := make([]*Client, numClients)

	// Start all clients
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		client := NewClient(i, &wg)
		clients[i] = client
		go client.Start(proxyAddress, "service"+strconv.FormatInt(int64(i%100+1), 10)+".com")
	}

	// Run the test for 30 seconds
	time.Sleep(30 * time.Second)

	// Stop all clients
	for _, client := range clients {
		client.Stop()
	}

	// Wait for all clients to finish
	wg.Wait()

	log.Println("All clients stopped")
}
