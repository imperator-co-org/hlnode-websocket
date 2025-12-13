package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)

		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
		}
		resp.Result, _ = json.Marshal("0x123")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	req := &Request{
		JSONRPC: "2.0",
		Method:  "test_method",
		Params:  json.RawMessage("[]"),
		ID:      json.RawMessage("1"),
	}

	resp, err := client.Call(context.Background(), req)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}

	var result string
	json.Unmarshal(resp.Result, &result)
	if result != "0x123" {
		t.Errorf("Expected result 0x123, got %s", result)
	}
}

func TestClientCallRaw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","result":"0x456","id":1}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	body := []byte(`{"jsonrpc":"2.0","method":"test","params":[],"id":1}`)

	resp, err := client.CallRaw(context.Background(), body)
	if err != nil {
		t.Fatalf("CallRaw failed: %v", err)
	}

	var response Response
	json.Unmarshal(resp, &response)

	var result string
	json.Unmarshal(response.Result, &result)
	if result != "0x456" {
		t.Errorf("Expected result 0x456, got %s", result)
	}
}

func TestClientGetBlockNumber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method != "eth_blockNumber" {
			t.Errorf("Expected method eth_blockNumber, got %s", req.Method)
		}

		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
		}
		resp.Result, _ = json.Marshal("0x789abc")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	blockNum, err := client.GetBlockNumber(context.Background())
	if err != nil {
		t.Fatalf("GetBlockNumber failed: %v", err)
	}

	if blockNum != "0x789abc" {
		t.Errorf("Expected 0x789abc, got %s", blockNum)
	}
}

func TestClientGetFullBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method != "eth_getBlockByNumber" {
			t.Errorf("Expected method eth_getBlockByNumber, got %s", req.Method)
		}

		block := FullBlockHeader{
			Number:     "0x100",
			Hash:       "0xblockhash",
			ParentHash: "0xparent",
			Timestamp:  "0x12345",
			GasLimit:   "0x1000000",
			GasUsed:    "0x500000",
		}

		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
		}
		resp.Result, _ = json.Marshal(block)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	block, err := client.GetFullBlock(context.Background(), "0x100")
	if err != nil {
		t.Fatalf("GetFullBlock failed: %v", err)
	}

	if block.Number != "0x100" {
		t.Errorf("Expected number 0x100, got %s", block.Number)
	}

	if block.Hash != "0xblockhash" {
		t.Errorf("Expected hash 0xblockhash, got %s", block.Hash)
	}
}

func TestClientGetBlockLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method != "eth_getLogs" {
			t.Errorf("Expected method eth_getLogs, got %s", req.Method)
		}

		logs := []Log{
			{
				Address:     "0xcontract",
				Topics:      []string{"0xtopic1", "0xtopic2"},
				BlockNumber: "0x100",
				LogIndex:    "0x0",
			},
		}

		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
		}
		resp.Result, _ = json.Marshal(logs)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	logs, err := client.GetBlockLogs(context.Background(), "0x100")
	if err != nil {
		t.Fatalf("GetBlockLogs failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	if logs[0].Address != "0xcontract" {
		t.Errorf("Expected address 0xcontract, got %s", logs[0].Address)
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(json.RawMessage("1"), ErrCodeInvalidRequest, "Invalid request")

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}

	if resp.Error == nil {
		t.Fatal("Expected error to be set")
	}

	if resp.Error.Code != ErrCodeInvalidRequest {
		t.Errorf("Expected error code %d, got %d", ErrCodeInvalidRequest, resp.Error.Code)
	}

	if resp.Error.Message != "Invalid request" {
		t.Errorf("Expected message 'Invalid request', got '%s'", resp.Error.Message)
	}
}
