package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hlnode-websocket/internal/broadcaster"
	"hlnode-websocket/internal/rpc"

	"github.com/gorilla/websocket"
)

// mockRPCServer creates a mock RPC server for testing
func mockRPCServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpc.Request
		json.NewDecoder(r.Body).Decode(&req)

		var resp rpc.Response
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "eth_blockNumber":
			resp.Result, _ = json.Marshal("0x123456")
		case "eth_chainId":
			resp.Result, _ = json.Marshal("0x1")
		case "eth_getBlockByNumber":
			block := rpc.FullBlockHeader{
				Number:     "0x123456",
				Hash:       "0xabc123",
				ParentHash: "0xdef456",
				Timestamp:  "0x12345678",
				GasLimit:   "0x1000000",
				GasUsed:    "0x500000",
			}
			resp.Result, _ = json.Marshal(block)
		default:
			resp.Result, _ = json.Marshal("ok")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// TestWebSocketConnection tests basic WebSocket connection
func TestWebSocketConnection(t *testing.T) {
	mockServer := mockRPCServer()
	defer mockServer.Close()

	rpcClient := rpc.NewClient(mockServer.URL)
	bc := broadcaster.NewBroadcaster()
	go bc.Run()

	wsHandler := NewWebSocketHandler(rpcClient, bc)
	server := httptest.NewServer(wsHandler)
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Connection should succeed
	t.Log("WebSocket connection established successfully")
}

// TestWebSocketRPCCall tests JSON-RPC calls over WebSocket
func TestWebSocketRPCCall(t *testing.T) {
	mockServer := mockRPCServer()
	defer mockServer.Close()

	rpcClient := rpc.NewClient(mockServer.URL)
	bc := broadcaster.NewBroadcaster()
	go bc.Run()

	wsHandler := NewWebSocketHandler(rpcClient, bc)
	server := httptest.NewServer(wsHandler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send eth_blockNumber request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
		"id":      1,
	}
	if err := conn.WriteJSON(request); err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Read response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Parse response
	var resp rpc.Response
	if err := json.Unmarshal(message, &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response format
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}

	var blockNum string
	json.Unmarshal(resp.ID, &blockNum)

	var result string
	json.Unmarshal(resp.Result, &result)
	if result != "0x123456" {
		t.Errorf("Expected block number 0x123456, got %s", result)
	}

	t.Logf("eth_blockNumber response: %s", result)
}

// TestWebSocketSubscribe tests eth_subscribe for newHeads
func TestWebSocketSubscribe(t *testing.T) {
	mockServer := mockRPCServer()
	defer mockServer.Close()

	rpcClient := rpc.NewClient(mockServer.URL)
	bc := broadcaster.NewBroadcaster()
	go bc.Run()

	wsHandler := NewWebSocketHandler(rpcClient, bc)
	server := httptest.NewServer(wsHandler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Subscribe to newHeads
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params":  []string{"newHeads"},
		"id":      1,
	}
	if err := conn.WriteJSON(request); err != nil {
		t.Fatalf("Failed to send subscribe request: %v", err)
	}

	// Read subscription response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var resp rpc.Response
	if err := json.Unmarshal(message, &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify we got a subscription ID
	var subID string
	json.Unmarshal(resp.Result, &subID)
	if !strings.HasPrefix(subID, "0x") {
		t.Errorf("Expected subscription ID starting with 0x, got %s", subID)
	}

	t.Logf("Subscription ID: %s", subID)
}

// TestWebSocketUnsubscribe tests eth_unsubscribe
func TestWebSocketUnsubscribe(t *testing.T) {
	mockServer := mockRPCServer()
	defer mockServer.Close()

	rpcClient := rpc.NewClient(mockServer.URL)
	bc := broadcaster.NewBroadcaster()
	go bc.Run()

	wsHandler := NewWebSocketHandler(rpcClient, bc)
	server := httptest.NewServer(wsHandler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// First subscribe
	subRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params":  []string{"newHeads"},
		"id":      1,
	}
	conn.WriteJSON(subRequest)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, _ := conn.ReadMessage()

	var subResp rpc.Response
	json.Unmarshal(message, &subResp)
	var subID string
	json.Unmarshal(subResp.Result, &subID)

	// Now unsubscribe
	unsubRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_unsubscribe",
		"params":  []string{subID},
		"id":      2,
	}
	if err := conn.WriteJSON(unsubRequest); err != nil {
		t.Fatalf("Failed to send unsubscribe: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read unsubscribe response: %v", err)
	}

	var resp rpc.Response
	json.Unmarshal(message, &resp)

	var success bool
	json.Unmarshal(resp.Result, &success)
	if !success {
		t.Error("Expected unsubscribe to return true")
	}

	t.Log("Unsubscribe successful")
}

