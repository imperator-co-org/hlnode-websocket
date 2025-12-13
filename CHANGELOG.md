# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
- JSON-RPC proxy for Hyperliquid EVM
- WebSocket support with eth_subscribe
- Subscriptions: newHeads, logs, newPendingTransactions, syncing
- Prometheus metrics
- Health check endpoint
