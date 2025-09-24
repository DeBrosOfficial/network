# DeBros Network - Windows Docker Setup Guide

Simple step-by-step guide to run the DeBros Network locally on Windows using Docker.

## Prerequisites

✅ **Docker Desktop for Windows** - Must be installed and running  
✅ **PowerShell** - Comes with Windows  
✅ **Git** - For cloning the repository  

## Step 1: Clone and Build

```powershell
# Clone the repository
git clone https://github.com/DeBrosOfficial/network.git
cd network

# Build the CLI for local testing
go build -ldflags "-X 'main.version=0.50.1-beta'" -o bin/network-cli.exe cmd/cli/main.go
```

## Step 2: Start the Docker Network

```powershell
# Start all services (this will take 1-2 minutes)
docker-compose up -d

# Check that all containers are running
docker-compose ps
```

**Expected output:** 4 containers running:
- `debros-bootstrap` (Bootstrap Node)
- `debros-node-2` (Node 2)  
- `debros-node-3` (Node 3)
- `debros-gateway` (HTTP Gateway)

## Step 3: Wait for Network Initialization

```powershell
# Wait for all services to start up
Start-Sleep -Seconds 30

# Verify bootstrap node is running
docker logs debros-bootstrap --tail 5
```

**Look for:** `"current_peers": 2` (means node-2 and node-3 connected)

## Step 4: Setup CLI Environment

```powershell
# Set these environment variables (required for every new terminal)
$env:DEBROS_GATEWAY_URL="http://localhost:6001"
$BOOTSTRAP_PEER="/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWQX6jcPTVSsBuVuxdkbMbau3DAqZT4pc7UgGh2FvDxrKr"
```

## Step 5: Test the Network

### Basic Health Check
```powershell
# Check if network is healthy
.\bin\network-cli.exe health --bootstrap $BOOTSTRAP_PEER
```
**Expected:** `Status: 🟢 healthy`

### View Connected Peers
```powershell
# List connected peers
.\bin\network-cli.exe peers --bootstrap $BOOTSTRAP_PEER
```
**Expected:** 2 peers (your CLI + bootstrap node) - **This is normal!**

### Test Messaging (Requires Authentication)
```powershell
# Send a message (first time will prompt for wallet authentication)
.\bin\network-cli.exe pubsub publish test-topic "Hello DeBros Network!" --bootstrap $BOOTSTRAP_PEER

# Listen for messages (in another terminal)
.\bin\network-cli.exe pubsub subscribe test-topic 10s --bootstrap $BOOTSTRAP_PEER
```

### Test Database (Requires Authentication)
```powershell
# Query the database
.\bin\network-cli.exe query "SELECT * FROM namespaces" --bootstrap $BOOTSTRAP_PEER
```

## Network Architecture

Your local network consists of:

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Bootstrap     │◄──►│     Node-2       │◄──►│     Node-3      │
│ localhost:4001  │    │ localhost:4002   │    │ localhost:4003  │
│ localhost:5001  │    │ localhost:5002   │    │ localhost:5003  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         ▲                                                        
         │                                                        
         ▼                                                        
┌─────────────────┐    ┌──────────────────┐                      
│   CLI Client    │    │     Gateway      │                      
│ (Your Terminal) │    │ localhost:6001   │                      
└─────────────────┘    └──────────────────┘                      
```

**Ports Used:**
- **4001-4003**: P2P communication between nodes
- **5001-5003**: Database HTTP API  
- **6001**: Gateway HTTP API
- **7001-7003**: Database internal communication

## Daily Usage Commands

### Start Your Development Session
```powershell
# Start the network
docker-compose up -d

# Setup environment (run in every new terminal)
$env:DEBROS_GATEWAY_URL="http://localhost:6001"
$BOOTSTRAP_PEER="/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWQX6jcPTVSsBuVuxdkbMbau3DAqZT4pc7UgGh2FvDxrKr"

# Quick health check
.\bin\network-cli.exe health --bootstrap $BOOTSTRAP_PEER
```

### Common CLI Commands
```powershell
# Network status
.\bin\network-cli.exe status --bootstrap $BOOTSTRAP_PEER

# Send message
.\bin\network-cli.exe pubsub publish my-topic "Hello Network!" --bootstrap $BOOTSTRAP_PEER

# Listen for messages
.\bin\network-cli.exe pubsub subscribe my-topic 15s --bootstrap $BOOTSTRAP_PEER

# Database query
.\bin\network-cli.exe query "SELECT datetime('now') as current_time" --bootstrap $BOOTSTRAP_PEER

# List all database tables
.\bin\network-cli.exe query "SELECT name FROM sqlite_master WHERE type='table'" --bootstrap $BOOTSTRAP_PEER
```

### End Your Development Session
```powershell
# Stop network (keeps all data)
docker-compose stop

# OR stop and remove all data (clean slate)
docker-compose down -v
```

## Troubleshooting

### Check Container Status
```powershell
# View all containers
docker-compose ps

# Check specific container logs
docker logs debros-bootstrap --tail 20
docker logs debros-gateway --tail 20
docker logs debros-node-2 --tail 20
```

### Common Issues and Solutions

**❌ "Connection refused" error**
```powershell
# Solution: Check containers are running and wait longer
docker-compose ps
Start-Sleep -Seconds 60
```

**❌ "Authentication failed"**
```powershell
# Solution: Check gateway is running
docker logs debros-gateway --tail 10
```

**❌ Only seeing 2 peers instead of 4**
```
✅ This is NORMAL! The CLI only sees itself + bootstrap node.
   Node-2 and Node-3 are connected to bootstrap but not visible to CLI.
   Your network is working correctly!
```

### Restart Services
```powershell
# Restart all services
docker-compose restart

# Restart specific service
docker-compose restart bootstrap-node

# Complete clean restart
docker-compose down -v
docker-compose up -d
```

### View Real-time Logs
```powershell
# Follow logs for all services
docker-compose logs -f

# Follow logs for specific service
docker-compose logs -f bootstrap-node
```

## Important Notes

### Authentication
- First CLI command will prompt you to connect your wallet
- You'll receive an API key for future commands
- API key is stored locally and reused automatically

### Network Isolation
- **Your Docker Network**: Completely isolated for development
- **Production Network**: Live DeBros Network (separate)
- **Default CLI**: Connects to production (without `--bootstrap` flag)
- **CLI with `--bootstrap`**: Connects to your local Docker network

### Data Persistence
- All data is stored in Docker volumes
- Survives container restarts
- Use `docker-compose down -v` to reset everything

## Quick Reference Card

```powershell
# Essential daily commands
docker-compose up -d                                    # Start
$env:DEBROS_GATEWAY_URL="http://localhost:6001"        # Setup
$BOOTSTRAP_PEER="/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWQX6jcPTVSsBuVuxdkbMbau3DAqZT4pc7UgGh2FvDxrKr"
.\bin\network-cli.exe health --bootstrap $BOOTSTRAP_PEER # Test
docker-compose down                                     # Stop
```

---

🎉 **Your local DeBros Network is ready for development and testing!**

The network is completely isolated from production and perfect for:
- Testing new features
- Learning the DeBros Network
- Developing applications
- Experimenting safely
