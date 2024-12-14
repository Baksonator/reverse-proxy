package main

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/cpu"
	"io"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"
)

type Config struct {
	Backends       *sync.Map // Map of host/path to backend addresses
	BackendIndices *sync.Map // Tracks the next backend for each host/path (round-robin)
	Cache          *sync.Map // A thread-safe cache for storing responses
	TLSCertFile    string    // Path to TLS certificate file
	TLSKeyFile     string    // Path to TLS private key file
}

// Metrics for Prometheus
var (
	activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "l7_proxy_active_connections",
		Help: "Number of active connections being handled by the proxy.",
	})
	totalRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "l7_proxy_total_requests",
		Help: "Total number of requests processed by the proxy.",
	})
	requestErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "l7_proxy_request_errors",
		Help: "Total number of request errors encountered by the proxy.",
	})
	requestLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "l7_proxy_request_latency_seconds",
		Help:    "Histogram of request latency in seconds.",
		Buckets: prometheus.LinearBuckets(0, 5, 60),
	})
)

var (
	cpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "l7_proxy_cpu_usage_seconds_total",
		Help: "Total CPU time used by the proxy process in seconds.",
	})
	memoryUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "l7_proxy_memory_usage_bytes",
		Help: "Memory usage of the proxy process in bytes.",
	})
	goroutines = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "l7_proxy_goroutines",
		Help: "Number of goroutines currently running.",
	})
)

var (
	cpuUsageMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "l7_cpu_usage_percent",
		Help: "Current CPU usage as a percentage.",
	})
)

func getCPUUsage() (float64, error) {
	percentages, err := cpu.Percent(15, false) // Get overall CPU usage
	if err != nil || len(percentages) <= 0 {
		return 0, err
	}
	return percentages[0], nil
}

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(activeConnections, totalRequests, requestErrors, requestLatency, cpuUsage, cpuUsageMetric, memoryUsage, goroutines)
}

func startBackendRegistrationAPI(config *Config) {
	http.HandleFunc("/register-backend", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var registration struct {
			Host    string `json:"host"`
			Backend string `json:"backend"`
		}

		// Parse the JSON payload
		if err := json.NewDecoder(r.Body).Decode(&registration); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		// Validate the input
		if registration.Host == "" || registration.Backend == "" {
			http.Error(w, "Host and Backend are required", http.StatusBadRequest)
			return
		}

		// Add the backend
		addBackend(config, registration.Host, registration.Backend)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Backend %s registered successfully for host %s", registration.Backend, registration.Host)
	})

	go func() {
		log.Println("Backend registration API listening on :8081")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatalf("Failed to start backend registration API: %v", err)
		}
	}()
}

func getNextBackend(config *Config, host string) (string, error) {
	value, ok := config.Backends.Load(host)
	if !ok {
		return "", fmt.Errorf("no backends available for host: %s", host)
	}

	// Convert the backends map to a slice
	backends := value.(*sync.Map)
	var backendList []string
	backends.Range(func(key, _ any) bool {
		backendList = append(backendList, key.(string))
		return true
	})

	if len(backendList) == 0 {
		return "", fmt.Errorf("no backends available for host: %s", host)
	}

	// Get or initialize the index
	indexValue, _ := config.BackendIndices.LoadOrStore(host, 0)
	index := indexValue.(int)

	// Select the backend and update the index
	backend := backendList[index]
	config.BackendIndices.Store(host, (index+1)%len(backendList))

	return backend, nil
}

func getFromCache(config *Config, key string) ([]byte, bool) {
	value, ok := config.Cache.Load(key)
	if !ok {
		return nil, false
	}
	return value.([]byte), true
}

func storeInCache(config *Config, key string, data []byte) {
	config.Cache.Store(key, data)
}

func handleHTTPRequest(w http.ResponseWriter, r *http.Request, config *Config) {
	activeConnections.Inc()
	defer activeConnections.Dec()

	startTime := time.Now()
	totalRequests.Inc() // Increment total requests

	host := r.Host

	//// Check the cache
	//cacheKey := fmt.Sprintf("%s:%s", host, r.URL.String())
	//if cachedResponse, found := getFromCache(config, cacheKey); found {
	//	log.Printf("Cache hit for %s", cacheKey)
	//	w.Write(cachedResponse)
	//	return
	//}

	// Get the next backend using round-robin
	backendURL, err := getNextBackend(config, host)
	if err != nil {
		http.Error(w, "No backend available", http.StatusServiceUnavailable)
		log.Printf("No backend available for host: %s", host)
		return
	}

	// Create a new request to forward to the backend
	req, err := http.NewRequest(r.Method, backendURL+r.URL.Path, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		log.Printf("Failed to create request for backend: %v", err)
		return
	}
	req.Header = r.Header

	// Perform the request to the backend
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to connect to backend", http.StatusBadGateway)
		log.Printf("Failed to connect to backend: %v", err)
		return
	}
	defer resp.Body.Close()

	// Copy the response back to the client
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read backend response", http.StatusInternalServerError)
		log.Printf("Failed to read backend response: %v", err)
		return
	}

	// Cache the response
	//storeInCache(config, cacheKey, body)

	// Write the response back to the client
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(body)

	log.Printf("Forwarded request to backend: %s", backendURL)

	duration := time.Since(startTime).Milliseconds()
	requestLatency.Observe(float64(duration))
}

func collectCPUMetrics() {
	for {
		cpuUsageCurr, err := getCPUUsage()
		if err == nil {
			cpuUsageMetric.Set(cpuUsageCurr)
		}

		//time.Sleep(15 * time.Second)
	}
}

func addBackend(config *Config, host string, backend string) {
	// Get or initialize the backend list
	value, _ := config.Backends.LoadOrStore(host, &sync.Map{})
	backends := value.(*sync.Map)

	// Add the backend to the list
	backends.Store(backend, true) // `true` acts as a placeholder
	log.Printf("Registered backend: %s -> %s", host, backend)
}

func collectProfilingMetrics() {
	var memStats runtime.MemStats

	for {
		// Capture CPU usage
		cpuUsage.Set(float64(runtime.NumCPU())) // Placeholder for per-core CPU stats

		// Capture memory usage
		runtime.ReadMemStats(&memStats)
		memoryUsage.Set(float64(memStats.Alloc))

		// Goroutine count
		goroutines.Set(float64(runtime.NumGoroutine()))

		time.Sleep(5 * time.Second) // Adjust sampling interval as needed
	}
}

func startMetricsServer(metricsAddr string) {
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Metrics server listening on %s", metricsAddr)
	log.Fatal(http.ListenAndServe(metricsAddr, nil))
}

func main() {
	config := &Config{
		Backends:       &sync.Map{},
		BackendIndices: &sync.Map{},
		Cache:          &sync.Map{},
		TLSCertFile:    "cert.pem",
		TLSKeyFile:     "key.pem",
	}

	go collectCPUMetrics()

	go collectProfilingMetrics()

	go func() {
		log.Println("Starting pprof server on :6060")
		log.Println(http.ListenAndServe(":6060", nil)) // Use default pprof routes
	}()

	go startMetricsServer(":9100")

	// Start backend registration API
	startBackendRegistrationAPI(config)

	// Start the proxy server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleHTTPRequest(w, r, config)
	})

	server := &http.Server{
		Addr: ":443",
	}

	log.Printf("Starting L7 reverse proxy on :443")
	if err := server.ListenAndServeTLS(config.TLSCertFile, config.TLSKeyFile); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
