package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Backend configuration
	port := "8080"                                                   // Backend port
	proxyRegistrationURL := "http://localhost:8081/register-backend" // Proxy registration endpoint
	host := "example.com"                                            // Hostname to register for

	// Register this backend with the proxy
	err := registerWithProxy(proxyRegistrationURL, host, "http://127.0.0.1:"+port)
	if err != nil {
		log.Fatalf("Failed to register backend with proxy: %v", err)
	}
	log.Printf("Backend registered successfully with proxy: %s -> %s", host, "http://127.0.0.1:"+port)

	// Start the backend server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Hello from backend on port %s!", port)
	})

	log.Printf("Backend server listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start backend server: %v", err)
	}
}

// registerWithProxy registers this backend with the proxy
func registerWithProxy(proxyURL, host, backend string) error {
	// Prepare the registration payload
	payload := map[string]string{
		"host":    host,
		"backend": backend,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal registration payload: %w", err)
	}

	// Send the registration request
	resp, err := http.Post(proxyURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registration failed with status code: %d", resp.StatusCode)
	}

	return nil
}
