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

	go pollBlocks(rpcClient, bc, cfg.PollInterval)

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

func pollBlocks(client *rpc.Client, bc *broadcaster.Broadcaster, interval time.Duration) {
	ticker := time.NewTicker(interval)
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

			// Broadcast syncing status if there are subscribers
			if len(subMgr.GetSubscriptionsByType(subscription.SubTypeSyncing)) > 0 {
				syncStatus, err := client.GetSyncing(ctx)
				if err == nil {
					metrics.UpstreamRequestsTotal.Inc()
					bc.BroadcastSyncing(syncStatus)
				}
			}

			lastBlockNum = blockNum
		}
	}
}
