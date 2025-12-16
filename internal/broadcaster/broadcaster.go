package broadcaster

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"hlnode-websocket/internal/logger"
	"hlnode-websocket/internal/metrics"
	"hlnode-websocket/internal/rpc"
	"hlnode-websocket/internal/subscription"

	"github.com/gorilla/websocket"
)

// ClientInfo contains metadata about a connected client
type ClientInfo struct {
	ID            string    `json:"id"`
	IP            string    `json:"ip"`
	UserAgent     string    `json:"userAgent"`
	ConnectedAt   time.Time `json:"connectedAt"`
	Subscriptions []string  `json:"subscriptions"`
	MessagesSent  int64     `json:"messagesSent"`
	MessagesRecv  int64     `json:"messagesReceived"`
}

// Client represents a WebSocket client
type Client struct {
	ID          string
	IP          string
	UserAgent   string
	ConnectedAt time.Time
	conn        *websocket.Conn
	send        chan []byte
	closed      atomic.Bool
	msgSent     atomic.Int64
	msgRecv     atomic.Int64
	mu          sync.Mutex
}

// Broadcaster manages WebSocket clients and broadcasts messages
type Broadcaster struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	subManager *subscription.Manager
	mu         sync.RWMutex

	totalConnections    atomic.Int64
	totalDisconnections atomic.Int64
}

// NewBroadcaster creates a new broadcaster instance
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 1000),
		unregister: make(chan *Client, 1000),
		subManager: subscription.NewManager(),
	}
}

// NewClient creates a new WebSocket client with metadata
func NewClient(conn *websocket.Conn, r *http.Request) *Client {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	return &Client{
		ID:          generateClientID(),
		IP:          ip,
		UserAgent:   r.UserAgent(),
		ConnectedAt: time.Now(),
		conn:        conn,
		send:        make(chan []byte, 512),
	}
}

// Run starts the broadcaster's main loop
func (b *Broadcaster) Run() {
	for {
		select {
		case client := <-b.register:
			b.mu.Lock()
			b.clients[client.ID] = client
			b.mu.Unlock()
			b.totalConnections.Add(1)

			metrics.WSActiveConnections.Inc()
			metrics.WSConnectionsTotal.Inc()

			logger.Info("Client %s connected from %s (total: %d)", client.ID, client.IP, len(b.clients))

		case client := <-b.unregister:
			b.mu.Lock()
			if _, ok := b.clients[client.ID]; ok {
				delete(b.clients, client.ID)
				close(client.send)
				b.subManager.UnsubscribeAll(client.ID)
			}
			b.mu.Unlock()
			b.totalDisconnections.Add(1)

			metrics.WSActiveConnections.Dec()
			metrics.WSDisconnectionsTotal.Inc()

			logger.Info("Client %s disconnected (total: %d)", client.ID, len(b.clients))
		}
	}
}

// Register adds a client to the broadcaster
func (b *Broadcaster) Register(client *Client) {
	b.register <- client
}

// Unregister removes a client from the broadcaster
func (b *Broadcaster) Unregister(client *Client) {
	b.unregister <- client
}

// SubscriptionManager returns the subscription manager
func (b *Broadcaster) SubscriptionManager() *subscription.Manager {
	return b.subManager
}

// GetClientInfo returns info about a specific client
func (b *Broadcaster) GetClientInfo(clientID string) *ClientInfo {
	b.mu.RLock()
	client, ok := b.clients[clientID]
	b.mu.RUnlock()

	if !ok {
		return nil
	}

	subs := b.subManager.GetClientSubscriptions(clientID)

	return &ClientInfo{
		ID:            client.ID,
		IP:            client.IP,
		UserAgent:     client.UserAgent,
		ConnectedAt:   client.ConnectedAt,
		Subscriptions: subs,
		MessagesSent:  client.msgSent.Load(),
		MessagesRecv:  client.msgRecv.Load(),
	}
}

// GetAllClientsInfo returns info about all connected clients
func (b *Broadcaster) GetAllClientsInfo() []ClientInfo {
	b.mu.RLock()
	clients := make([]*Client, 0, len(b.clients))
	for _, c := range b.clients {
		clients = append(clients, c)
	}
	b.mu.RUnlock()

	infos := make([]ClientInfo, 0, len(clients))
	for _, client := range clients {
		subs := b.subManager.GetClientSubscriptions(client.ID)
		infos = append(infos, ClientInfo{
			ID:            client.ID,
			IP:            client.IP,
			UserAgent:     client.UserAgent,
			ConnectedAt:   client.ConnectedAt,
			Subscriptions: subs,
			MessagesSent:  client.msgSent.Load(),
			MessagesRecv:  client.msgRecv.Load(),
		})
	}
	return infos
}

// Stats returns connection statistics
type Stats struct {
	ActiveClients       int   `json:"activeClients"`
	TotalConnections    int64 `json:"totalConnections"`
	TotalDisconnections int64 `json:"totalDisconnections"`
}

