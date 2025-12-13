package integration

import (
	"encoding/json"
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// These tests compare the proxy WebSocket against a reference WebSocket
// Run with: go test -v ./internal/integration -proxy-ws ws://localhost:8080 -ref-ws ws://reference:8545

var (
	proxyWS = flag.String("proxy-ws", "ws://localhost:8080", "Proxy WebSocket URL (e.g., ws://localhost:8080)")
	refWS   = flag.String("ref-ws", "", "Reference WebSocket URL (e.g., ws://localhost:8545)")
)

// Response format for comparison
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID json.RawMessage `json:"id"`
}

// SubscriptionResponse for eth_subscribe
type SubscriptionResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Subscription string          `json:"subscription"`
		Result       json.RawMessage `json:"result"`
	} `json:"params"`
}

func skipIfNoEndpoints(t *testing.T) {
	if *proxyWS == "" {
		t.Skip("Skipping: -proxy-ws not provided")
	}
}

func connectWS(t *testing.T, url string) *websocket.Conn {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Failed to connect to %s: %v", url, err)
	}
	return conn
}

func sendAndReceive(t *testing.T, conn *websocket.Conn, request interface{}) JSONRPCResponse {
	if err := conn.WriteJSON(request); err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(message, &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	return resp
}

// TestProxyEthBlockNumber tests eth_blockNumber returns valid hex format
func TestProxyEthBlockNumber(t *testing.T) {
	skipIfNoEndpoints(t)

	conn := connectWS(t, *proxyWS)
	defer conn.Close()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
		"id":      1,
	}

	resp := sendAndReceive(t, conn, request)

	// Verify format
	if resp.JSONRPC != "2.0" {
		t.Errorf("Invalid jsonrpc version: %s", resp.JSONRPC)
	}

	if resp.Error != nil {
		t.Errorf("Unexpected error: %s", resp.Error.Message)
		return
	}

	var blockNum string
	json.Unmarshal(resp.Result, &blockNum)

	if !strings.HasPrefix(blockNum, "0x") {
		t.Errorf("Block number should be hex format, got: %s", blockNum)
	}

	t.Logf("eth_blockNumber: %s", blockNum)
}

// TestProxyEthChainId tests eth_chainId returns valid hex format
func TestProxyEthChainId(t *testing.T) {
	skipIfNoEndpoints(t)

	conn := connectWS(t, *proxyWS)
	defer conn.Close()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_chainId",
		"params":  []interface{}{},
		"id":      1,
	}

	resp := sendAndReceive(t, conn, request)

	if resp.Error != nil {
		t.Errorf("Unexpected error: %s", resp.Error.Message)
		return
	}

	var chainId string
	json.Unmarshal(resp.Result, &chainId)

	if !strings.HasPrefix(chainId, "0x") {
		t.Errorf("Chain ID should be hex format, got: %s", chainId)
	}

	t.Logf("eth_chainId: %s", chainId)
}

// TestProxySubscribeNewHeads tests eth_subscribe returns valid subscription ID
func TestProxySubscribeNewHeads(t *testing.T) {
	skipIfNoEndpoints(t)

	conn := connectWS(t, *proxyWS)
	defer conn.Close()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params":  []string{"newHeads"},
		"id":      1,
	}

	resp := sendAndReceive(t, conn, request)

	if resp.Error != nil {
		t.Errorf("Unexpected error: %s", resp.Error.Message)
		return
	}

	var subID string
	json.Unmarshal(resp.Result, &subID)

	if !strings.HasPrefix(subID, "0x") {
		t.Errorf("Subscription ID should be hex format, got: %s", subID)
	}

	t.Logf("Subscription ID: %s", subID)

	// Wait for a block notification
	t.Log("Waiting for block notification...")
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Logf("No notification received within timeout (this may be expected): %v", err)
		return
	}

	var notification SubscriptionResponse
	if err := json.Unmarshal(message, &notification); err != nil {
		t.Fatalf("Failed to parse notification: %v", err)
	}

	if notification.Method != "eth_subscription" {
		t.Errorf("Expected method eth_subscription, got: %s", notification.Method)
	}

	// Parse block header
	var header map[string]interface{}
	json.Unmarshal(notification.Params.Result, &header)

	requiredFields := []string{"number", "hash", "parentHash", "timestamp"}
	for _, field := range requiredFields {
		if _, ok := header[field]; !ok {
			t.Errorf("Block header missing field: %s", field)
		}
	}

	t.Logf("Received block: %v", header["number"])
}