// TestWebSocketInvalidRequest tests error handling for invalid requests
func TestWebSocketInvalidRequest(t *testing.T) {
	mockServer := mockRPCServer()
	defer mockServer.Close()

	rpcClient := rpc.NewClient(mockServer.URL)
	bc := broadcaster.NewBroadcaster()
	go bc.Run()

	wsHandler := NewWebSocketHandler(rpcClient, bc)
	server := httptest.NewServer(wsHandler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send request with wrong jsonrpc version
	request := map[string]interface{}{
		"jsonrpc": "1.0", // Wrong version
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
		"id":      1,
	}
	conn.WriteJSON(request)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, _ := conn.ReadMessage()

	var resp rpc.Response
	json.Unmarshal(message, &resp)

	if resp.Error == nil {
		t.Error("Expected error for invalid jsonrpc version")
	} else if resp.Error.Code != rpc.ErrCodeInvalidRequest {
		t.Errorf("Expected error code %d, got %d", rpc.ErrCodeInvalidRequest, resp.Error.Code)
	}

	t.Logf("Got expected error: %s", resp.Error.Message)
}

// TestWebSocketSubscriptionNotification tests that subscribers receive notifications
func TestWebSocketSubscriptionNotification(t *testing.T) {
	mockServer := mockRPCServer()
	defer mockServer.Close()

	rpcClient := rpc.NewClient(mockServer.URL)
	bc := broadcaster.NewBroadcaster()
	go bc.Run()

	wsHandler := NewWebSocketHandler(rpcClient, bc)
	server := httptest.NewServer(wsHandler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Subscribe to newHeads
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params":  []string{"newHeads"},
		"id":      1,
	}
	conn.WriteJSON(request)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	conn.ReadMessage() // Read subscription response

	// Simulate a new block broadcast
	testHeader := &rpc.FullBlockHeader{
		Number:     "0x999",
		Hash:       "0xtest",
		ParentHash: "0xparent",
		Timestamp:  "0x12345",
		GasLimit:   "0x1000",
		GasUsed:    "0x500",
	}

	// Give time for client registration
	time.Sleep(100 * time.Millisecond)

	bc.BroadcastNewHead(testHeader)

	// Read notification
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read notification: %v", err)
	}

	// Parse notification
	var notification map[string]interface{}
	json.Unmarshal(message, &notification)

	if notification["method"] != "eth_subscription" {
		t.Errorf("Expected method eth_subscription, got %v", notification["method"])
	}

	params := notification["params"].(map[string]interface{})
	result := params["result"].(map[string]interface{})

	if result["number"] != "0x999" {
		t.Errorf("Expected block number 0x999, got %v", result["number"])
	}

	t.Logf("Received notification for block: %v", result["number"])
}

// TestWebSocketLogsSubscription tests logs subscription with filter
func TestWebSocketLogsSubscription(t *testing.T) {
	mockServer := mockRPCServer()
	defer mockServer.Close()

	rpcClient := rpc.NewClient(mockServer.URL)
	bc := broadcaster.NewBroadcaster()
	go bc.Run()

	wsHandler := NewWebSocketHandler(rpcClient, bc)
	server := httptest.NewServer(wsHandler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Subscribe to logs with filter
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params": []interface{}{
			"logs",
			map[string]interface{}{
				"address": []string{"0x1234567890abcdef"},
			},
		},
		"id": 1,
	}
	conn.WriteJSON(request)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, _ := conn.ReadMessage()

	var resp rpc.Response
	json.Unmarshal(message, &resp)

	var subID string
	json.Unmarshal(resp.Result, &subID)
	if !strings.HasPrefix(subID, "0x") {
		t.Errorf("Expected subscription ID starting with 0x, got %s", subID)
	}

	t.Logf("Logs subscription ID: %s", subID)
}
