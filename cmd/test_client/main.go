package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/0xAtelerix/example/application"
)

type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  any           `json:"result"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      int           `json:"id"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type EventTransaction struct {
	Event  application.Event `json:"event"`
	TxHash string            `json:"hash"`
}

// RemoteEvent represents the structure of events from the remote API
type RemoteEvent struct {
	ID                string        `json:"id"`
	Name              string        `json:"name"`
	Description       string        `json:"description"`
	Options           []interface{} `json:"options"`
	WinningOption     string        `json:"winningOption"`
	TotalProvers      int           `json:"totalProvers"`
	Participation     int           `json:"participation"`
	ParticipationRate float64       `json:"participationRate"`
	ConsensusRate     float64       `json:"consensusRate"`
	TargetDate        string        `json:"targetDate"`
	ClosedAt          string        `json:"closedAt"`
}

const (
	maxWorkers        = 50 // Number of workers for processing
	maxQueueSize      = 100
	maxRetries        = 3 // Number of retries for RPC calls
	rpcURL            = "http://localhost:8080/rpc"
	maxConcurrentTx   = 50 // Concurrent transaction limit
	batchInterval     = 2  // Seconds between batches
	initialRetryDelay = 1  // Initial retry delay in seconds
	maxRetryDelay     = 2  // Maximum retry delay in seconds
	batchSize         = 50 // Default batch size for processing events
)

// min returns the smaller of two durations
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

type rpcClient struct {
	client      *http.Client
	url         string
	requestID   int64
	rateLimiter chan struct{}
}

func newRPCClient(url string) *rpcClient {
	return &rpcClient{
		client:      &http.Client{Timeout: 10 * time.Second},
		url:         url,
		rateLimiter: make(chan struct{}, maxConcurrentTx),
	}
}

type eventWork struct {
	event application.Event
	index int
}

type processingStats struct {
	processed  int32
	successful int32
	failed     int32
	total      int32
	startTime  time.Time
	mu         sync.Mutex
}

func (s *processingStats) update(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processed++
	if err == nil {
		s.successful++
	} else {
		s.failed++
	}
}

func (s *processingStats) print() {
	s.mu.Lock()
	defer s.mu.Unlock()
	elapsed := time.Since(s.startTime)
	rate := float64(s.processed) / elapsed.Seconds()

	fmt.Printf("\rProgress: %d/%d (%.1f%%) | Success: %d | Failed: %d | Rate: %.1f/s | Elapsed: %s\n",
		s.processed, s.total, float64(s.processed)/float64(s.total)*100,
		s.successful, s.failed, rate, elapsed.Round(time.Second))
}

func main() {
	// Create RPC client with rate limiting
	rpc := newRPCClient(rpcURL)

	// Demonstrate RPC methods first
	fmt.Println("=== Testing RPC Methods ===")
	demonstrateRPCMethods(rpc)

	fmt.Println("\n=== Processing Remote Events ===")
	// Fetch events from remote API
	remoteEvents := fetchRemoteEvents()
	fmt.Printf("Fetched %d events from remote API\n", len(remoteEvents))

	// Initialize processing stats
	stats := &processingStats{
		startTime: time.Now(),
		total:     int32(len(remoteEvents)),
	}

	// Create worker pool
	jobs := make(chan eventWork, maxQueueSize)
	results := make(chan error, len(remoteEvents))

	// Start workers
	for w := 1; w <= maxWorkers; w++ {
		go worker(w, jobs, results, stats)
	}

	// Convert all events first (can be done in parallel)
	events := make([]application.Event, len(remoteEvents))
	var wg sync.WaitGroup
	for i := range remoteEvents {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			events[i] = convertToLocalEvent(remoteEvents[i], int64(i+1))
		}(i)
	}
	wg.Wait()

	// Queue events for processing in batches
	go func() {
		for i := 0; i < len(events); i += batchSize {
			end := i + batchSize
			if end > len(events) {
				end = len(events)
			}

			// Queue current batch
			for j := i; j < end; j++ {
				jobs <- eventWork{event: events[j], index: j}
			}

			// Wait for batch interval before next batch
			if end < len(events) {
				fmt.Printf("\nWaiting %d seconds before next batch...\n", batchInterval)
				time.Sleep(time.Duration(batchInterval) * time.Second)
			}
		}
		close(jobs)
	}()

	// Wait for results and update stats
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			stats.print()
		}
	}()

	for i := 0; i < len(events); i++ {
		<-results
	}

	stats.print()
	fmt.Println("\nProcessing complete!")
}

