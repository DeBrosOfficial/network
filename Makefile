# Network - Distributed P2P Database System
# Makefile for development and build tasks

.PHONY: build clean test run-bootstrap run-node run-example deps tidy fmt vet

# Build targets
build: deps
	@echo "Building network executables..."
	@mkdir -p bin
	go build -o bin/bootstrap cmd/bootstrap/main.go
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

# Run bootstrap node
run-bootstrap:
	@echo "Starting bootstrap node..."
	go run cmd/bootstrap/main.go -port 4001 -data ./data/bootstrap

# Run regular node
run-node:
	@echo "Starting regular node..."
	go run cmd/node/main.go -data ./data/node

# Show current bootstrap configuration
show-bootstrap:
	@echo "Current bootstrap configuration from .env:"
	@cat .env 2>/dev/null || echo "No .env file found - using defaults"

# Run example
run-example: 
	@echo "Running basic usage example..."
	go run examples/basic_usage.go

# Build Anchat
build-anchat:
	@echo "Building Anchat..."
	cd anchat && go build -o bin/anchat cmd/cli/main.go

# Run Anchat demo
run-anchat:
	@echo "Starting Anchat demo..."
	cd anchat && go run cmd/cli/main.go demo_user

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
	@mkdir -p data/bootstrap data/node1 data/node2
	@mkdir -p data/test-bootstrap data/test-node1 data/test-node2
	@mkdir -p anchat/bin
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
	@echo "1. make run-bootstrap      # Start bootstrap node"
	@echo "2. make run-node           # Start regular node (auto-loads bootstrap from .env)"
	@echo "3. make run-example        # Test basic functionality"
	@echo "4. make run-anchat         # Start messaging app"
	@echo "5. make show-bootstrap     # Check bootstrap configuration"
	@echo "6. make cli-health         # Check network health"
	@echo "7. make cli-peers          # List peers"
	@echo "8. make cli-storage-test   # Test storage"
	@echo "9. make cli-pubsub-test    # Test messaging"

# Full development workflow
dev: clean build build-anchat test
	@echo "Development workflow complete!"

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build all executables"
	@echo "  build-anchat  - Build Anchat application"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  run-bootstrap - Start bootstrap node"
	@echo "  run-node      - Start regular node (auto-loads bootstrap from .env)"
	@echo "  run-example   - Run usage example"
	@echo "  run-anchat    - Run Anchat demo"
	@echo "  run-cli       - Run network CLI help"
	@echo "  show-bootstrap - Show current bootstrap configuration"
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
