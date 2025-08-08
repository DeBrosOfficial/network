# Network - Distributed P2P Database System
# Makefile for development and build tasks

.PHONY: build clean test run-node run-node2 run-node3 run-example deps tidy fmt vet

# Build targets
build: deps
	@echo "Building network executables..."
	@mkdir -p bin
	go build -o bin/node cmd/node/main.go
	go build -o bin/network-cli cmd/cli/main.go
	@echo "Build complete!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf data/
	@echo "Clean complete!"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run bootstrap node explicitly
run-node:
	@echo "Starting BOOTSTRAP node (role=bootstrap)..."
	go run cmd/node/main.go -role bootstrap -data ./data/bootstrap

# Run second node (regular) - requires BOOTSTRAP multiaddr
# Usage: make run-node2 BOOTSTRAP=/ip4/127.0.0.1/tcp/4001/p2p/<ID> HTTP=5002 RAFT=7002
run-node2:
	@echo "Starting REGULAR node2 (role=node)..."
	@if [ -z "$(BOOTSTRAP)" ]; then echo "ERROR: Provide BOOTSTRAP multiaddr: make run-node2 BOOTSTRAP=/ip4/127.0.0.1/tcp/4001/p2p/<ID> [HTTP=5002 RAFT=7002]"; exit 1; fi
	go run cmd/node/main.go -role node -id node2 -data ./data/node2 -bootstrap $(BOOTSTRAP) -rqlite-http-port ${HTTP-5002} -rqlite-raft-port ${RAFT-7002}

# Run third node (regular) - requires BOOTSTRAP multiaddr
# Usage: make run-node3 BOOTSTRAP=/ip4/127.0.0.1/tcp/4001/p2p/<ID> HTTP=5003 RAFT=7003
run-node3:
	@echo "Starting REGULAR node3 (role=node)..."
	@if [ -z "$(BOOTSTRAP)" ]; then echo "ERROR: Provide BOOTSTRAP multiaddr: make run-node3 BOOTSTRAP=/ip4/127.0.0.1/tcp/4001/p2p/<ID> [HTTP=5003 RAFT=7003]"; exit 1; fi
	go run cmd/node/main.go -role node -id node3 -data ./data/node3 -bootstrap $(BOOTSTRAP) -rqlite-http-port ${HTTP-5003} -rqlite-raft-port ${RAFT-7003}

# Show how to run with flags
show-bootstrap:
    @echo "Provide bootstrap via flags, e.g.:"
    @echo "  make run-node2 BOOTSTRAP=/ip4/127.0.0.1/tcp/4001/p2p/<PEER_ID> HTTP=5002 RAFT=7002"

# Run network CLI
run-cli:
	@echo "Running network CLI help..."
	./bin/network-cli help

# Network CLI helper commands
cli-health:
	@echo "Checking network health..."
	./bin/network-cli health

cli-peers:
	@echo "Listing network peers..."
	./bin/network-cli peers

cli-status:
	@echo "Getting network status..."
	./bin/network-cli status

cli-storage-test:
	@echo "Testing storage operations..."
	@./bin/network-cli storage put test-key "Hello Network" || echo "Storage test requires running network"
	@./bin/network-cli storage get test-key || echo "Storage test requires running network"
	@./bin/network-cli storage list || echo "Storage test requires running network"

cli-pubsub-test:
	@echo "Testing pub/sub operations..."
	@./bin/network-cli pubsub publish test-topic "Hello World" || echo "PubSub test requires running network"
	@./bin/network-cli pubsub topics || echo "PubSub test requires running network"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Development setup
dev-setup: deps
	@echo "Setting up development environment..."
	@mkdir -p data/bootstrap data/node data/node-node2 data/node-node3
	@mkdir -p data/test-bootstrap data/test-node1 data/test-node2
	@echo "Development setup complete!"

# Multi-node testing
test-multinode: build
	@echo "🧪 Starting comprehensive multi-node test..."
	@chmod +x scripts/test-multinode.sh
	@./scripts/test-multinode.sh

test-peer-discovery: build
	@echo "🔍 Testing peer discovery (requires running nodes)..."
	@echo "Connected peers:"
	@./bin/network-cli peers --timeout 10s

test-replication: build
	@echo "🔄 Testing data replication (requires running nodes)..."
	@./bin/network-cli storage put "replication:test:$$(date +%s)" "Test data - $$(date)"
	@sleep 2
	@echo "Retrieving replicated data:"
	@./bin/network-cli storage list replication:test:

test-consensus: build
	@echo "🗄️ Testing database consensus (requires running nodes)..."
	@./bin/network-cli query "CREATE TABLE IF NOT EXISTS consensus_test (id INTEGER PRIMARY KEY, test_data TEXT, timestamp TEXT)"
	@./bin/network-cli query "INSERT INTO consensus_test (test_data, timestamp) VALUES ('Makefile test', '$$(date)')"
	@./bin/network-cli query "SELECT * FROM consensus_test ORDER BY id DESC LIMIT 5"

# Start development cluster (requires multiple terminals)
dev-cluster:
	@echo "To start a development cluster, run these commands in separate terminals:"
	@echo "1. make run-node           # Start bootstrap node"
	@echo "2. make run-node2 BOOTSTRAP=/ip4/127.0.0.1/tcp/4001/p2p/<ID> HTTP=5002 RAFT=7002"
	@echo "3. make run-node3 BOOTSTRAP=/ip4/127.0.0.1/tcp/4001/p2p/<ID> HTTP=5003 RAFT=7003"
	@echo "4. make run-example        # Test basic functionality"
	@echo "5. make cli-health         # Check network health"
	@echo "6. make cli-peers          # List peers"
	@echo "7. make cli-storage-test   # Test storage"
	@echo "8. make cli-pubsub-test    # Test messaging"

# Full development workflow
dev: clean build test
	@echo "Development workflow complete!"

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build all executables"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  run-node      - Start bootstrap node (role=bootstrap)"
	@echo "  run-node2     - Start second node (role=node). Provide BOOTSTRAP, optional HTTP/RAFT"
	@echo "  run-node3     - Start third node (role=node). Provide BOOTSTRAP, optional HTTP/RAFT"
	@echo "  run-example   - Run usage example"
	@echo "  run-cli       - Run network CLI help"
	@echo "  show-bootstrap - Show example bootstrap usage with flags"
	@echo "  cli-health    - Check network health"
	@echo "  cli-peers     - List network peers"
	@echo "  cli-status    - Get network status"
	@echo "  cli-storage-test - Test storage operations"
	@echo "  cli-pubsub-test  - Test pub/sub operations"
	@echo "  test-multinode   - Full multi-node test with 1 bootstrap + 2 nodes"
	@echo "  test-peer-discovery - Test peer discovery (requires running nodes)"
	@echo "  test-replication - Test data replication (requires running nodes)"
	@echo "  test-consensus   - Test database consensus (requires running nodes)"
	@echo "  deps          - Download dependencies"
	@echo "  tidy          - Tidy dependencies"
	@echo "  fmt           - Format code"
	@echo "  vet           - Vet code"
	@echo "  dev-setup     - Setup development environment"
	@echo "  dev-cluster   - Show cluster startup commands"
	@echo "  dev           - Full development workflow"
	@echo "  help          - Show this help"
