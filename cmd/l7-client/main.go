package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	proxyURL := "https://localhost" // Proxy address
	host := "example.com"           // Host header to use for routing
	numRequests := 5                // Number of requests to send

	// Configure HTTP client with insecure TLS for testing self-signed certificates
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	for i := 1; i <= numRequests; i++ {
		// Create a new request
		req, err := http.NewRequest("GET", proxyURL, nil)
		if err != nil {
			log.Fatalf("Failed to create request: %v", err)
		}

		// Set the Host header for routing
		req.Host = host

		// Send the request
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// Read and display the response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Failed to read response: %v", err)
		}

		fmt.Printf("Request %d: %s\n", i, string(body))
	}
}
