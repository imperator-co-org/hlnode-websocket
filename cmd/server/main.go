package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hlnode-websocket/internal/broadcaster"
	"hlnode-websocket/internal/config"
	"hlnode-websocket/internal/handlers"
	"hlnode-websocket/internal/logger"
	"hlnode-websocket/internal/metrics"
	"hlnode-websocket/internal/rpc"
	"hlnode-websocket/internal/subscription"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()

	logger.Info("Starting hlnode-websocket")
	logger.Info("Upstream RPC: %s", cfg.RPCURL)
	logger.Info("HTTP Port: %d", cfg.ProxyPort)
	logger.Info("Poll Interval: %v", cfg.PollInterval)

	rpcClient := rpc.NewClient(cfg.RPCURL)

	bc := broadcaster.NewBroadcaster()
	go bc.Run()

	wsHandler := handlers.NewWebSocketHandler(rpcClient, bc)

	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "WebSocket connection required"}`))
			return
		}
		wsHandler.ServeHTTP(w, r)
	})

	// Prometheus metrics
	mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "ok",
			"activeClients": bc.GetStats().ActiveClients,
		})
	})

	// List active connections
	mux.HandleFunc("/connections", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stats":   bc.GetStats(),
			"clients": bc.GetAllClientsInfo(),
		})
	})

	// Enhanced stats with all metrics
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		bcStats := bc.GetStats()
		subMgr := bc.SubscriptionManager()

		response := map[string]interface{}{
			"websocket": map[string]interface{}{
				"activeConnections":   bcStats.ActiveClients,
				"totalConnections":    bcStats.TotalConnections,
				"totalDisconnections": bcStats.TotalDisconnections,
			},
			"subscriptions": map[string]int{
				"newHeads":      len(subMgr.GetSubscriptionsByType(subscription.SubTypeNewHeads)),
				"logs":          len(subMgr.GetSubscriptionsByType(subscription.SubTypeLogs)),
				"gasPrice":      len(subMgr.GetSubscriptionsByType(subscription.SubTypeGasPrice)),
				"blockReceipts": len(subMgr.GetSubscriptionsByType(subscription.SubTypeBlockReceipts)),
				"syncing":       len(subMgr.GetSubscriptionsByType(subscription.SubTypeSyncing)),
			},
		}

		json.NewEncoder(w).Encode(response)
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.ProxyPort),
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go pollBlocks(rpcClient, bc, cfg)
	go pollSyncing(rpcClient, bc, cfg)

	go func() {
		logger.Info("Endpoints: / (WebSocket), /metrics, /health, /connections, /stats")
		logger.Info("Subscriptions: newHeads, logs, gasPrice, blockReceipts, syncing")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	logger.Info("Stopped")
}

func pollBlocks(client *rpc.Client, bc *broadcaster.Broadcaster, cfg *config.Config) {
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	var lastBlockNum string
	var lastGasPrice string
	ctx := context.Background()

	for range ticker.C {
		blockNum, err := client.GetBlockNumber(ctx)
		if err != nil {
			logger.Error("Failed to fetch block number: %v", err)
			metrics.UpstreamErrorsTotal.Inc()
			continue
		}

		metrics.UpstreamRequestsTotal.Inc()

		// Broadcast gas price if changed (check every poll, not just on new block)
		subMgr := bc.SubscriptionManager()
		if len(subMgr.GetSubscriptionsByType(subscription.SubTypeGasPrice)) > 0 {
			gasPrice, err := client.GetGasPrice(ctx)
			if err == nil {
				metrics.UpstreamRequestsTotal.Inc()
				if gasPrice != lastGasPrice {
					bigBlockGasPrice, _ := client.GetBigBlockGasPrice(ctx)
					if bigBlockGasPrice != "" {
						metrics.UpstreamRequestsTotal.Inc()
					}
					gasPriceInfo := &rpc.GasPriceInfo{
						GasPrice:         gasPrice,
						BigBlockGasPrice: bigBlockGasPrice,
						BlockNumber:      blockNum,
					}
					bc.BroadcastGasPrice(gasPriceInfo)
					lastGasPrice = gasPrice
				}
			}
		}

		if blockNum == "" || blockNum == lastBlockNum {
			continue
		}

		fullBlock, err := client.GetFullBlock(ctx, blockNum)
		if err != nil {
			logger.Error("Failed to fetch block: %v", err)
			metrics.UpstreamErrorsTotal.Inc()
			continue
		}

		metrics.UpstreamRequestsTotal.Inc()

		if fullBlock != nil {
			var blockInt int64
			fmt.Sscanf(fullBlock.Number, "0x%x", &blockInt)
			logger.Info("Block: %s (%d)", fullBlock.Number, blockInt)
			metrics.BlocksProcessedTotal.Inc()
			bc.BroadcastNewHead(fullBlock)

			// Broadcast logs
			logs, err := client.GetBlockLogs(ctx, blockNum)
			if err == nil {
				metrics.UpstreamRequestsTotal.Inc()
				for _, logEntry := range logs {
					bc.BroadcastLog(&logEntry)
				}
			}

			// Broadcast block receipts if there are subscribers
			if len(subMgr.GetSubscriptionsByType(subscription.SubTypeBlockReceipts)) > 0 {
				receipts, err := client.GetBlockReceipts(ctx, blockNum)
				if err == nil {
					metrics.UpstreamRequestsTotal.Inc()
					blockReceipts := &rpc.BlockReceipts{
						BlockNumber: fullBlock.Number,
						BlockHash:   fullBlock.Hash,
						Receipts:    receipts,
					}
					bc.BroadcastBlockReceipts(blockReceipts)
				}
			}

			lastBlockNum = blockNum
		}
	}
}

// pollSyncing checks sync status every 1 second with a 2s timeout
func pollSyncing(client *rpc.Client, bc *broadcaster.Broadcaster, cfg *config.Config) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	const queryTimeout = 2 * time.Second

	for range ticker.C {
		subMgr := bc.SubscriptionManager()
		if len(subMgr.GetSubscriptionsByType(subscription.SubTypeSyncing)) == 0 {
			continue
		}

		// Create context with 2s timeout
		ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)

		// Try to get the latest block with timeout
		blockNum, err := client.GetBlockNumber(ctx)
		if err != nil {
			cancel()
			// Query failed or timeout - consider node out of sync
			logger.Warn("Sync check failed (timeout or error): %v", err)
			syncStatus := &rpc.SyncStatus{Syncing: true}
			bc.BroadcastSyncing(syncStatus)
			continue
		}

		fullBlock, err := client.GetFullBlock(ctx, blockNum)
		cancel()

		if err != nil || fullBlock == nil {
			// Cannot get block - consider node out of sync
			logger.Warn("Sync check failed (cannot get block): %v", err)
			syncStatus := &rpc.SyncStatus{Syncing: true}
			bc.BroadcastSyncing(syncStatus)
			continue
		}

		// Parse block timestamp (hex string to int64)
		var blockTimestamp int64
		_, parseErr := fmt.Sscanf(fullBlock.Timestamp, "0x%x", &blockTimestamp)
		if parseErr != nil || blockTimestamp == 0 {
			// Cannot parse timestamp - consider node out of sync
			logger.Warn("Sync check failed (cannot parse timestamp): %v", parseErr)
			syncStatus := &rpc.SyncStatus{Syncing: true}
			bc.BroadcastSyncing(syncStatus)
			continue
		}

		blockTime := time.Unix(blockTimestamp, 0)
		blockAge := time.Since(blockTime)

		// Node is out of sync if block is older than threshold
		isSyncing := blockAge > cfg.SyncThreshold

		syncStatus := &rpc.SyncStatus{
			Syncing:      isSyncing,
			CurrentBlock: fullBlock.Number,
		}

		if isSyncing {
			logger.Warn("Node out of sync: block %s is %.1fs old (threshold: %v)", fullBlock.Number, blockAge.Seconds(), cfg.SyncThreshold)
		}

		bc.BroadcastSyncing(syncStatus)
	}
}