func (b *Broadcaster) GetStats() Stats {
	b.mu.RLock()
	active := len(b.clients)
	b.mu.RUnlock()

	return Stats{
		ActiveClients:       active,
		TotalConnections:    b.totalConnections.Load(),
		TotalDisconnections: b.totalDisconnections.Load(),
	}
}

// SendToClient sends a message to a specific client by ID
func (b *Broadcaster) SendToClient(clientID string, data []byte) bool {
	b.mu.RLock()
	client, ok := b.clients[clientID]
	b.mu.RUnlock()

	if !ok {
		return false
	}

	select {
	case client.send <- data:
		client.msgSent.Add(1)
		metrics.WSMessagesSent.Inc()
		return true
	default:
		return false
	}
}

// BroadcastNewHead sends a new block header to all newHeads subscribers
func (b *Broadcaster) BroadcastNewHead(header *rpc.FullBlockHeader) {
	subs := b.subManager.GetSubscriptionsByType(subscription.SubTypeNewHeads)
	if len(subs) == 0 {
		return
	}

	for _, sub := range subs {
		data, err := subscription.CreateNotification(sub.ID, header)
		if err != nil {
			logger.Error("Failed to create notification: %v", err)
			continue
		}
		if b.SendToClient(sub.ClientID, data) {
			metrics.WSBlockNotificationsSent.Inc()
		}
	}
}

// BroadcastLog sends logs to subscribers matching their filters
func (b *Broadcaster) BroadcastLog(logEntry *rpc.Log) {
	subs := b.subManager.GetSubscriptionsByType(subscription.SubTypeLogs)
	if len(subs) == 0 {
		return
	}

	for _, sub := range subs {
		var filter subscription.LogFilter
		if len(sub.Params) > 0 {
			json.Unmarshal(sub.Params, &filter)
		}

		if !subscription.MatchesLogFilter(logEntry, &filter) {
			continue
		}

		data, err := subscription.CreateNotification(sub.ID, logEntry)
		if err != nil {
			logger.Error("Failed to create log notification: %v", err)
			continue
		}
		if b.SendToClient(sub.ClientID, data) {
			metrics.WSLogNotificationsSent.Inc()
		}
	}
}

// BroadcastGasPrice sends gas price updates to subscribers
func (b *Broadcaster) BroadcastGasPrice(gasPriceInfo *rpc.GasPriceInfo) {
	subs := b.subManager.GetSubscriptionsByType(subscription.SubTypeGasPrice)
	if len(subs) == 0 {
		return
	}

	for _, sub := range subs {
		data, err := subscription.CreateNotification(sub.ID, gasPriceInfo)
		if err != nil {
			logger.Error("Failed to create gas price notification: %v", err)
			continue
		}
		if b.SendToClient(sub.ClientID, data) {
			metrics.WSGasPriceNotificationsSent.Inc()
		}
	}
}

// BroadcastBlockReceipts sends block receipts to subscribers
func (b *Broadcaster) BroadcastBlockReceipts(receipts *rpc.BlockReceipts) {
	subs := b.subManager.GetSubscriptionsByType(subscription.SubTypeBlockReceipts)
	if len(subs) == 0 {
		return
	}

	for _, sub := range subs {
		data, err := subscription.CreateNotification(sub.ID, receipts)
		if err != nil {
			logger.Error("Failed to create block receipts notification: %v", err)
			continue
		}
		if b.SendToClient(sub.ClientID, data) {
			metrics.WSBlockReceiptsNotificationsSent.Inc()
		}
	}
}

// BroadcastSyncing sends sync status updates to subscribers
// For Hyperliquid, this typically returns false (not syncing)
func (b *Broadcaster) BroadcastSyncing(syncStatus *rpc.SyncStatus) {
	subs := b.subManager.GetSubscriptionsByType(subscription.SubTypeSyncing)
	if len(subs) == 0 {
		return
	}

	// Standard eth_syncing subscription returns just the sync object or false
	var result interface{}
	if syncStatus.Syncing {
		result = syncStatus
	} else {
		result = false
	}

	for _, sub := range subs {
		data, err := subscription.CreateNotification(sub.ID, result)
		if err != nil {
			logger.Error("Failed to create sync notification: %v", err)
			continue
		}
		if b.SendToClient(sub.ClientID, data) {
			metrics.WSSyncingNotificationsSent.Inc()
		}
	}
}

// ClientCount returns the number of connected clients
func (b *Broadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// WritePump pumps messages from the send channel to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// IncrementRecv increments the received message counter
func (c *Client) IncrementRecv() {
	c.msgRecv.Add(1)
	metrics.WSMessagesReceived.Inc()
}

// Close marks the client as closed
func (c *Client) Close() {
	c.closed.Store(true)
}

// IsClosed returns whether the client is closed
func (c *Client) IsClosed() bool {
	return c.closed.Load()
}

// Conn returns the underlying WebSocket connection
func (c *Client) Conn() *websocket.Conn {
	return c.conn
}

// Send returns the send channel
func (c *Client) Send() chan []byte {
	return c.send
}

func generateClientID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
