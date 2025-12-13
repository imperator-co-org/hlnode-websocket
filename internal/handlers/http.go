package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"hlnode-proxy/internal/logger"
	"hlnode-proxy/internal/metrics"
	"hlnode-proxy/internal/rpc"
)

// HTTPHandler handles JSON-RPC requests over HTTP
type HTTPHandler struct {
	client *rpc.Client
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(client *rpc.Client) *HTTPHandler {
	return &HTTPHandler{
		client: client,
	}
}

// ServeHTTP handles incoming HTTP requests
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		metrics.RPCErrorsTotal.WithLabelValues("read_error").Inc()
		h.writeError(w, nil, rpc.ErrCodeParseError, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	// Handle batch requests
	if len(body) > 0 && body[0] == '[' {
		h.handleBatch(w, r, body, start)
		return
	}

	var req rpc.Request
	if err := json.Unmarshal(body, &req); err != nil {
		metrics.RPCErrorsTotal.WithLabelValues("parse_error").Inc()
		h.writeError(w, nil, rpc.ErrCodeParseError, "Failed to parse JSON-RPC request")
		return
	}

	if req.JSONRPC != "2.0" {
		metrics.RPCErrorsTotal.WithLabelValues("invalid_version").Inc()
		h.writeError(w, req.ID, rpc.ErrCodeInvalidRequest, "Invalid JSON-RPC version")
		return
	}

	if req.Method == "" {
		metrics.RPCErrorsTotal.WithLabelValues("missing_method").Inc()
		h.writeError(w, req.ID, rpc.ErrCodeInvalidRequest, "Method is required")
		return
	}

	// Track request metrics
	metrics.RPCRequestsTotal.WithLabelValues(req.Method).Inc()

	resp, err := h.client.CallRaw(r.Context(), body)
	if err != nil {
		metrics.RPCErrorsTotal.WithLabelValues("upstream_error").Inc()
		logger.Error("Failed to forward request: %v", err)
		h.writeError(w, req.ID, rpc.ErrCodeInternalError, "Failed to forward request to upstream")
		return
	}

	// Track duration
	duration := time.Since(start).Seconds()
	metrics.RPCRequestDuration.WithLabelValues(req.Method).Observe(duration)

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// handleBatch handles batch JSON-RPC requests
func (h *HTTPHandler) handleBatch(w http.ResponseWriter, r *http.Request, body []byte, start time.Time) {
	// Parse to count requests
	var reqs []rpc.Request
	if err := json.Unmarshal(body, &reqs); err == nil {
		for _, req := range reqs {
			if req.Method != "" {
				metrics.RPCRequestsTotal.WithLabelValues(req.Method).Inc()
			}
		}
	}

	resp, err := h.client.CallRaw(r.Context(), body)
	if err != nil {
		metrics.RPCErrorsTotal.WithLabelValues("batch_upstream_error").Inc()
		logger.Error("Failed to forward batch request: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode([]rpc.Response{
			*rpc.NewErrorResponse(nil, rpc.ErrCodeInternalError, "Failed to forward batch request"),
		})
		return
	}

	// Track batch duration
	duration := time.Since(start).Seconds()
	metrics.RPCRequestDuration.WithLabelValues("batch").Observe(duration)

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// writeError writes a JSON-RPC error response
func (h *HTTPHandler) writeError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rpc.NewErrorResponse(id, code, message))
}
