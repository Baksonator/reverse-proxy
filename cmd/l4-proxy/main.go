package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
)

type Config struct {
	Backends sync.Map // Thread-safe map for backend name-to-address mapping
}

// Proxy listens for incoming connections
func startProxy(address string, config *Config) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	defer listener.Close()

	log.Printf("Listening on %s", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(conn, config)
	}
}

// Handle individual connections
func handleConnection(conn net.Conn, config *Config) {
	bufferedConn := newBufferedConn(conn)

	defer bufferedConn.Close()

	sni, err := extractSNI(bufferedConn)
	if err != nil {
		log.Printf("Failed to extract SNI: %v", err)
		return
	}

	// Parse SNI
	serviceName, err := parseSNI(sni)
	if err != nil {
		log.Printf("Failed to parse SNI: %v", err)
		return
	}

	// Step 2: Discover the backend
	value, ok := config.Backends.Load(serviceName)
	if !ok {
		log.Printf("No backend found for SNI: %s", sni)
		return
	}
	backendAddr := value.(string)

	// Forward traffic
	if err := forwardTraffic(bufferedConn, backendAddr); err != nil {
		log.Printf("Failed to forward traffic: %v", err)
	}
}

func extractSNI(bufferedConn bufferedConn) (string, error) {
	// Peek into the connection to read the TLS ClientHello without consuming the data
	// Use a buffered reader to peek at the handshake
	// Read enough data to cover the ClientHello message
	buf, err := bufferedConn.Peek(253)
	if err != nil {
		return "", fmt.Errorf("failed to read TLS handshake: %w", err)
	}

	// Parse the ClientHello to extract the SNI
	sni, err := parseTLSClientHello(buf)
	if err != nil {
		return "", fmt.Errorf("failed to parse ClientHello: %w", err)
	}

	return sni, nil
}

func parseTLSClientHello(data []byte) (string, error) {
	if len(data) < 5 {
		return "", fmt.Errorf("data too short for TLS handshake")
	}

	// Ensure this is a TLS Handshake record
	if data[0] != 0x16 { // Record type: Handshake
		return "", fmt.Errorf("not a TLS handshake")
	}

	// Ensure the protocol version is at least TLS 1.0
	if data[1] != 0x03 || (data[2] != 0x01 && data[2] != 0x02 && data[2] != 0x03) {
		return "", fmt.Errorf("unsupported TLS version")
	}

	// Get the length of the handshake record
	recordLen := int(data[3])<<8 | int(data[4])
	if len(data)-5 < recordLen {
		return "", fmt.Errorf("incomplete handshake record")
	}

	// Skip to the ClientHello message
	handshakeType := data[5]
	if handshakeType != 0x01 { // Handshake type: ClientHello
		return "", fmt.Errorf("not a ClientHello message")
	}

	// Skip past the fixed-length parts of the ClientHello
	offset := 43
	if len(data) < offset {
		return "", fmt.Errorf("invalid ClientHello message")
	}

	// Get the session ID length and skip it
	sessionIDLen := int(data[offset])
	offset += 1 + sessionIDLen
	if len(data) < offset {
		return "", fmt.Errorf("invalid ClientHello session ID")
	}

	// Get the cipher suites length and skip it
	cipherSuitesLen := int(data[offset])<<8 | int(data[offset+1])
	offset += 2 + cipherSuitesLen
	if len(data) < offset {
		return "", fmt.Errorf("invalid ClientHello cipher suites")
	}

	// Get the compression methods length and skip it
	compressionMethodsLen := int(data[offset])
	offset += 1 + compressionMethodsLen
	if len(data) < offset {
		return "", fmt.Errorf("invalid ClientHello compression methods")
	}

	// Start parsing extensions
	extensionsLen := int(data[offset])<<8 | int(data[offset+1])
	offset += 2
	if len(data) < offset+extensionsLen {
		return "", fmt.Errorf("extensions length exceeds data size")
	}

	// Parse each extension
	end := offset + extensionsLen
	for offset+4 <= end {
		extType := uint16(data[offset])<<8 | uint16(data[offset+1])
		extLen := int(data[offset+2])<<8 | int(data[offset+3])
		offset += 4

		if offset+extLen > end {
			return "", fmt.Errorf("extension length exceeds data size")
		}

		// Check if this is the SNI extension (type 0x00)
		if extType == 0x00 {
			// Parse the SNI extension
			sniLen := int(data[offset+3])<<8 | int(data[offset+4])
			if offset+5+sniLen > end {
				return "", fmt.Errorf("invalid SNI length")
			}
			return string(data[offset+5 : offset+5+sniLen]), nil
		}

		// Move to the next extension
		offset += extLen
	}

	return "", fmt.Errorf("SNI not found in ClientHello")
}

// Parse SNI (custom logic goes here)
func parseSNI(sni string) (string, error) {
	// Example: Return the raw SNI as the service name
	return sni, nil
}

// Forward traffic to the backend service
func forwardTraffic(conn net.Conn, backendAddr string) error {
	backendConn, err := net.Dial("tcp", backendAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to backend: %w", err)
	}
	defer backendConn.Close()

	log.Printf("Forwarding traffic between client and backend")
	done := make(chan struct{})

	// Client -> Backend
	go func() {
		log.Println("Starting client -> backend forwarding")
		_, err := io.Copy(backendConn, conn)
		if err != nil {
			log.Printf("Error forwarding client to backend: %v", err)
		}
		log.Println("Finished client -> backend forwarding")
		done <- struct{}{}
	}()

	// Backend -> Client
	go func() {
		log.Println("Starting backend -> client forwarding")
		_, err := io.Copy(conn, backendConn)
		if err != nil {
			log.Printf("Error forwarding backend to client: %v", err)
		}
		log.Println("Finished backend -> client forwarding")
		done <- struct{}{}
	}()

	// Wait for both directions to finish
	<-done
	<-done
	log.Println("Handled connection")

	return nil
}

type bufferedConn struct {
	r        *bufio.Reader
	net.Conn // So that most methods are embedded
}

func newBufferedConn(c net.Conn) bufferedConn {
	return bufferedConn{bufio.NewReader(c), c}
}

func newBufferedConnSize(c net.Conn, n int) bufferedConn {
	return bufferedConn{bufio.NewReaderSize(c, n), c}
}

func (b bufferedConn) Peek(n int) ([]byte, error) {
	return b.r.Peek(n)
}

func (b bufferedConn) Read(p []byte) (int, error) {
	return b.r.Read(p)
}

func startRegistrationServer(config *Config, address string) {
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var registration struct {
			Name    string `json:"name"`
			Address string `json:"address"`
		}

		// Decode the JSON payload
		if err := json.NewDecoder(r.Body).Decode(&registration); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		// Validate the registration
		if registration.Name == "" || registration.Address == "" {
			http.Error(w, "Name and Address are required", http.StatusBadRequest)
			return
		}

		// Register the backend
		config.Backends.Store(registration.Name, registration.Address)
		log.Printf("Registered backend: %s -> %s", registration.Name, registration.Address)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Backend %s registered successfully", registration.Name)
	})

	log.Printf("Registration server listening on %s", address)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatalf("Failed to start registration server: %v", err)
	}
}

func main() {
	// Initialize the proxy configuration
	config := &Config{
		Backends: sync.Map{},
	}

	// Start the registration server
	go startRegistrationServer(config, ":8081") // Registration server on port 8081

	err := startProxy(":443", config)
	if err != nil {
		log.Fatalf("Failed to start proxy server: %v", err)
	}
	log.Println("Proxy server listening on :443")
}
