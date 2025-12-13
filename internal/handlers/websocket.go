package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"hlnode-proxy/internal/broadcaster"
	"hlnode-proxy/internal/logger"
	"hlnode-proxy/internal/metrics"
	"hlnode-proxy/internal/rpc"
	"hlnode-proxy/internal/subscription"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	HandshakeTimeout: 10 * time.Second,
}

// WebSocketHandler handles WebSocket connections (reth-compatible)
type WebSocketHandler struct {
	client      *rpc.Client
	broadcaster *broadcaster.Broadcaster
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(client *rpc.Client, bc *broadcaster.Broadcaster) *WebSocketHandler {
	return &WebSocketHandler{
		client:      client,
		broadcaster: bc,
	}
}

// ServeHTTP upgrades the connection to WebSocket and handles messages
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade connection: %v", err)
		return
	}

	conn.SetReadLimit(1024 * 1024)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	client := broadcaster.NewClient(conn, r)
	h.broadcaster.Register(client)

	go client.WritePump()

	defer func() {
		client.Close()
		h.broadcaster.Unregister(client)
		conn.Close()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warn("WebSocket error: %v", err)
			}
			break
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		client.IncrementRecv()

		go h.handleMessage(client, message)
	}
}

// handleMessage processes an incoming WebSocket message
func (h *WebSocketHandler) handleMessage(client *broadcaster.Client, message []byte) {
	if len(message) > 0 && message[0] == '[' {
		h.handleBatchMessage(client, message)
		return
	}

	var req rpc.Request
	if err := json.Unmarshal(message, &req); err != nil {
		h.sendError(client, nil, rpc.ErrCodeParseError, "Failed to parse JSON-RPC request")
		return
	}

	if req.JSONRPC != "2.0" {
		h.sendError(client, req.ID, rpc.ErrCodeInvalidRequest, "Invalid JSON-RPC version")
		return
	}

	if req.Method == "" {
		h.sendError(client, req.ID, rpc.ErrCodeInvalidRequest, "Method is required")
		return
	}

	// Track WebSocket RPC request
	metrics.WSRPCRequestsTotal.WithLabelValues(req.Method).Inc()

	switch req.Method {
	case "eth_subscribe":
		h.handleSubscribe(client, &req)
		return
	case "eth_unsubscribe":
		h.handleUnsubscribe(client, &req)
		return
	}

	resp, err := h.client.Call(context.Background(), &req)
	if err != nil {
		logger.Error("Failed to forward request: %v", err)
		h.sendError(client, req.ID, rpc.ErrCodeInternalError, "Failed to forward request")
		return
	}

	data, _ := json.Marshal(resp)
	select {
	case client.Send() <- data:
	default:
		logger.Warn("Client send buffer full")
	}
}

// handleBatchMessage processes a batch of requests
func (h *WebSocketHandler) handleBatchMessage(client *broadcaster.Client, message []byte) {
	// Parse to count requests
	var reqs []rpc.Request
	if err := json.Unmarshal(message, &reqs); err == nil {
		for _, req := range reqs {
			if req.Method != "" {
				metrics.WSRPCRequestsTotal.WithLabelValues(req.Method).Inc()
			}
		}
	}

	resp, err := h.client.CallRaw(context.Background(), message)
	if err != nil {
		logger.Error("Failed to forward batch request: %v", err)
		return
	}

	select {
	case client.Send() <- resp:
	default:
		logger.Warn("Client send buffer full")
	}
}

// handleSubscribe handles eth_subscribe requests
func (h *WebSocketHandler) handleSubscribe(client *broadcaster.Client, req *rpc.Request) {
	var params []json.RawMessage
	if err := json.Unmarshal(req.Params, &params); err != nil || len(params) == 0 {
		h.sendError(client, req.ID, rpc.ErrCodeInvalidParams, "Invalid subscription parameters")
		return
	}

	var subType string
	if err := json.Unmarshal(params[0], &subType); err != nil {
		h.sendError(client, req.ID, rpc.ErrCodeInvalidParams, "Subscription type must be a string")
		return
	}

	var subscriptionType subscription.SubscriptionType
	var filterParams json.RawMessage

	switch subType {
	case "newHeads":
		subscriptionType = subscription.SubTypeNewHeads
	case "logs":
		subscriptionType = subscription.SubTypeLogs
		if len(params) > 1 {
			filterParams = params[1]
		}
	case "newPendingTransactions":
		subscriptionType = subscription.SubTypeNewPendingTransactions
	case "syncing":
		subscriptionType = subscription.SubTypeSyncing
	default:
		h.sendError(client, req.ID, rpc.ErrCodeInvalidParams,
			"Unsupported subscription type. Supported: newHeads, logs, newPendingTransactions, syncing")
		return
	}

	subManager := h.broadcaster.SubscriptionManager()
	subID, err := subManager.Subscribe(client.ID, subscriptionType, filterParams)
	if err != nil {
		h.sendError(client, req.ID, rpc.ErrCodeInternalError, "Failed to create subscription")
		return
	}

	resp := &rpc.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}
	resp.Result, _ = json.Marshal(subID)

	data, _ := json.Marshal(resp)
	select {
	case client.Send() <- data:
	default:
	}
}

// handleUnsubscribe handles eth_unsubscribe requests
func (h *WebSocketHandler) handleUnsubscribe(client *broadcaster.Client, req *rpc.Request) {
	var params []string
	if err := json.Unmarshal(req.Params, &params); err != nil || len(params) == 0 {
		h.sendError(client, req.ID, rpc.ErrCodeInvalidParams, "Invalid unsubscribe parameters")
		return
	}

	subID := params[0]
	subManager := h.broadcaster.SubscriptionManager()
	success := subManager.Unsubscribe(client.ID, subID)

	resp := &rpc.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}
	resp.Result, _ = json.Marshal(success)

	data, _ := json.Marshal(resp)
	select {
	case client.Send() <- data:
	default:
	}
}

// sendError sends a JSON-RPC error response to a WebSocket client
func (h *WebSocketHandler) sendError(client *broadcaster.Client, id json.RawMessage, code int, message string) {
	resp := rpc.NewErrorResponse(id, code, message)
	data, _ := json.Marshal(resp)
	select {
	case client.Send() <- data:
	default:
	}
}
