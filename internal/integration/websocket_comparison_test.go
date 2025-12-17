package integration

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// These tests compare the local WebSocket against a reference WebSocket
// Run with: WS_COMPARE=ws://volcano-compute:8545 go test -v ./internal/integration

// WS_LOCAL is always localhost:8080
const wsLocal = "ws://localhost:8080"

func getWSCompare() string {
	return os.Getenv("WS_COMPARE")
}

func skipIfNoWSCompare(t *testing.T) {
	if getWSCompare() == "" {
		t.Skip("Skipping: WS_COMPARE env not set")
	}
}

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

// TestWebSocketEthBlockNumber tests eth_blockNumber returns valid hex format
func TestWebSocketEthBlockNumber(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
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

// TestWebSocketEthChainId tests eth_chainId returns valid hex format
func TestWebSocketEthChainId(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
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

// TestWebSocketSubscribeNewHeads tests eth_subscribe returns valid subscription ID
func TestWebSocketSubscribeNewHeads(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
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

// TestWebSocketCompareWithReference compares responses between local and reference WebSocket
func TestWebSocketCompareWithReference(t *testing.T) {
	skipIfNoWSCompare(t)

	localConn := connectWS(t, wsLocal)
	defer localConn.Close()

	refConn := connectWS(t, getWSCompare())
	defer refConn.Close()

	// Test eth_blockNumber from both
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
		"id":      1,
	}

	localResp := sendAndReceive(t, localConn, request)
	refResp := sendAndReceive(t, refConn, request)

	// Both should have same format
	if localResp.JSONRPC != refResp.JSONRPC {
		t.Errorf("JSONRPC version mismatch: local=%s, ref=%s", localResp.JSONRPC, refResp.JSONRPC)
	}

	// Both should be hex format
	var localNum, refNum string
	json.Unmarshal(localResp.Result, &localNum)
	json.Unmarshal(refResp.Result, &refNum)

	if !strings.HasPrefix(localNum, "0x") || !strings.HasPrefix(refNum, "0x") {
		t.Errorf("Both should return hex: local=%s, ref=%s", localNum, refNum)
	}

	t.Logf("Local block: %s, Reference block: %s", localNum, refNum)
}

// TestWebSocketEthGetBlockByNumber tests eth_getBlockByNumber response format
func TestWebSocketEthGetBlockByNumber(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
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

// TestWebSocketBatchRequest tests batch JSON-RPC requests
func TestWebSocketBatchRequest(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
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

// TestWebSocketSubscribeLogs tests logs subscription with address filter
func TestWebSocketSubscribeLogs(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
	defer conn.Close()

	// Subscribe to logs with single address filter
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params": []interface{}{
			"logs",
			map[string]interface{}{
				"address": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			},
		},
		"id": 1,
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

	t.Logf("Logs subscription ID: %s", subID)

	// Unsubscribe
	unsubRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_unsubscribe",
		"params":  []string{subID},
		"id":      2,
	}
	unsubResp := sendAndReceive(t, conn, unsubRequest)

	var success bool
	json.Unmarshal(unsubResp.Result, &success)
	if !success {
		t.Error("Expected unsubscribe to return true")
	}
}

// TestWebSocketSubscribeLogsMultipleAddresses tests logs subscription with multiple address filter
func TestWebSocketSubscribeLogsMultipleAddresses(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
	defer conn.Close()

	// Subscribe to logs with multiple addresses (as per README example)
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params": []interface{}{
			"logs",
			map[string]interface{}{
				"address": []string{
					"0xf24090f1895cee4033103e670cc58edc28294841",
					"0xdAC17F958D2ee523a2206206994597C13D831ec7",
				},
			},
		},
		"id": 1,
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

	t.Logf("Multiple addresses logs subscription ID: %s", subID)
}

// TestWebSocketSubscribeLogsWithTopics tests logs subscription with topic filter
func TestWebSocketSubscribeLogsWithTopics(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
	defer conn.Close()

	// Subscribe to logs with topic filter (Transfer events as per README)
	transferTopic := "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params": []interface{}{
			"logs",
			map[string]interface{}{
				"address": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
				"topics":  []string{transferTopic},
			},
		},
		"id": 1,
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

	t.Logf("Logs with topics subscription ID: %s", subID)
}

// TestWebSocketSubscribeGasPrice tests gasPrice subscription (Hyperliquid custom)
func TestWebSocketSubscribeGasPrice(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
	defer conn.Close()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params":  []string{"gasPrice"},
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

	t.Logf("gasPrice subscription ID: %s", subID)

	// Wait for a notification (gas price updates periodically)
	t.Log("Waiting for gasPrice notification...")
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
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

	// Parse gas price info
	var gasPriceInfo map[string]interface{}
	json.Unmarshal(notification.Params.Result, &gasPriceInfo)

	if _, ok := gasPriceInfo["gasPrice"]; !ok {
		t.Error("gasPrice notification missing 'gasPrice' field")
	}

	t.Logf("Received gasPrice notification: %v", gasPriceInfo["gasPrice"])
}

// TestWebSocketSubscribeBlockReceipts tests blockReceipts subscription (Hyperliquid custom)
func TestWebSocketSubscribeBlockReceipts(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
	defer conn.Close()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params":  []string{"blockReceipts"},
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

	t.Logf("blockReceipts subscription ID: %s", subID)

	// Wait for a notification
	t.Log("Waiting for blockReceipts notification...")
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

	// Parse block receipts
	var receiptsInfo map[string]interface{}
	json.Unmarshal(notification.Params.Result, &receiptsInfo)

	requiredFields := []string{"blockNumber", "blockHash", "receipts"}
	for _, field := range requiredFields {
		if _, ok := receiptsInfo[field]; !ok {
			t.Errorf("blockReceipts notification missing '%s' field", field)
		}
	}

	t.Logf("Received blockReceipts notification for block: %v", receiptsInfo["blockNumber"])
}

// TestWebSocketSubscribeSyncing tests syncing subscription (Hyperliquid custom)
func TestWebSocketSubscribeSyncing(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
	defer conn.Close()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params":  []string{"syncing"},
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

	t.Logf("syncing subscription ID: %s", subID)

	// Wait for a notification (syncing checks periodically)
	t.Log("Waiting for syncing notification...")
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
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

	// Syncing can return true/false or an object with syncing status
	t.Logf("Received syncing notification: %s", string(notification.Params.Result))
}

// TestWebSocketUnsubscribe tests eth_unsubscribe
func TestWebSocketUnsubscribe(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
	defer conn.Close()

	// First subscribe to newHeads
	subRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params":  []string{"newHeads"},
		"id":      1,
	}
	subResp := sendAndReceive(t, conn, subRequest)

	var subID string
	json.Unmarshal(subResp.Result, &subID)

	t.Logf("Subscribed with ID: %s", subID)

	// Now unsubscribe
	unsubRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_unsubscribe",
		"params":  []string{subID},
		"id":      2,
	}
	unsubResp := sendAndReceive(t, conn, unsubRequest)

	if unsubResp.Error != nil {
		t.Errorf("Unexpected error: %s", unsubResp.Error.Message)
		return
	}

	var success bool
	json.Unmarshal(unsubResp.Result, &success)

	if !success {
		t.Error("Expected unsubscribe to return true")
	}

	t.Log("Unsubscribe successful")

	// Try to unsubscribe again (should return false)
	unsubResp2 := sendAndReceive(t, conn, unsubRequest)
	var success2 bool
	json.Unmarshal(unsubResp2.Result, &success2)

	if success2 {
		t.Error("Expected second unsubscribe to return false")
	}

	t.Log("Second unsubscribe correctly returned false")
}

