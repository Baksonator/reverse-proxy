package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

func startServer(port int) {
	mux := http.NewServeMux() // Use a custom ServeMux
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Hello from backend on port %d!", port)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux, // Use the custom ServeMux
	}

	log.Printf("Backend server listening on port %s", port)
	if err := server.ListenAndServe(); err != nil {
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

func main() {
	proxyRegistrationURL := "http://localhost:8081/register-backend" // Proxy registration endpoint // Hostname to register for

	// Run 10 servers concurrently
	for i := 0; i < 100; i++ {
		port := 8082 + i
		host := fmt.Sprintf("service%d.com", i+1)
		go registerWithProxy(proxyRegistrationURL, host, "http://127.0.0.1:"+strconv.FormatInt(int64(port), 10))
		go startServer(port)
	}

	time.Sleep(40 * time.Second)
}
