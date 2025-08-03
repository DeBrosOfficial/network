#!/bin/bash

set -e  # Exit on any error
trap 'echo -e "${RED}An error occurred. Installation aborted.${NOCOLOR}"; exit 1' ERR

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BLUE='\033[38;2;2;128;175m'
YELLOW='\033[1;33m'
NOCOLOR='\033[0m'

# Default values
INSTALL_DIR="/opt/debros"
REPO_URL="https://github.com/DeBrosOfficial/debros-network.git"
MIN_GO_VERSION="1.19"
BOOTSTRAP_PORT="4001"
NODE_PORT="4002"
RQLITE_BOOTSTRAP_PORT="5001"
RQLITE_NODE_PORT="5002"
RAFT_BOOTSTRAP_PORT="7001"
RAFT_NODE_PORT="7002"

log() {
    echo -e "${CYAN}[$(date '+%Y-%m-%d %H:%M:%S')]${NOCOLOR} $1"
}

error() {
    echo -e "${RED}[ERROR]${NOCOLOR} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NOCOLOR} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NOCOLOR} $1"
}

# Check if running as root
if [[ $EUID -eq 0 ]]; then
    error "This script should not be run as root. Please run as a regular user with sudo privileges."
    exit 1
fi

# Check if sudo is available
if ! command -v sudo &>/dev/null; then
    error "sudo command not found. Please ensure you have sudo privileges."
    exit 1
fi

# Detect OS
detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    else
        error "Cannot detect operating system"
        exit 1
    fi

    case $OS in
        ubuntu|debian)
            PACKAGE_MANAGER="apt"
            ;;
        centos|rhel|fedora)
            PACKAGE_MANAGER="yum"
            if command -v dnf &> /dev/null; then
                PACKAGE_MANAGER="dnf"
            fi
            ;;
        *)
            error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    log "Detected OS: $OS $VERSION"
}

# Check Go installation and version
check_go_installation() {
    if command -v go &> /dev/null; then
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        log "Found Go version: $GO_VERSION"
        
        # Compare versions (simplified)
        if [ "$(printf '%s\n' "$MIN_GO_VERSION" "$GO_VERSION" | sort -V | head -n1)" = "$MIN_GO_VERSION" ]; then
            success "Go version is sufficient"
            return 0
        else
            warning "Go version $GO_VERSION is too old. Minimum required: $MIN_GO_VERSION"
            return 1
        fi
    else
        log "Go not found on system"
        return 1
    fi
}

# Install Go
install_go() {
    log "Installing Go..."
    
    case $PACKAGE_MANAGER in
        apt)
            sudo apt update
            sudo apt install -y wget
            ;;
        yum|dnf)
            sudo $PACKAGE_MANAGER install -y wget
            ;;
    esac

    # Download and install Go
    GO_TARBALL="go1.21.0.linux-amd64.tar.gz"
    ARCH=$(uname -m)
    
    if [ "$ARCH" = "aarch64" ]; then
        GO_TARBALL="go1.21.0.linux-arm64.tar.gz"
    fi

    cd /tmp
    wget -q "https://golang.org/dl/$GO_TARBALL"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "$GO_TARBALL"
    
    # Add Go to PATH
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    
    export PATH=$PATH:/usr/local/go/bin
    success "Go installed successfully"
}

# Install system dependencies
install_dependencies() {
    log "Installing system dependencies..."
    
    case $PACKAGE_MANAGER in
        apt)
            sudo apt update
            sudo apt install -y git make build-essential curl
            ;;
        yum|dnf)
            sudo $PACKAGE_MANAGER groupinstall -y "Development Tools"
            sudo $PACKAGE_MANAGER install -y git make curl
            ;;
    esac
    
    success "System dependencies installed"
}

# Check port availability
check_ports() {
    local ports=($BOOTSTRAP_PORT $NODE_PORT $RQLITE_BOOTSTRAP_PORT $RQLITE_NODE_PORT $RAFT_BOOTSTRAP_PORT $RAFT_NODE_PORT)
    
    for port in "${ports[@]}"; do
        if sudo netstat -tuln 2>/dev/null | grep -q ":$port " || ss -tuln 2>/dev/null | grep -q ":$port "; then
            error "Port $port is already in use. Please free it up and try again."
            exit 1
        fi
    done
    
    success "All required ports are available"
}