// TestWebSocketInvalidSubscription tests error for invalid subscription type
func TestWebSocketInvalidSubscription(t *testing.T) {
	skipIfNoWSCompare(t)

	conn := connectWS(t, wsLocal)
	defer conn.Close()

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params":  []string{"invalidType"},
		"id":      1,
	}

	resp := sendAndReceive(t, conn, request)

	if resp.Error == nil {
		t.Error("Expected error for invalid subscription type")
		return
	}

	t.Logf("Got expected error: %s", resp.Error.Message)
}

// TestWebSocketCompareLogsWithTopicFilter compares logs subscription responses between
// local and reference WebSocket when using topic-only filter (no address)
func TestWebSocketCompareLogsWithTopicFilter(t *testing.T) {
	skipIfNoWSCompare(t)

	localConn := connectWS(t, wsLocal)
	defer localConn.Close()

	refConn := connectWS(t, getWSCompare())
	defer refConn.Close()

	// Subscribe to logs with topic filter only (Transfer event signature)
	// This is the exact example from the README
	transferTopic := "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_subscribe",
		"params": []interface{}{
			"logs",
			map[string]interface{}{
				"topics": []string{transferTopic},
			},
		},
		"id": 1,
	}

	// Subscribe on local
	localResp := sendAndReceive(t, localConn, request)
	if localResp.Error != nil {
		t.Errorf("Local subscription error: %s", localResp.Error.Message)
		return
	}

	var localSubID string
	json.Unmarshal(localResp.Result, &localSubID)
	t.Logf("Local subscription ID: %s", localSubID)

	// Subscribe on reference
	refResp := sendAndReceive(t, refConn, request)
	if refResp.Error != nil {
		t.Errorf("Reference subscription error: %s", refResp.Error.Message)
		return
	}

	var refSubID string
	json.Unmarshal(refResp.Result, &refSubID)
	t.Logf("Reference subscription ID: %s", refSubID)

	// Both should return hex subscription IDs
	if !strings.HasPrefix(localSubID, "0x") || !strings.HasPrefix(refSubID, "0x") {
		t.Errorf("Both IDs should be hex: local=%s, ref=%s", localSubID, refSubID)
	}

	// Wait for log notifications from both and compare
	t.Log("Waiting for log notifications from both WebSockets...")

	// Create channels to receive notifications
	type logNotification struct {
		subscription string
		log          map[string]interface{}
		raw          string
		err          error
	}

	localChan := make(chan logNotification, 1)
	refChan := make(chan logNotification, 1)

	// Read from local
	go func() {
		localConn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, message, err := localConn.ReadMessage()
		if err != nil {
			localChan <- logNotification{err: err}
			return
		}
		var notification SubscriptionResponse
		if err := json.Unmarshal(message, &notification); err != nil {
			localChan <- logNotification{err: err}
			return
		}
		var logData map[string]interface{}
		json.Unmarshal(notification.Params.Result, &logData)
		localChan <- logNotification{
			subscription: notification.Params.Subscription,
			log:          logData,
			raw:          string(notification.Params.Result),
		}
	}()

	// Read from reference
	go func() {
		refConn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, message, err := refConn.ReadMessage()
		if err != nil {
			refChan <- logNotification{err: err}
			return
		}
		var notification SubscriptionResponse
		if err := json.Unmarshal(message, &notification); err != nil {
			refChan <- logNotification{err: err}
			return
		}
		var logData map[string]interface{}
		json.Unmarshal(notification.Params.Result, &logData)
		refChan <- logNotification{
			subscription: notification.Params.Subscription,
			log:          logData,
			raw:          string(notification.Params.Result),
		}
	}()

	// Wait for both notifications with timeout
	var localLog, refLog logNotification
	timeout := time.After(65 * time.Second)

	for i := 0; i < 2; i++ {
		select {
		case localLog = <-localChan:
			if localLog.err != nil {
				t.Logf("Local notification error (may be expected): %v", localLog.err)
			} else {
				t.Logf("Received local log notification")
			}
		case refLog = <-refChan:
			if refLog.err != nil {
				t.Logf("Reference notification error (may be expected): %v", refLog.err)
			} else {
				t.Logf("Received reference log notification")
			}
		case <-timeout:
			t.Logf("Timeout waiting for notifications (may be expected if no Transfer events)")
			return
		}
	}

	// If both received notifications, compare the log structure
	if localLog.err == nil && refLog.err == nil {
		t.Log("Comparing log notification structures...")

		// Check both have required fields
		requiredFields := []string{"address", "topics", "data", "blockNumber", "transactionHash", "logIndex"}
		for _, field := range requiredFields {
			_, localHas := localLog.log[field]
			_, refHas := refLog.log[field]

			if localHas != refHas {
				t.Errorf("Field '%s' presence mismatch: local=%v, ref=%v", field, localHas, refHas)
			}
		}

		// Log both responses for comparison
		t.Logf("Local log: blockNumber=%v, address=%v, logIndex=%v",
			localLog.log["blockNumber"], localLog.log["address"], localLog.log["logIndex"])
		t.Logf("Reference log: blockNumber=%v, address=%v, logIndex=%v",
			refLog.log["blockNumber"], refLog.log["address"], refLog.log["logIndex"])

		// Check topics match the filter (first topic should be Transfer signature)
		if localTopics, ok := localLog.log["topics"].([]interface{}); ok && len(localTopics) > 0 {
			if localTopics[0] != transferTopic {
				t.Errorf("Local log topic[0] mismatch: expected %s, got %v", transferTopic, localTopics[0])
			}
		}

		if refTopics, ok := refLog.log["topics"].([]interface{}); ok && len(refTopics) > 0 {
			if refTopics[0] != transferTopic {
				t.Errorf("Reference log topic[0] mismatch: expected %s, got %v", transferTopic, refTopics[0])
			}
		}

		t.Log("Both WebSockets returned correctly structured log notifications!")
	}
}
