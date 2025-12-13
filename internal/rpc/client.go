package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for making upstream RPC calls
type Client struct {
	httpClient *http.Client
	rpcURL     string
}

// NewClient creates a new RPC client
func NewClient(rpcURL string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rpcURL: rpcURL,
	}
}

// Call makes a JSON-RPC call to the upstream server
func (c *Client) Call(ctx context.Context, req *Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp Response
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &rpcResp, nil
}

// CallRaw forwards raw JSON bytes and returns raw response bytes
func (c *Client) CallRaw(ctx context.Context, body []byte) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// GetBlockNumber fetches the latest block number
func (c *Client) GetBlockNumber(ctx context.Context) (string, error) {
	req := &Request{
		JSONRPC: "2.0",
		Method:  "eth_blockNumber",
		Params:  json.RawMessage("[]"),
		ID:      json.RawMessage("1"),
	}

	resp, err := c.Call(ctx, req)
	if err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("RPC error: %s", resp.Error.Message)
	}

	var blockNum string
	if err := json.Unmarshal(resp.Result, &blockNum); err != nil {
		return "", fmt.Errorf("failed to unmarshal block number: %w", err)
	}

	return blockNum, nil
}

// GetFullBlock fetches a full block header for newHeads subscription
func (c *Client) GetFullBlock(ctx context.Context, blockNum string) (*FullBlockHeader, error) {
	params, _ := json.Marshal([]interface{}{blockNum, false})
	req := &Request{
		JSONRPC: "2.0",
		Method:  "eth_getBlockByNumber",
		Params:  params,
		ID:      json.RawMessage("1"),
	}

	resp, err := c.Call(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", resp.Error.Message)
	}

	if resp.Result == nil || string(resp.Result) == "null" {
		return nil, nil
	}

	var header FullBlockHeader
	if err := json.Unmarshal(resp.Result, &header); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &header, nil
}

// GetBlockLogs fetches logs for a specific block
func (c *Client) GetBlockLogs(ctx context.Context, blockNum string) ([]Log, error) {
	filter := map[string]interface{}{
		"fromBlock": blockNum,
		"toBlock":   blockNum,
	}
	params, _ := json.Marshal([]interface{}{filter})
	req := &Request{
		JSONRPC: "2.0",
		Method:  "eth_getLogs",
		Params:  params,
		ID:      json.RawMessage("1"),
	}

	resp, err := c.Call(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", resp.Error.Message)
	}

	if resp.Result == nil || string(resp.Result) == "null" {
		return nil, nil
	}

	var logs []Log
	if err := json.Unmarshal(resp.Result, &logs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	return logs, nil
}