# Configuration wizard
configuration_wizard() {
    log "${BLUE}==================================================${NOCOLOR}"
    log "${GREEN}        DeBros Network Configuration Wizard       ${NOCOLOR}"
    log "${BLUE}==================================================${NOCOLOR}"

    # Node type selection
    while true; do
        echo -e "${GREEN}Select node type:${NOCOLOR}"
        echo -e "${CYAN}1) Bootstrap Node (Network entry point)${NOCOLOR}"
        echo -e "${CYAN}2) Regular Node (Connects to existing network)${NOCOLOR}"
        read -rp "Enter your choice (1 or 2): " NODE_TYPE_CHOICE
        
        case $NODE_TYPE_CHOICE in
            1)
                NODE_TYPE="bootstrap"
                break
                ;;
            2)
                NODE_TYPE="regular"
                break
                ;;
            *)
                error "Invalid choice. Please enter 1 or 2."
                ;;
        esac
    done

    # Solana wallet address
    log "${GREEN}Enter your Solana wallet address to be eligible for node operator rewards:${NOCOLOR}"
    while true; do
        read -rp "Solana Wallet Address: " SOLANA_WALLET
        if [[ -n "$SOLANA_WALLET" && ${#SOLANA_WALLET} -ge 32 ]]; then
            break
        else
            error "Please enter a valid Solana wallet address"
        fi
    done

    # Data directory
    read -rp "Installation directory [default: $INSTALL_DIR]: " CUSTOM_INSTALL_DIR
    if [[ -n "$CUSTOM_INSTALL_DIR" ]]; then
        INSTALL_DIR="$CUSTOM_INSTALL_DIR"
    fi

    # Firewall configuration
    read -rp "Configure firewall automatically? (yes/no) [default: yes]: " CONFIGURE_FIREWALL
    CONFIGURE_FIREWALL="${CONFIGURE_FIREWALL:-yes}"

    success "Configuration completed"
}

# Create user and directories
setup_directories() {
    log "Setting up directories and permissions..."

    # Create debros user if it doesn't exist
    if ! id "debros" &>/dev/null; then
        sudo useradd -r -s /bin/false -d "$INSTALL_DIR" debros
        log "Created debros user"
    fi

    # Create directory structure
    sudo mkdir -p "$INSTALL_DIR"/{bin,configs,keys,data,logs}
    sudo mkdir -p "$INSTALL_DIR/keys/$NODE_TYPE"
    sudo mkdir -p "$INSTALL_DIR/data/$NODE_TYPE"/{rqlite,storage}

    # Set ownership and permissions
    sudo chown -R debros:debros "$INSTALL_DIR"
    sudo chmod 755 "$INSTALL_DIR"
    sudo chmod 700 "$INSTALL_DIR/keys"
    sudo chmod 600 "$INSTALL_DIR/keys/$NODE_TYPE" 2>/dev/null || true

    success "Directory structure created"
}

# Clone or update repository
setup_source_code() {
    log "Setting up source code..."

    if [ -d "$INSTALL_DIR/src" ]; then
        log "Updating existing repository..."
        cd "$INSTALL_DIR/src"
        sudo -u debros git pull
    else
        log "Cloning repository..."
        sudo -u debros git clone "$REPO_URL" "$INSTALL_DIR/src"
        cd "$INSTALL_DIR/src"
    fi

    success "Source code ready"
}

# Generate identity key
generate_identity() {
    log "Generating node identity..."

    cd "$INSTALL_DIR/src"

    # Create a temporary Go program for key generation
    cat > /tmp/generate_identity.go << 'EOF'
package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run generate_identity.go <key_file_path>")
		os.Exit(1)
	}

	keyFile := os.Args[1]

	// Generate identity
	priv, pub, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
	if err != nil {
		panic(err)
	}

	// Get peer ID
	peerID, err := peer.IDFromPublicKey(pub)
	if err != nil {
		panic(err)
	}

	// Marshal private key
	data, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		panic(err)
	}

	// Create directory
	if err := os.MkdirAll(filepath.Dir(keyFile), 0700); err != nil {
		panic(err)
	}

	// Save identity
	if err := os.WriteFile(keyFile, data, 0600); err != nil {
		panic(err)
	}

	fmt.Printf("Generated Peer ID: %s\n", peerID.String())
	fmt.Printf("Identity saved to: %s\n", keyFile)
}
EOF

    # Generate the identity key
    sudo -u debros go run /tmp/generate_identity.go "$INSTALL_DIR/keys/$NODE_TYPE/identity.key"
    rm /tmp/generate_identity.go

    success "Node identity generated"
}

