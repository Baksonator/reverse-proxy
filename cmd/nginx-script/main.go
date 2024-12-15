package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
)

type Record struct {
	RequestTime   string `json:"request_time"`
	RequestMethod string `json:"request_method"`
}

func main() {
	// Open the file
	file, err := os.Open("C:\\Users\\bbaka\\AppData\\Local\\Docker\\json_access2.log")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	var requestTimes []float64

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record Record
		line := scanner.Text()

		// Parse JSON
		err := json.Unmarshal([]byte(line), &record)
		if err != nil {
			log.Printf("Failed to parse line: %s, error: %v", line, err)
			continue
		}

		// Filter for "POST" request_method
		if record.RequestMethod != "POST" || record.RequestTime >= "60.000" || record.RequestTime == "60.001" || record.RequestTime == "59.999" || record.RequestTime >= "60.003" {
			continue
		}

		// Convert request_time to float64 and append to slice
		rt, err := strconv.ParseFloat(record.RequestTime, 64)
		if err != nil {
			log.Printf("Failed to parse request_time: %s, error: %v", record.RequestTime, err)
			continue
		}
		requestTimes = append(requestTimes, rt)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	// Calculate statistics
	if len(requestTimes) == 0 {
		fmt.Println("No valid request times found.")
		return
	}

	// Average
	average := calculateAverage(requestTimes)

	// Median
	median := calculateMedian(requestTimes)

	// 95th Percentile
	p95 := calculatePercentile(requestTimes, 95)

	p99 := calculatePercentile(requestTimes, 99)

	p999 := calculatePercentile(requestTimes, 99.9)

	// Output the results
	fmt.Printf("Average: %.6f\n", average)
	fmt.Printf("Median: %.6f\n", median)
	fmt.Printf("P95: %.6f\n", p95)
	fmt.Printf("P99: %.6f\n", p99)
	fmt.Printf("P999: %.6f\n", p999)
}

func calculateAverage(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateMedian(values []float64) float64 {
	sort.Float64s(values)
	n := len(values)
	if n%2 == 0 {
		return (values[n/2-1] + values[n/2]) / 2
	}
	return values[n/2]
}

func calculatePercentile(values []float64, percentile float64) float64 {
	sort.Float64s(values)
	index := math.Ceil(float64(percentile)/100*float64(len(values))) - 1
	return values[int(index)]
}
