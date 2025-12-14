package subscription

import (
	"encoding/json"
	"testing"

	"hlnode-proxy/internal/rpc"
)

func TestManagerSubscribe(t *testing.T) {
	m := NewManager()

	subID, err := m.Subscribe("client1", SubTypeNewHeads, nil)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if subID == "" {
		t.Error("Expected non-empty subscription ID")
	}

	// Verify subscription exists
	subs := m.GetSubscriptionsByType(SubTypeNewHeads)
	if len(subs) != 1 {
		t.Errorf("Expected 1 subscription, got %d", len(subs))
	}

	if subs[0].ID != subID {
		t.Errorf("Subscription ID mismatch")
	}
}

func TestManagerUnsubscribe(t *testing.T) {
	m := NewManager()

	subID, _ := m.Subscribe("client1", SubTypeNewHeads, nil)

	// Unsubscribe
	success := m.Unsubscribe("client1", subID)
	if !success {
		t.Error("Unsubscribe should return true")
	}

	// Verify subscription removed
	subs := m.GetSubscriptionsByType(SubTypeNewHeads)
	if len(subs) != 0 {
		t.Errorf("Expected 0 subscriptions, got %d", len(subs))
	}
}

func TestManagerUnsubscribeWrongClient(t *testing.T) {
	m := NewManager()

	subID, _ := m.Subscribe("client1", SubTypeNewHeads, nil)

	// Try to unsubscribe with wrong client ID
	success := m.Unsubscribe("client2", subID)
	if success {
		t.Error("Unsubscribe should fail for wrong client")
	}
}

func TestManagerUnsubscribeAll(t *testing.T) {
	m := NewManager()

	m.Subscribe("client1", SubTypeNewHeads, nil)
	m.Subscribe("client1", SubTypeLogs, nil)
	m.Subscribe("client1", SubTypeGasPrice, nil)

	m.UnsubscribeAll("client1")

	// Verify all subscriptions removed
	if len(m.GetSubscriptionsByType(SubTypeNewHeads)) != 0 {
		t.Error("newHeads subscriptions should be removed")
	}
	if len(m.GetSubscriptionsByType(SubTypeLogs)) != 0 {
		t.Error("logs subscriptions should be removed")
	}
}

func TestCreateNotification(t *testing.T) {
	header := &rpc.FullBlockHeader{
		Number: "0x123",
		Hash:   "0xabc",
	}

	data, err := CreateNotification("0xsubid", header)
	if err != nil {
		t.Fatalf("CreateNotification failed: %v", err)
	}

	var notification SubscriptionNotification
	if err := json.Unmarshal(data, &notification); err != nil {
		t.Fatalf("Failed to unmarshal notification: %v", err)
	}

	if notification.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %s", notification.JSONRPC)
	}

	if notification.Method != "eth_subscription" {
		t.Errorf("Expected method eth_subscription, got %s", notification.Method)
	}

	if notification.Params.Subscription != "0xsubid" {
		t.Errorf("Expected subscription 0xsubid, got %s", notification.Params.Subscription)
	}
}

func TestMatchesLogFilter(t *testing.T) {
	tests := []struct {
		name     string
		log      *rpc.Log
		filter   *LogFilter
		expected bool
	}{
		{
			name:     "nil filter matches all",
			log:      &rpc.Log{Address: "0x1234"},
			filter:   nil,
			expected: true,
		},
		{
			name:     "empty filter matches all",
			log:      &rpc.Log{Address: "0x1234"},
			filter:   &LogFilter{},
			expected: true,
		},
		{
			name: "address match",
			log:  &rpc.Log{Address: "0x1234"},
			filter: &LogFilter{
				Address: []string{"0x1234"},
			},
			expected: true,
		},
		{
			name: "address no match",
			log:  &rpc.Log{Address: "0x1234"},
			filter: &LogFilter{
				Address: []string{"0x5678"},
			},
			expected: false,
		},
		{
			name: "topic match",
			log:  &rpc.Log{Topics: []string{"0xabc", "0xdef"}},
			filter: &LogFilter{
				Topics: [][]string{{"0xabc"}},
			},
			expected: true,
		},
		{
			name: "topic no match",
			log:  &rpc.Log{Topics: []string{"0xabc"}},
			filter: &LogFilter{
				Topics: [][]string{{"0xdef"}},
			},
			expected: false,
		},
		{
			name: "multiple addresses one match",
			log:  &rpc.Log{Address: "0x1234"},
			filter: &LogFilter{
				Address: []string{"0x5678", "0x1234"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesLogFilter(tt.log, tt.filter)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