# Build binaries
build_binaries() {
    log "Building DeBros Network binaries..."

    cd "$INSTALL_DIR/src"
    
    # Build all binaries
    sudo -u debros make build

    # Copy binaries to installation directory
    sudo cp bin/* "$INSTALL_DIR/bin/"
    sudo chown debros:debros "$INSTALL_DIR/bin/"*

    success "Binaries built and installed"
}

# Generate configuration files
generate_configs() {
    log "Generating configuration files..."

    if [ "$NODE_TYPE" = "bootstrap" ]; then
        cat > /tmp/config.yaml << EOF
node:
  data_dir: "$INSTALL_DIR/data/bootstrap"
  key_file: "$INSTALL_DIR/keys/bootstrap/identity.key"
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/$BOOTSTRAP_PORT"
  solana_wallet: "$SOLANA_WALLET"

database:
  rqlite_port: $RQLITE_BOOTSTRAP_PORT
  rqlite_raft_port: $RAFT_BOOTSTRAP_PORT

logging:
  level: "info"
  file: "$INSTALL_DIR/logs/bootstrap.log"
EOF
    else
        cat > /tmp/config.yaml << EOF
node:
  data_dir: "$INSTALL_DIR/data/node"
  key_file: "$INSTALL_DIR/keys/node/identity.key"
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/$NODE_PORT"
  solana_wallet: "$SOLANA_WALLET"

database:
  rqlite_port: $RQLITE_NODE_PORT
  rqlite_raft_port: $RAFT_NODE_PORT

logging:
  level: "info"
  file: "$INSTALL_DIR/logs/node.log"
EOF
    fi

    sudo mv /tmp/config.yaml "$INSTALL_DIR/configs/$NODE_TYPE.yaml"
    sudo chown debros:debros "$INSTALL_DIR/configs/$NODE_TYPE.yaml"

    success "Configuration files generated"
}

# Configure firewall
configure_firewall() {
    if [[ "$CONFIGURE_FIREWALL" == "yes" ]]; then
        log "Configuring firewall..."

        if command -v ufw &> /dev/null; then
            if [ "$NODE_TYPE" = "bootstrap" ]; then
                sudo ufw allow $BOOTSTRAP_PORT
                sudo ufw allow $RQLITE_BOOTSTRAP_PORT
                sudo ufw allow $RAFT_BOOTSTRAP_PORT
            else
                sudo ufw allow $NODE_PORT
                sudo ufw allow $RQLITE_NODE_PORT
                sudo ufw allow $RAFT_NODE_PORT
            fi
            
            # Enable ufw if not already active
            UFW_STATUS=$(sudo ufw status | grep -o "Status: [a-z]*" | awk '{print $2}' || echo "inactive")
            if [[ "$UFW_STATUS" != "active" ]]; then
                echo "y" | sudo ufw enable
            fi
            
            success "Firewall configured"
        else
            warning "UFW not found. Please configure firewall manually."
        fi
    fi
}

# Create systemd service
create_systemd_service() {
    log "Creating systemd service..."

    cat > /tmp/debros-$NODE_TYPE.service << EOF
[Unit]
Description=DeBros Network $NODE_TYPE Node
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=debros
Group=debros
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/bin/$NODE_TYPE -config $INSTALL_DIR/configs/$NODE_TYPE.yaml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=debros-$NODE_TYPE

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=$INSTALL_DIR

[Install]
WantedBy=multi-user.target
EOF

    sudo mv /tmp/debros-$NODE_TYPE.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable debros-$NODE_TYPE.service

    success "Systemd service created and enabled"
}

# Start the service
start_service() {
    log "Starting DeBros Network $NODE_TYPE node..."

    sudo systemctl start debros-$NODE_TYPE.service
    sleep 3

    if systemctl is-active --quiet debros-$NODE_TYPE.service; then
        success "DeBros Network $NODE_TYPE node started successfully"
    else
        error "Failed to start DeBros Network $NODE_TYPE node"
        log "Check logs with: sudo journalctl -u debros-$NODE_TYPE.service"
        exit 1
    fi
}

# Display banner
display_banner() {
    echo -e "${BLUE}========================================================================${NOCOLOR}"
    echo -e "${CYAN}
    ____       ____                   _   _      _                      _    
   |  _ \  ___| __ ) _ __ ___  ___   | \ | | ___| |___      _____  _ __| | __
   | | | |/ _ \  _ \|  __/ _ \/ __|  |  \| |/ _ \ __\ \ /\ / / _ \|  __| |/ /
   | |_| |  __/ |_) | | | (_) \__ \  | |\  |  __/ |_ \ V  V / (_) | |  |   < 
   |____/ \___|____/|_|  \___/|___/  |_| \_|\___|\__| \_/\_/ \___/|_|  |_|\_\\
${NOCOLOR}"
    echo -e "${BLUE}========================================================================${NOCOLOR}"
}

# Main installation function
main() {
    display_banner

    log "${BLUE}==================================================${NOCOLOR}"
    log "${GREEN}        Starting DeBros Network Installation      ${NOCOLOR}"
    log "${BLUE}==================================================${NOCOLOR}"

    detect_os
    check_ports
    
    # Check and install Go if needed
    if ! check_go_installation; then
        install_go
    fi
    
    install_dependencies
    configuration_wizard
    setup_directories
    setup_source_code
    generate_identity
    build_binaries
    generate_configs
    configure_firewall
    create_systemd_service
    start_service

    # Display completion information
    log "${BLUE}==================================================${NOCOLOR}"
    log "${GREEN}        Installation Complete!                    ${NOCOLOR}"
    log "${BLUE}==================================================${NOCOLOR}"
    
    log "${GREEN}Node Type:${NOCOLOR} ${CYAN}$NODE_TYPE${NOCOLOR}"
    log "${GREEN}Installation Directory:${NOCOLOR} ${CYAN}$INSTALL_DIR${NOCOLOR}"
    log "${GREEN}Configuration:${NOCOLOR} ${CYAN}$INSTALL_DIR/configs/$NODE_TYPE.yaml${NOCOLOR}"
    log "${GREEN}Logs:${NOCOLOR} ${CYAN}$INSTALL_DIR/logs/$NODE_TYPE.log${NOCOLOR}"
    
    if [ "$NODE_TYPE" = "bootstrap" ]; then
        log "${GREEN}Bootstrap Port:${NOCOLOR} ${CYAN}$BOOTSTRAP_PORT${NOCOLOR}"
        log "${GREEN}RQLite Port:${NOCOLOR} ${CYAN}$RQLITE_BOOTSTRAP_PORT${NOCOLOR}"
        log "${GREEN}Raft Port:${NOCOLOR} ${CYAN}$RAFT_BOOTSTRAP_PORT${NOCOLOR}"
    else
        log "${GREEN}Node Port:${NOCOLOR} ${CYAN}$NODE_PORT${NOCOLOR}"
        log "${GREEN}RQLite Port:${NOCOLOR} ${CYAN}$RQLITE_NODE_PORT${NOCOLOR}"
        log "${GREEN}Raft Port:${NOCOLOR} ${CYAN}$RAFT_NODE_PORT${NOCOLOR}"
    fi

    log "${BLUE}==================================================${NOCOLOR}"
    log "${GREEN}Management Commands:${NOCOLOR}"
    log "${CYAN}  - sudo systemctl status debros-$NODE_TYPE${NOCOLOR} (Check status)"
    log "${CYAN}  - sudo systemctl restart debros-$NODE_TYPE${NOCOLOR} (Restart service)"
    log "${CYAN}  - sudo systemctl stop debros-$NODE_TYPE${NOCOLOR} (Stop service)"
    log "${CYAN}  - sudo systemctl start debros-$NODE_TYPE${NOCOLOR} (Start service)"
    log "${CYAN}  - sudo journalctl -u debros-$NODE_TYPE.service -f${NOCOLOR} (View logs)"
    log "${CYAN}  - $INSTALL_DIR/bin/cli${NOCOLOR} (Use CLI tools)"
    log "${BLUE}==================================================${NOCOLOR}"

    success "DeBros Network $NODE_TYPE node is now running!"
    log "${CYAN}For documentation visit: https://docs.debros.io${NOCOLOR}"
}

# Run main function
main "$@"