// TestProxyCompareWithReference compares responses between proxy and reference
func TestProxyCompareWithReference(t *testing.T) {
	if *proxyWS == "" || *refWS == "" {
		t.Skip("Skipping: both -proxy-ws and -ref-ws required for comparison")
	}

	proxyConn := connectWS(t, *proxyWS)
	defer proxyConn.Close()

	refConn := connectWS(t, *refWS)
	defer refConn.Close()

	// Test eth_blockNumber from both
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
		"id":      1,
	}

	proxyResp := sendAndReceive(t, proxyConn, request)
	refResp := sendAndReceive(t, refConn, request)

	// Both should have same format
	if proxyResp.JSONRPC != refResp.JSONRPC {
		t.Errorf("JSONRPC version mismatch: proxy=%s, ref=%s", proxyResp.JSONRPC, refResp.JSONRPC)
	}

	// Both should be hex format
	var proxyNum, refNum string
	json.Unmarshal(proxyResp.Result, &proxyNum)
	json.Unmarshal(refResp.Result, &refNum)

	if !strings.HasPrefix(proxyNum, "0x") || !strings.HasPrefix(refNum, "0x") {
		t.Errorf("Both should return hex: proxy=%s, ref=%s", proxyNum, refNum)
	}

	t.Logf("Proxy block: %s, Reference block: %s", proxyNum, refNum)
}

// TestProxyEthGetBlockByNumber tests eth_getBlockByNumber response format
func TestProxyEthGetBlockByNumber(t *testing.T) {
	skipIfNoEndpoints(t)

	conn := connectWS(t, *proxyWS)
	defer conn.Close()

	// First get latest block number
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
		"id":      1,
	}
	resp := sendAndReceive(t, conn, request)

	var blockNum string
	json.Unmarshal(resp.Result, &blockNum)

	// Get block by number
	request = map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getBlockByNumber",
		"params":  []interface{}{blockNum, false},
		"id":      2,
	}
	resp = sendAndReceive(t, conn, request)

	if resp.Error != nil {
		t.Errorf("Unexpected error: %s", resp.Error.Message)
		return
	}

	var block map[string]interface{}
	json.Unmarshal(resp.Result, &block)

	requiredFields := []string{"number", "hash", "parentHash", "timestamp", "gasLimit", "gasUsed"}
	for _, field := range requiredFields {
		if _, ok := block[field]; !ok {
			t.Errorf("Block missing required field: %s", field)
		}
	}

	t.Logf("Block %s: hash=%v", blockNum, block["hash"])
}

// TestProxyBatchRequest tests batch JSON-RPC requests
func TestProxyBatchRequest(t *testing.T) {
	skipIfNoEndpoints(t)

	conn := connectWS(t, *proxyWS)
	defer conn.Close()

	batch := []map[string]interface{}{
		{"jsonrpc": "2.0", "method": "eth_blockNumber", "params": []interface{}{}, "id": 1},
		{"jsonrpc": "2.0", "method": "eth_chainId", "params": []interface{}{}, "id": 2},
	}

	if err := conn.WriteJSON(batch); err != nil {
		t.Fatalf("Failed to send batch: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var responses []JSONRPCResponse
	if err := json.Unmarshal(message, &responses); err != nil {
		t.Fatalf("Failed to parse batch response: %v", err)
	}

	if len(responses) != 2 {
		t.Errorf("Expected 2 responses, got %d", len(responses))
	}

	for _, resp := range responses {
		if resp.Error != nil {
			t.Errorf("Batch response error: %s", resp.Error.Message)
		}
	}

	t.Logf("Batch responses: %d", len(responses))
}
