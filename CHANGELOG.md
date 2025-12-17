# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.6] - 2025-12-17

### Changed
- **Syncing interval**: Now checks every 1 second instead of per block
- **Robust error handling**: Returns `true` (out of sync) on query timeout (2s), errors, or parse failures

## [1.0.5] - 2025-12-17

### Added
- **Smart sync detection**: `syncing` subscription now compares block timestamp with current time
- New `SYNC_THRESHOLD` environment variable (default: 15s) to configure max block age

### Changed
- `syncing` subscription no longer forwards `eth_syncing`, uses intelligent block age detection instead

## [1.0.4] - 2025-12-17

### Changed
- **WebSocket-only**: Removed HTTP JSON-RPC proxy functionality, now exclusively WebSocket
- Updated documentation to reflect WebSocket-only architecture
- Cleaned up README, Dockerfile labels, and CHANGELOG references

### Removed
- HTTP `POST /` endpoint for JSON-RPC requests
- `hlnode_websocket_rpc_requests_total` metric (HTTP RPC)

## [1.0.1] - 2025-12-14

### Added
- **Custom `gasPrice` subscription**: Real-time gas price updates with `bigBlockGasPrice` support (Hyperliquid-specific)
- **Custom `blockReceipts` subscription**: Stream all transaction receipts for each new block
- **Custom `syncing` subscription**: eth_syncing compatible sync status subscription
- Flexible log filter parsing: `address` can be string or array, `topics` supports OR matching
- Case-insensitive address/topic matching for log filters
- New Prometheus metrics: `ws_gas_price_notifications_total`, `ws_block_receipts_notifications_total`, `ws_syncing_notifications_total`

### Changed
- README updated with comprehensive examples for all subscription types
- Documentation now uses JSON/Postman-friendly examples

### Removed
- Removed `newPendingTransactions` subscription (not supported by Hyperliquid)

## [1.0.0] - 2025-12-13

### Added
- GitHub Actions CI/CD pipeline
- Multi-architecture Docker builds (amd64, arm64)
- Automatic version tagging for Docker images
- Makefile for local development

### Changed
- Improved Dockerfile with non-root user and OCI labels
- Moved main.go to cmd/server/ following Go project layout

## [0.1.0] - 2025-12-13

### Added
- Initial release
- WebSocket service for Hyperliquid EVM
- WebSocket support with eth_subscribe
- Subscriptions: newHeads, logs
- Prometheus metrics
- Health check endpoint
