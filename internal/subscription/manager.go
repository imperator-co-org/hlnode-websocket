package subscription

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
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
	// Custom Hyperliquid subscriptions
	SubTypeGasPrice      SubscriptionType = "gasPrice"
	SubTypeBlockReceipts SubscriptionType = "blockReceipts"
)

// Subscription represents an active subscription
type Subscription struct {
	ID       string
	Type     SubscriptionType
	Params   json.RawMessage
	ClientID string
}

// LogFilter represents filter params for logs subscription
// Supports flexible parsing where address can be string or []string
// and topics can be (string | []string | null)[] for position-based OR matching
type LogFilter struct {
	Address []string
	Topics  [][]string
}

// logFilterRaw is used for flexible JSON unmarshalling
type logFilterRaw struct {
	Address json.RawMessage   `json:"address,omitempty"`
	Topics  []json.RawMessage `json:"topics,omitempty"`
}

// UnmarshalJSON implements custom unmarshalling for LogFilter
// to handle flexible address and topics formats
func (f *LogFilter) UnmarshalJSON(data []byte) error {
	var raw logFilterRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Parse address: can be string or []string
	if len(raw.Address) > 0 {
		// Try as single string first
		var singleAddr string
		if err := json.Unmarshal(raw.Address, &singleAddr); err == nil {
			f.Address = []string{normalizeAddress(singleAddr)}
		} else {
			// Try as array of strings
			var addrArray []string
			if err := json.Unmarshal(raw.Address, &addrArray); err == nil {
				f.Address = make([]string, len(addrArray))
				for i, addr := range addrArray {
					f.Address[i] = normalizeAddress(addr)
				}
			}
		}
	}

	// Parse topics: each element can be null, string, or []string
	if len(raw.Topics) > 0 {
		f.Topics = make([][]string, len(raw.Topics))
		for i, topicRaw := range raw.Topics {
			if topicRaw == nil || string(topicRaw) == "null" {
				// null means match any topic at this position
				f.Topics[i] = nil
				continue
			}

			// Try as single string first
			var singleTopic string
			if err := json.Unmarshal(topicRaw, &singleTopic); err == nil {
				f.Topics[i] = []string{normalizeTopic(singleTopic)}
			} else {
				// Try as array of strings (OR matching)
				var topicArray []string
				if err := json.Unmarshal(topicRaw, &topicArray); err == nil {
					f.Topics[i] = make([]string, len(topicArray))
					for j, topic := range topicArray {
						f.Topics[i][j] = normalizeTopic(topic)
					}
				}
			}
		}
	}

	return nil
}

// normalizeAddress normalizes an Ethereum address to lowercase for comparison
func normalizeAddress(addr string) string {
	return strings.ToLower(addr)
}

// normalizeTopic normalizes a topic hash to lowercase for comparison
func normalizeTopic(topic string) string {
	return strings.ToLower(topic)
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
// Comparison is case-insensitive since filter values are normalized to lowercase
func MatchesLogFilter(logEntry *rpc.Log, filter *LogFilter) bool {
	if filter == nil {
		return true
	}

	// Check address filter (case-insensitive)
	if len(filter.Address) > 0 {
		found := false
		logAddr := strings.ToLower(logEntry.Address)
		for _, addr := range filter.Address {
			if logAddr == addr {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check topics filter (case-insensitive)
	// Each topic position can have multiple values (OR matching)
	// nil/empty at a position means match any
	for i, topicFilter := range filter.Topics {
		if len(topicFilter) == 0 {
			// nil or empty slice means match any topic at this position
			continue
		}
		if i >= len(logEntry.Topics) {
			// Log doesn't have enough topics
			return false
		}
		found := false
		logTopic := strings.ToLower(logEntry.Topics[i])
		for _, topic := range topicFilter {
			if logTopic == topic {
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