func fetchRemoteEvents() []RemoteEvent {
	resp, err := http.Get("https://predicted-provers.replit.app/api/blockchain/concluded-events")
	if err != nil {
		panic(fmt.Sprintf("Failed to fetch remote events: %v", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(fmt.Sprintf("Failed to read response body: %v", err))
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(body, &data); err != nil {
		panic(fmt.Sprintf("Failed to parse JSON: %v", err))
	}

	// Extract just the events array
	var events []RemoteEvent
	if err := json.Unmarshal(data["events"], &events); err != nil {
		panic(fmt.Sprintf("Failed to parse events: %v", err))
	}

	return events
}

func convertToLocalEvent(remote RemoteEvent, eventID int64) application.Event {
	// Create options slice
	options := make([]application.EventOption, 0, len(remote.Options))
	var winningOptionID int64
	optionID := int64(1)

	for _, opt := range remote.Options {
		optStr, ok := opt.(string)
		if !ok {
			continue // Skip if not a string
		}
		isWinner := optStr == remote.WinningOption
		if isWinner {
			winningOptionID = optionID
		}
		options = append(options, application.EventOption{
			ID:        optionID,
			Name:      optStr,
			IsWinner:  isWinner,
			VoteCount: 0, // We don't have this information from the remote API
		})
		optionID++
	}

	return application.Event{
		EventID:     eventID,
		EventName:   remote.Name,
		Description: &remote.Description,
		TargetDate:  remote.TargetDate,
		Status:      "Closed", // These are concluded events
		ClosedAt:    remote.ClosedAt,
		Options:     options,
		ConsensusMetrics: application.ConsensusMetrics{
			TotalProvers:       remote.TotalProvers,
			ParticipationCount: remote.Participation,
			ParticipationRate:  remote.ParticipationRate,
			WinningOptionId:    winningOptionID,
			WinningOptionName:  remote.WinningOption,
			ConsensusRate:      remote.ConsensusRate,
		},
	}
}

func worker(id int, jobs <-chan eventWork, results chan<- error, stats *processingStats) {
	for j := range jobs {
		err := sendEventTransaction(j.event)
		stats.update(err)
		results <- err
	}
}

func sendEventTransaction(event application.Event) error {
	client := newRPCClient(rpcURL)

	// Acquire rate limiter slot
	client.rateLimiter <- struct{}{}
	defer func() { <-client.rateLimiter }()

	// Generate deterministic transaction hash based on event ID
	txHash := fmt.Sprintf("0x%064x", event.EventID)

	tx := EventTransaction{
		Event:  event,
		TxHash: txHash,
	}

	// 1. Send Transaction
	fmt.Printf("\nProcessing event: %s (ID: %d)\n", event.EventName, event.EventID)
	sendResult := client.call("sendTransaction", []any{tx})
	if sendResult.Error != nil {
		return fmt.Errorf("error sending transaction: %v", sendResult.Error)
	}
	fmt.Printf("Transaction sent: %v\n", sendResult.Result)

	// 2. Check Transaction Status with retry
	var txStatus string
	for retry := 0; retry < maxRetries; retry++ {
		time.Sleep(time.Duration(retry+1) * time.Second)
		statusResult := client.call("getTransactionStatus", []any{txHash})
		if statusResult.Error != nil {
			fmt.Printf("Error checking status (attempt %d): %v\n", retry+1, statusResult.Error)
			continue
		}
		txStatus = fmt.Sprintf("%v", statusResult.Result)
		fmt.Printf("Transaction status: %s\n", txStatus)
		if txStatus == "Processed" {
			break
		}
	}

	if txStatus != "Processed" {
		return fmt.Errorf("transaction did not process in time")
	}

	return nil
}

func sendRPCRequest(client *http.Client, url string, method string, params []any) *JSONRPCResponse {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		panic(err)
	}

	resp, err := client.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var result JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		panic(err)
	}

	return &result
}

func demonstrateRPCMethods(rpc *rpcClient) {
	// 1. Test getStatus
	fmt.Println("\n1. Testing getStatus:")
	statusResp := rpc.call("getStatus", nil)
	printResponse(statusResp)

	// 2. Test listEvents (empty chain)
	fmt.Println("\n2. Testing listEvents:")
	listResp := rpc.call("listEvents", []any{map[string]any{"offset": 0, "limit": 10}})
	printResponse(listResp)

	// 3. Test getEvent (non-existent)
	fmt.Println("\n3. Testing getEvent (should fail):")
	getResp := rpc.call("getEvent", []any{map[string]any{"eventId": 1}})
	printResponse(getResp)

	fmt.Println("\nStarting event processing...")
}

func printResponse(resp *JSONRPCResponse) {
	if resp.Error != nil {
		fmt.Printf("Error: %v\n", resp.Error)
	} else {
		prettyJSON, _ := json.MarshalIndent(resp.Result, "", "  ")
		fmt.Printf("Success: %s\n", string(prettyJSON))
	}
}

func (c *rpcClient) call(method string, params []any) *JSONRPCResponse {
	c.requestID++
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      int(c.requestID),
	}

	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			time.Sleep(time.Duration(retry) * time.Second)
		}

		reqBody, err := json.Marshal(request)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := c.client.Post(c.url, "application/json", bytes.NewReader(reqBody))
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		var result JSONRPCResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			lastErr = err
			continue
		}

		return &result
	}

	return &JSONRPCResponse{Error: &JSONRPCError{Message: lastErr.Error()}}
}
