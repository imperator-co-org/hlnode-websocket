package subscription

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"

	"hlnode-proxy/internal/logger"
	"hlnode-proxy/internal/metrics"
	"hlnode-proxy/internal/rpc"
)

// SubscriptionType represents the type of subscription
type SubscriptionType string

const (
	SubTypeNewHeads               SubscriptionType = "newHeads"
	SubTypeLogs                   SubscriptionType = "logs"
	SubTypeNewPendingTransactions SubscriptionType = "newPendingTransactions"
	SubTypeSyncing                SubscriptionType = "syncing"
)

// Subscription represents an active subscription
type Subscription struct {
	ID       string
	Type     SubscriptionType
	Params   json.RawMessage
	ClientID string
}

// LogFilter represents filter params for logs subscription
type LogFilter struct {
	Address []string   `json:"address,omitempty"`
	Topics  [][]string `json:"topics,omitempty"`
}

// Manager manages all active subscriptions
type Manager struct {
	subscriptions map[string]*Subscription
	clientSubs    map[string][]string
	mu            sync.RWMutex
}

// NewManager creates a new subscription manager
func NewManager() *Manager {
	return &Manager{
		subscriptions: make(map[string]*Subscription),
		clientSubs:    make(map[string][]string),
	}
}

// Subscribe creates a new subscription
func (m *Manager) Subscribe(clientID string, subType SubscriptionType, params json.RawMessage) (string, error) {
	subID := generateSubscriptionID()

	sub := &Subscription{
		ID:       subID,
		Type:     subType,
		Params:   params,
		ClientID: clientID,
	}

	m.mu.Lock()
	m.subscriptions[subID] = sub
	m.clientSubs[clientID] = append(m.clientSubs[clientID], subID)
	m.mu.Unlock()

	metrics.WSActiveSubscriptions.WithLabelValues(string(subType)).Inc()
	metrics.WSSubscriptionsCreated.WithLabelValues(string(subType)).Inc()

	logger.Info("Client %s subscribed to %s (sub_id: %s)", clientID, subType, subID)
	return subID, nil
}

// Unsubscribe removes a subscription
func (m *Manager) Unsubscribe(clientID, subID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	sub, exists := m.subscriptions[subID]
	if !exists || sub.ClientID != clientID {
		return false
	}

	delete(m.subscriptions, subID)

	subs := m.clientSubs[clientID]
	for i, id := range subs {
		if id == subID {
			m.clientSubs[clientID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	metrics.WSActiveSubscriptions.WithLabelValues(string(sub.Type)).Dec()
	metrics.WSSubscriptionsRemoved.WithLabelValues(string(sub.Type)).Inc()

	logger.Info("Client %s unsubscribed from %s (sub_id: %s)", clientID, sub.Type, subID)
	return true
}

// UnsubscribeAll removes all subscriptions for a client
func (m *Manager) UnsubscribeAll(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs := m.clientSubs[clientID]
	for _, subID := range subs {
		if sub, exists := m.subscriptions[subID]; exists {
			metrics.WSActiveSubscriptions.WithLabelValues(string(sub.Type)).Dec()
			metrics.WSSubscriptionsRemoved.WithLabelValues(string(sub.Type)).Inc()
			delete(m.subscriptions, subID)
		}
	}
	delete(m.clientSubs, clientID)

	if len(subs) > 0 {
		logger.Info("Removed %d subscriptions for client %s", len(subs), clientID)
	}
}

// GetSubscriptionsByType returns all subscriptions of a given type
func (m *Manager) GetSubscriptionsByType(subType SubscriptionType) []*Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Subscription
	for _, sub := range m.subscriptions {
		if sub.Type == subType {
			result = append(result, sub)
		}
	}
	return result
}

// GetClientSubscriptions returns subscription IDs for a client
func (m *Manager) GetClientSubscriptions(clientID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs := m.clientSubs[clientID]
	if subs == nil {
		return []string{}
	}
	result := make([]string, len(subs))
	copy(result, subs)
	return result
}

// SubscriptionNotification represents a notification sent to subscribers
type SubscriptionNotification struct {
	JSONRPC string             `json:"jsonrpc"`
	Method  string             `json:"method"`
	Params  NotificationParams `json:"params"`
}

// NotificationParams contains subscription notification params
type NotificationParams struct {
	Subscription string          `json:"subscription"`
	Result       json.RawMessage `json:"result"`
}

// CreateNotification creates a notification message for a subscription
func CreateNotification(subID string, result interface{}) ([]byte, error) {
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	notification := SubscriptionNotification{
		JSONRPC: "2.0",
		Method:  "eth_subscription",
		Params: NotificationParams{
			Subscription: subID,
			Result:       resultBytes,
		},
	}

	return json.Marshal(notification)
}

// MatchesLogFilter checks if a log matches the given filter
func MatchesLogFilter(logEntry *rpc.Log, filter *LogFilter) bool {
	if filter == nil {
		return true
	}

	if len(filter.Address) > 0 {
		found := false
		for _, addr := range filter.Address {
			if logEntry.Address == addr {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	for i, topicFilter := range filter.Topics {
		if len(topicFilter) == 0 {
			continue
		}
		if i >= len(logEntry.Topics) {
			return false
		}
		found := false
		for _, topic := range topicFilter {
			if logEntry.Topics[i] == topic {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func generateSubscriptionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "0x" + hex.EncodeToString(bytes)
}
