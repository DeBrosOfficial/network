# Multi-stage build for DeBros Network
# Stage 1: Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev wget tar

# Set working directory
WORKDIR /app

# Build RQLite from source for Alpine compatibility
RUN RQLITE_VERSION="8.43.0" && \
    git clone --depth 1 --branch v${RQLITE_VERSION} https://github.com/rqlite/rqlite.git /tmp/rqlite && \
    cd /tmp/rqlite && \
    go build -o /usr/local/bin/rqlited ./cmd/rqlited && \
    go build -o /usr/local/bin/rqlite ./cmd/rqlite && \
    chmod +x /usr/local/bin/rqlited /usr/local/bin/rqlite && \
    rm -rf /tmp/rqlite

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build only node and gateway executables (CLI is built separately on host)
RUN mkdir -p bin && \
    go build -ldflags "-X 'main.version=0.50.1-beta' -X 'main.commit=unknown' -X 'main.date=2025-09-23T15:40:00Z'" -o bin/node ./cmd/node && \
    go build -ldflags "-X 'main.version=0.50.1-beta' -X 'main.commit=unknown' -X 'main.date=2025-09-23T15:40:00Z' -X 'github.com/DeBrosOfficial/network/pkg/gateway.BuildVersion=0.51.1-beta' -X 'github.com/DeBrosOfficial/network/pkg/gateway.BuildCommit=unknown' -X 'github.com/DeBrosOfficial/network/pkg/gateway.BuildTime=2025-09-23T15:40:00Z'" -o bin/gateway ./cmd/gateway

# Stage 2: Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user
RUN addgroup -g 1001 -S debros && \
    adduser -u 1001 -S debros -G debros

# Create necessary directories
RUN mkdir -p /app/bin /app/data /app/configs /app/logs && \
    chown -R debros:debros /app

# Copy built executables from builder stage
COPY --from=builder /app/bin/ /app/bin/

# Copy RQLite binaries from builder stage
COPY --from=builder /usr/local/bin/rqlited /usr/local/bin/rqlited
COPY --from=builder /usr/local/bin/rqlite /usr/local/bin/rqlite

# Set working directory
WORKDIR /app

# Switch to non-root user
USER debros

# Expose ports
# 4001: LibP2P P2P communication
# 5001: RQLite HTTP API  
# 6001: Gateway HTTP/WebSocket
# 7001: RQLite Raft consensus
EXPOSE 4001 5001 6001 7001

# Default command (can be overridden in docker-compose)
CMD ["./bin/node"]
