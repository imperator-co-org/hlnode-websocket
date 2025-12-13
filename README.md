# hlnode-proxy

JSON-RPC and WebSocket proxy for Hyperliquid EVM with eth_subscribe support.

## Quick Start

```bash
go build -o hlnode-proxy ./cmd/server
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

## Prometheus Metrics

| Metric | Description |
|--------|-------------|
| `hlnode_proxy_ws_active_connections` | Active WebSocket connections |
| `hlnode_proxy_ws_rpc_requests_total{method}` | WS RPC requests by method |
| `hlnode_proxy_rpc_requests_total{method}` | HTTP RPC requests by method |
| `hlnode_proxy_rpc_request_duration_seconds{method}` | Request duration histogram |
| `hlnode_proxy_blocks_processed_total` | Blocks processed |

## Docker

```bash
docker build -t hlnode-proxy .
docker run -p 8080:8080 -e RPC_URL=http://node:3001/evm hlnode-proxy
```

## Tests

```bash
go test -v ./...
```
