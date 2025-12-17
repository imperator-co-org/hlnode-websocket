package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration
type Config struct {
	// RPCURL is the upstream Hyperliquid EVM RPC URL
	RPCURL string

	// WebSocketPort is the port for the WebSocket server
	WebSocketPort int

	// PollInterval is the interval for polling new blocks
	PollInterval time.Duration

	// SyncThreshold is the maximum allowed block age before considering node out of sync
	SyncThreshold time.Duration
}

// Load reads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		RPCURL:        getEnv("RPC_URL", ""),
		WebSocketPort: getEnvInt("WS_PORT", 8080),
		PollInterval:  getEnvDuration("POLL_INTERVAL", 100*time.Millisecond),
		SyncThreshold: getEnvDuration("SYNC_THRESHOLD", 15*time.Second),
	}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
