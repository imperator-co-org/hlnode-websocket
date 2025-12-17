package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Registry is a custom registry without default Go metrics
var Registry = prometheus.NewRegistry()

var (
	// WebSocket Connection metrics
	WSActiveConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "hlnode_websocket_ws_active_connections",
		Help: "Number of active WebSocket connections",
	})

	WSConnectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_connections_total",
		Help: "Total number of WebSocket connections",
	})

	WSDisconnectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_disconnections_total",
		Help: "Total number of WebSocket disconnections",
	})

	// WebSocket Message metrics
	WSMessagesReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_messages_received_total",
		Help: "Total messages received from WebSocket clients",
	})

	WSMessagesSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_messages_sent_total",
		Help: "Total messages sent to WebSocket clients",
	})

	// WebSocket RPC requests
	WSRPCRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_rpc_requests_total",
		Help: "WebSocket JSON-RPC requests by method",
	}, []string{"method"})

	// Subscription metrics
	WSActiveSubscriptions = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "hlnode_websocket_ws_active_subscriptions",
		Help: "Active subscriptions by type",
	}, []string{"type"})

	WSSubscriptionsCreated = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_subscriptions_created_total",
		Help: "Subscriptions created by type",
	}, []string{"type"})

	WSSubscriptionsRemoved = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_subscriptions_removed_total",
		Help: "Subscriptions removed by type",
	}, []string{"type"})

	// Block notification metrics
	WSBlockNotificationsSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_block_notifications_total",
		Help: "Block notifications sent to subscribers",
	})

	WSLogNotificationsSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_log_notifications_total",
		Help: "Log notifications sent to subscribers",
	})

	WSGasPriceNotificationsSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_gas_price_notifications_total",
		Help: "Gas price notifications sent to subscribers",
	})

	WSBlockReceiptsNotificationsSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_block_receipts_notifications_total",
		Help: "Block receipts notifications sent to subscribers",
	})

	WSSyncingNotificationsSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_ws_syncing_notifications_total",
		Help: "Syncing notifications sent to subscribers",
	})

	// Upstream metrics (shared)
	UpstreamRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_upstream_requests_total",
		Help: "Total requests to upstream RPC",
	})

	UpstreamErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_upstream_errors_total",
		Help: "Total errors from upstream RPC",
	})

	// Block processing
	BlocksProcessedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "hlnode_websocket_blocks_processed_total",
		Help: "Total blocks processed",
	})
)

func init() {
	Registry.MustRegister(
		// WebSocket
		WSActiveConnections,
		WSConnectionsTotal,
		WSDisconnectionsTotal,
		WSMessagesReceived,
		WSMessagesSent,
		WSRPCRequestsTotal,
		// Subscriptions
		WSActiveSubscriptions,
		WSSubscriptionsCreated,
		WSSubscriptionsRemoved,
		WSBlockNotificationsSent,
		WSLogNotificationsSent,
		WSGasPriceNotificationsSent,
		WSBlockReceiptsNotificationsSent,
		WSSyncingNotificationsSent,

		// Upstream
		UpstreamRequestsTotal,
		UpstreamErrorsTotal,
		BlocksProcessedTotal,
	)
}
