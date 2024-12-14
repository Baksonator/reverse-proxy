package main

import (
	"bytes"
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
			bodyContent := "Hello from client!"
			body := bytes.NewBufferString(bodyContent)

			req, err := http.NewRequest("POST", proxyAddress, body)
			if err != nil {
				log.Fatalf("Failed to create request: %v", err)
			}

			// Set the Host header for routing
			//req.Host = targetService
			req.Header.Set("Content-Type", "text/plain")

			// Send a request to the proxy
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Client %d failed to send request: %v", c.id, err)
				time.Sleep(1 * time.Second)
				continue
			}

			log.Printf("Client %d received response: %s", c.id, resp.Status)
			// Read and display the response
			bodyResp, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Failed to read response: %v", err)
			}

			log.Printf("Client %d received response body: %s", c.id, string(bodyResp))

			resp.Body.Close()

			time.Sleep(1 * time.Second)
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
	const proxyAddress = "http://localhost:80/" // Update to your proxy's actual address

	var wg sync.WaitGroup
	clients := make([]*Client, numClients)

	// Start all clients
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		client := NewClient(i, &wg)
		clients[i] = client
		go client.Start(proxyAddress, "service"+strconv.FormatInt(int64(i%50+1), 10)+".com")
	}

	// Run the test for 30 seconds
	time.Sleep(600 * time.Second)

	// Stop all clients
	for _, client := range clients {
		client.Stop()
	}

	// Wait for all clients to finish
	wg.Wait()

	log.Println("All clients stopped")
}
