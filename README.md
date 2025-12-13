# hlnode-proxy

[![Build](https://github.com/imperatorco/hlnode_proxy/actions/workflows/build.yml/badge.svg)](https://github.com/imperatorco/hlnode_proxy/actions/workflows/build.yml)
[![Docker](https://img.shields.io/docker/v/imperatorco/hlnode-proxy?label=Docker%20Hub)](https://hub.docker.com/r/imperatorco/hlnode-proxy)

JSON-RPC and WebSocket proxy for Hyperliquid EVM with eth_subscribe support.

## Quick Start

### Using Docker (recommended)

```bash
docker pull imperatorco/hlnode-proxy:latest
docker run -p 8080:8080 -e RPC_URL=http://your-node:3001/evm imperatorco/hlnode-proxy
```

### From Source

```bash
make build
RPC_URL=http://your-node:3001/evm ./hlnode-proxy
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /` | JSON-RPC (forwards all methods) |
| `GET /` | WebSocket subscriptions (auto-detected) |
| `GET /metrics` | Prometheus metrics |
| `GET /health` | Health check |
| `GET /connections` | List active clients |
| `GET /stats` | Server statistics |

## WebSocket Subscriptions

```javascript
const ws = new WebSocket('ws://localhost:8080');

// Subscribe to new blocks
ws.send(JSON.stringify({
  jsonrpc: "2.0",
  method: "eth_subscribe",
  params: ["newHeads"],
  id: 1
}));

// Subscribe to logs with filter
ws.send(JSON.stringify({
  jsonrpc: "2.0",
  method: "eth_subscribe",
  params: ["logs", { address: "0x..." }],
  id: 2
}));
```

**Supported:** `newHeads`, `logs`, `newPendingTransactions`, `syncing`

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RPC_URL` | - | Upstream RPC URL |
| `PROXY_PORT` | `8080` | Server port |
| `POLL_INTERVAL` | `100ms` | Block polling interval |

## Development

```bash
make build      # Build binary
make test       # Run tests
make lint       # Run golangci-lint
make docker     # Build Docker image locally
make run        # Build and run locally
```

## CI/CD

### Automatic Build

Every push to `main` and every Pull Request triggers the **build** workflow:
- Lint (golangci-lint)
- Tests with coverage
- Binary build
- Docker image build

### Release to Docker Hub

Create a tag to publish a new version:

```bash
git tag v1.0.0
git push origin v1.0.0
```

This automatically:
1. Builds multi-arch images (amd64 + arm64)
2. Pushes to Docker Hub: `imperatorco/hlnode-proxy:1.0.0`
3. Creates a GitHub Release

**Docker Hub:** https://hub.docker.com/r/imperatorco/hlnode-proxy

## Prometheus Metrics

| Metric | Description |
|--------|-------------|
| `hlnode_proxy_ws_active_connections` | Active WebSocket connections |
| `hlnode_proxy_ws_rpc_requests_total{method}` | WS RPC requests by method |
| `hlnode_proxy_rpc_requests_total{method}` | HTTP RPC requests by method |
| `hlnode_proxy_rpc_request_duration_seconds{method}` | Request duration histogram |
| `hlnode_proxy_blocks_processed_total` | Blocks processed |

## License

MIT
