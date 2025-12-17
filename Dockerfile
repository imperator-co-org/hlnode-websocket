# Build stage
FROM golang:1.23-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with version info
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /hlnode-websocket ./cmd/server

# Runtime stage
FROM alpine:3.19

# OCI Labels
LABEL org.opencontainers.image.source="https://hub.docker.com/r/imperatorco/hlnode-websocket"
LABEL org.opencontainers.image.description="WebSocket service for Hyperliquid EVM with eth_subscribe support"
LABEL org.opencontainers.image.licenses="MIT"

RUN apk --no-cache add ca-certificates
RUN adduser -D -u 1000 appuser

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /hlnode-websocket .

USER appuser

CMD ["./hlnode-websocket"]
