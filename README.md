# hlnode-websocket

WebSocket server for Hyperliquid EVM with eth_subscribe support.

## Features

- **WebSocket subscriptions**: Real-time streaming for blocks, logs, gas prices, and more
- **Custom Hyperliquid subscriptions**: `gasPrice`, `blockReceipts`, `syncing`
- **High performance**: Written in Go with minimal overhead
- **Lightweight**: WebSocket-only, no HTTP RPC overhead

## Quick Start

### Using Docker (recommended)

```bash
docker pull imperatorco/hlnode-websocket:latest
docker run -p 8080:8080 -e RPC_URL=http://your-node:3001/evm imperatorco/hlnode-websocket
```

### From Source

```bash
make build
RPC_URL=http://your-node:3001/evm ./hlnode-websocket
```

## Endpoint

| Endpoint | Description |
|----------|-------------|
| `ws://localhost:8080/` | WebSocket subscriptions |

## WebSocket Subscriptions

Connect via WebSocket: `ws://localhost:8080`

### Supported Subscription Types

| Type | Description | Custom |
|------|-------------|--------|
| `newHeads` | New block headers | ❌ |
| `logs` | Contract event logs with filters | ❌ |
| `gasPrice` | Gas price updates in real-time | ✅ Hyperliquid |
| `blockReceipts` | All transaction receipts per block | ✅ Hyperliquid |
| `syncing` | Sync status (eth_syncing compatible) | ✅ Hyperliquid |

---

### `newHeads` - Subscribe to new blocks

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "eth_subscribe",
  "params": ["newHeads"]
}
```

**Response:**
```json
{"jsonrpc":"2.0","id":1,"result":"0x9ce59a13ff..."}
```

**Notification:**
```json
{
  "jsonrpc": "2.0",
  "method": "eth_subscription",
  "params": {
    "subscription": "0x9ce59a13ff...",
    "result": {
      "number": "0x14c3a5f",
      "hash": "0x...",
      "parentHash": "0x...",
      "timestamp": "0x675d1234",
      "gasUsed": "0x5208",
      "gasLimit": "0x1c9c380"
    }
  }
}
```

---

### `logs` - Subscribe to contract events

**Request (single address):**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "eth_subscribe",
  "params": [
    "logs",
    {
      "address": "0xdAC17F958D2ee523a2206206994597C13D831ec7"
    }
  ]
}
```

**Request (multiple addresses):**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "eth_subscribe",
  "params": [
    "logs",
    {
      "address": [
        "0xf24090f1895cee4033103e670cc58edc28294841",
        "0xdAC17F958D2ee523a2206206994597C13D831ec7"
      ]
    }
  ]
}
```

**Request (filter by topics - e.g., Transfer events):**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "eth_subscribe",
  "params": [
    "logs",
    {
      "address": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
      "topics": [
        "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
      ]
    }
  ]
}
```

**Notification:**
```json
{
  "jsonrpc": "2.0",
  "method": "eth_subscription",
  "params": {
    "subscription": "0x...",
    "result": {
      "address": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
      "topics": ["0xddf252ad..."],
      "data": "0x...",
      "blockNumber": "0x14c3a5f",
      "transactionHash": "0x...",
      "logIndex": "0x0"
    }
  }
}
```

---

### `gasPrice` - Subscribe to gas price updates (Custom)

Real-time notifications when gas price changes.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "eth_subscribe",
  "params": ["gasPrice"]
}
```

**Notification:**
```json
{
  "jsonrpc": "2.0",
  "method": "eth_subscription",
  "params": {
    "subscription": "0x...",
    "result": {
      "gasPrice": "0x174876e800",
      "bigBlockGasPrice": "0x2540be400",
      "blockNumber": "0x14c3a5f"
    }
  }
}
```

---

### `blockReceipts` - Subscribe to block receipts (Custom)

Receive all transaction receipts for each new block.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "eth_subscribe",
  "params": ["blockReceipts"]
}
```

**Notification:**
```json
{
  "jsonrpc": "2.0",
  "method": "eth_subscription",
  "params": {
    "subscription": "0x...",
    "result": {
      "blockNumber": "0x14c3a5f",
      "blockHash": "0x...",
      "receipts": [
        {
          "transactionHash": "0x...",
          "from": "0x...",
          "to": "0x...",
          "status": "0x1",
          "gasUsed": "0x5208",
          "effectiveGasPrice": "0x174876e800",
          "logs": []
        }
      ]
    }
  }
}
```

---

### `syncing` - Subscribe to sync status (Custom)

Real-time sync status updates.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "eth_subscribe",
  "params": ["syncing"]
}
```

**Notification (not syncing):**
```json
{
  "jsonrpc": "2.0",
  "method": "eth_subscription",
  "params": {
    "subscription": "0x...",
    "result": false
  }
}
```

**Notification (syncing):**
```json
{
  "jsonrpc": "2.0",
  "method": "eth_subscription",
  "params": {
    "subscription": "0x...",
    "result": {
      "syncing": true,
      "startingBlock": "0x0",
      "currentBlock": "0x14c3a5f",
      "highestBlock": "0x14c4000"
    }
  }
}
```

---

### `eth_unsubscribe` - Unsubscribe

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 9,
  "method": "eth_unsubscribe",
  "params": ["0x9ce59a13ff..."]
}
```

**Response:**
```json
{"jsonrpc":"2.0","id":9,"result":true}
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RPC_URL` | - | Upstream RPC URL (required) |
| `WS_PORT` | `8080` | WebSocket server port |
| `POLL_INTERVAL` | `100ms` | Block polling interval |

## Development

```bash
make build      # Build binary
make test       # Run tests
make docker     # Build Docker image locally
make run        # Build and run locally
```

## CI/CD

### Release to Docker Hub

```bash
git tag v1.0.0
git push origin v1.0.0
```

**Docker Hub:** https://hub.docker.com/r/imperatorco/hlnode-websocket

## License

MIT