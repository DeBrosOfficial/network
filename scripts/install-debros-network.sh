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
REPO_URL="https://git.debros.io/DeBros/network.git"
MIN_GO_VERSION="1.19"
NODE_PORT="4001"
RQLITE_NODE_PORT="5001"
RAFT_NODE_PORT="7001"
UPDATE_MODE=false
NON_INTERACTIVE=false

log() {
    echo -e "${CYAN}[$(date '+%Y-%m-%d %H:%M:%S')]${NOCOLOR} $1"
}

# Check if running non-interactively (piped from curl)
if [ ! -t 0 ]; then
    NON_INTERACTIVE=true
    log "Running in non-interactive mode"
fi

error() {
    echo -e "${RED}[ERROR]${NOCOLOR} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NOCOLOR} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NOCOLOR} $1"
}

# Check if running as root and warn user
if [[ $EUID -eq 0 ]]; then
    warning "Running as root is not recommended for security reasons."
    if [ "$NON_INTERACTIVE" != true ]; then
        echo -n "Are you sure you want to continue? (yes/no): "
        read ROOT_CONFIRM
        if [[ "$ROOT_CONFIRM" != "yes" ]]; then
            error "Installation cancelled for security reasons."
            exit 1
        fi
    else
        log "Non-interactive mode: proceeding with root (use at your own risk)"
    fi
    # Create sudo alias that does nothing when running as root
    alias sudo=''
else
    # Check if sudo is available for non-root users
    if ! command -v sudo &>/dev/null; then
        error "sudo command not found. Please ensure you have sudo privileges."
        exit 1
    fi
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

# Check if DeBros Network is already installed
check_existing_installation() {
    if [ -d "$INSTALL_DIR" ] && [ -f "$INSTALL_DIR/bin/bootstrap" ] && [ -f "$INSTALL_DIR/bin/node" ]; then
        log "Found existing DeBros Network installation at $INSTALL_DIR"
        
        # Check if services are running
        BOOTSTRAP_RUNNING=false
        NODE_RUNNING=false
        
        if systemctl is-active --quiet debros-bootstrap.service 2>/dev/null; then
            BOOTSTRAP_RUNNING=true
            log "Bootstrap service is currently running"
        fi
        
        if systemctl is-active --quiet debros-node.service 2>/dev/null; then
            NODE_RUNNING=true
            log "Node service is currently running"
        fi
        
        if [ "$NON_INTERACTIVE" = true ]; then
            log "Non-interactive mode: updating existing installation"
            UPDATE_MODE=true
            return 0
        fi
        
        echo -e "${YELLOW}Existing installation detected!${NOCOLOR}"
        echo -e "${CYAN}Options:${NOCOLOR}"
        echo -e "${CYAN}1) Update existing installation${NOCOLOR}"
        echo -e "${CYAN}2) Remove and reinstall${NOCOLOR}"
        echo -e "${CYAN}3) Exit installer${NOCOLOR}"
        
        while true; do
            read -rp "Enter your choice (1, 2, or 3): " EXISTING_CHOICE
            case $EXISTING_CHOICE in
                1)
                    UPDATE_MODE=true
                    log "Will update existing installation"
                    return 0
                    ;;
                2)
                    log "Will remove and reinstall"
                    remove_existing_installation
                    UPDATE_MODE=false
                    return 0
                    ;;
                3)
                    log "Installation cancelled by user"
                    exit 0
                    ;;
                *)
                    error "Invalid choice. Please enter 1, 2, or 3."
                    ;;
            esac
        done
    else
        UPDATE_MODE=false
        return 0
    fi
}

# Remove existing installation
remove_existing_installation() {
    log "Removing existing installation..."
    
    # Stop services if they exist
    for service in debros-bootstrap debros-node; do
        if systemctl list-unit-files | grep -q "$service.service"; then
            log "Stopping $service service..."
            sudo systemctl stop $service.service 2>/dev/null || true
            sudo systemctl disable $service.service 2>/dev/null || true
            sudo rm -f /etc/systemd/system/$service.service
        fi
    done
    
    sudo systemctl daemon-reload
    
    # Remove installation directory
    if [ -d "$INSTALL_DIR" ]; then
        sudo rm -rf "$INSTALL_DIR"
        log "Removed installation directory"
    fi
    
    # Remove debros user
    if id "debros" &>/dev/null; then
        sudo userdel debros 2>/dev/null || true
        log "Removed debros user"
    fi
    
    success "Existing installation removed"
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
    
    # Add Go to system-wide PATH
    if ! grep -q "/usr/local/go/bin" /etc/environment 2>/dev/null; then
        echo 'PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/usr/local/go/bin"' | sudo tee /etc/environment > /dev/null
    fi
    
    # Also add to current user's bashrc for compatibility
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi
    
    # Update current session PATH
    export PATH=$PATH:/usr/local/go/bin
    
    success "Go installed successfully"
}

# Install system dependencies
install_dependencies() {
    log "Checking system dependencies..."
    
    # Check which dependencies are missing
    MISSING_DEPS=()
    
    case $PACKAGE_MANAGER in
        apt)
            # Check for required packages
            for pkg in git make build-essential curl; do
                if ! dpkg -l | grep -q "^ii  $pkg "; then
                    MISSING_DEPS+=($pkg)
                fi
            done
            
            if [ ${#MISSING_DEPS[@]} -gt 0 ]; then
                log "Installing missing dependencies: ${MISSING_DEPS[*]}"
                sudo apt update
                sudo apt install -y "${MISSING_DEPS[@]}"
            else
                success "All system dependencies already installed"
            fi
            ;;
        yum|dnf)
            # Check for required packages
            for pkg in git make curl; do
                if ! rpm -q $pkg &>/dev/null; then
                    MISSING_DEPS+=($pkg)
                fi
            done
            
            # Check for development tools
            if ! rpm -q gcc &>/dev/null; then
                MISSING_DEPS+=("Development Tools")
            fi
            
            if [ ${#MISSING_DEPS[@]} -gt 0 ]; then
                log "Installing missing dependencies: ${MISSING_DEPS[*]}"
                if [[ " ${MISSING_DEPS[*]} " =~ " Development Tools " ]]; then
                    sudo $PACKAGE_MANAGER groupinstall -y "Development Tools"
                fi
                # Remove "Development Tools" from array for individual package installation
                MISSING_DEPS=($(printf '%s\n' "${MISSING_DEPS[@]}" | grep -v "Development Tools"))
                if [ ${#MISSING_DEPS[@]} -gt 0 ]; then
                    sudo $PACKAGE_MANAGER install -y "${MISSING_DEPS[@]}"
                fi
            else
                success "All system dependencies already installed"
            fi
            ;;
    esac
    
    success "System dependencies ready"
}

# Install RQLite
install_rqlite() {
    # Check if RQLite is already installed
    if command -v rqlited &> /dev/null; then
        RQLITE_VERSION=$(rqlited -version | head -n1 | awk '{print $2}')
        log "Found RQLite version: $RQLITE_VERSION"
        success "RQLite already installed"
        return 0
    fi
    
    log "Installing RQLite..."
    
    # Determine architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            RQLITE_ARCH="amd64"
            ;;
        aarch64|arm64)
            RQLITE_ARCH="arm64"
            ;;
        armv7l)
            RQLITE_ARCH="arm"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
    
    # Download and install RQLite
    RQLITE_VERSION="8.30.0"
    RQLITE_TARBALL="rqlite-v${RQLITE_VERSION}-linux-${RQLITE_ARCH}.tar.gz"
    RQLITE_URL="https://github.com/rqlite/rqlite/releases/download/v${RQLITE_VERSION}/${RQLITE_TARBALL}"
    
    cd /tmp
    if ! wget -q "$RQLITE_URL"; then
        error "Failed to download RQLite from $RQLITE_URL"
        exit 1
    fi
    
    # Extract and install RQLite binaries
    tar -xzf "$RQLITE_TARBALL"
    RQLITE_DIR="rqlite-v${RQLITE_VERSION}-linux-${RQLITE_ARCH}"
    
    # Install RQLite binaries to system PATH
    sudo cp "$RQLITE_DIR/rqlited" /usr/local/bin/
    sudo cp "$RQLITE_DIR/rqlite" /usr/local/bin/
    sudo chmod +x /usr/local/bin/rqlited
    sudo chmod +x /usr/local/bin/rqlite
    
    # Cleanup
    rm -rf "$RQLITE_TARBALL" "$RQLITE_DIR"
    
    # Verify installation
    if command -v rqlited &> /dev/null; then
        INSTALLED_VERSION=$(rqlited -version | head -n1 | awk '{print $2}')
        success "RQLite v$INSTALLED_VERSION installed successfully"
    else
        error "RQLite installation failed"
        exit 1
    fi
}

# Check port availability
check_ports() {
    local ports=($NODE_PORT $RQLITE_NODE_PORT $RAFT_NODE_PORT)
    
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

    if [ "$NON_INTERACTIVE" = true ]; then
        log "Non-interactive mode: using default configuration"
        NODE_TYPE="node"
        SOLANA_WALLET="11111111111111111111111111111111"  # Placeholder wallet
        CONFIGURE_FIREWALL="yes"
        log "Node Type: $NODE_TYPE"
        log "Installation Directory: $INSTALL_DIR"
        log "Firewall Configuration: $CONFIGURE_FIREWALL"
        success "Configuration completed with defaults"
        return 0
    fi

# Setting default node type to "node"
    NODE_TYPE="node"

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
    else
        log "User 'debros' already exists"
    fi

    # Create directory structure
    sudo mkdir -p "$INSTALL_DIR"/{bin,configs,keys,data,logs}
    sudo mkdir -p "$INSTALL_DIR/keys/$NODE_TYPE"
    sudo mkdir -p "$INSTALL_DIR/data/$NODE_TYPE"/{rqlite,storage}

    # Set ownership first, then permissions
    sudo chown -R debros:debros "$INSTALL_DIR"
    sudo chmod 755 "$INSTALL_DIR"
    sudo chmod 700 "$INSTALL_DIR/keys"
    sudo chmod 700 "$INSTALL_DIR/keys/$NODE_TYPE"
    
    # Ensure the debros user can write to the keys directory
    sudo chmod 755 "$INSTALL_DIR/data"
    sudo chmod 755 "$INSTALL_DIR/logs"
    sudo chmod 755 "$INSTALL_DIR/configs"
    sudo chmod 755 "$INSTALL_DIR/bin"

    success "Directory structure ready"
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
    local identity_file="$INSTALL_DIR/keys/$NODE_TYPE/identity.key"
    
    if [ -f "$identity_file" ]; then
        if [ "$UPDATE_MODE" = true ]; then
            log "Identity key already exists, keeping existing key"
            success "Using existing node identity"
            return 0
        else
            log "Identity key already exists, regenerating..."
            sudo rm -f "$identity_file"
        fi
    fi

    log "Generating node identity..."

    cd "$INSTALL_DIR/src"

    # Create a custom identity generation script with output path support
    cat > /tmp/generate_identity_custom.go << 'EOF'
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	var outputPath string
	flag.StringVar(&outputPath, "output", "", "Output path for identity key")
	flag.Parse()

	if outputPath == "" {
		fmt.Println("Usage: go run generate_identity_custom.go -output <path>")
		os.Exit(1)
	}

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
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		panic(err)
	}

	// Save identity
	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		panic(err)
	}

	fmt.Printf("Generated Peer ID: %s\n", peerID.String())
	fmt.Printf("Identity saved to: %s\n", outputPath)
}
EOF

    # Ensure Go is in PATH and generate the identity key
    export PATH=$PATH:/usr/local/go/bin
    sudo -u debros env "PATH=$PATH:/usr/local/go/bin" "GOMOD=$(pwd)" go run /tmp/generate_identity_custom.go -output "$identity_file"
    rm /tmp/generate_identity_custom.go

    success "Node identity generated"
}

# Build binaries
build_binaries() {
    log "Building DeBros Network binaries..."

    cd "$INSTALL_DIR/src"
    
    # Ensure Go is in PATH and build all binaries
    export PATH=$PATH:/usr/local/go/bin
    sudo -u debros env "PATH=$PATH:/usr/local/go/bin" make build

    # If in update mode, stop services before copying binaries to avoid "Text file busy" error
    local services_were_running=()
    if [ "$UPDATE_MODE" = true ]; then
        log "Update mode: checking for running services before binary update..."
        
        for service in debros-bootstrap debros-node; do
            if systemctl is-active --quiet $service.service 2>/dev/null; then
                log "Stopping $service service to update binaries..."
                sudo systemctl stop $service.service
                services_were_running+=("$service")
            fi
        done
        
        # Give services a moment to fully stop
        if [ ${#services_were_running[@]} -gt 0 ]; then
            log "Waiting for services to stop completely..."
            sleep 3
        fi
    fi

    # Copy binaries to installation directory
    sudo cp bin/* "$INSTALL_DIR/bin/"
    sudo chown debros:debros "$INSTALL_DIR/bin/"*

    # If in update mode and services were running, restart them
    if [ "$UPDATE_MODE" = true ] && [ ${#services_were_running[@]} -gt 0 ]; then
        log "Restarting previously running services..."
        for service in "${services_were_running[@]}"; do
            log "Starting $service service..."
            sudo systemctl start $service.service
        done
    fi

    success "Binaries built and installed"
}

# Generate configuration files
generate_configs() {
    log "Generating configuration files..."

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

    sudo mv /tmp/config.yaml "$INSTALL_DIR/configs/$NODE_TYPE.yaml"
    sudo chown debros:debros "$INSTALL_DIR/configs/$NODE_TYPE.yaml"

    success "Configuration files generated"
}

# Configure firewall
configure_firewall() {
    if [[ "$CONFIGURE_FIREWALL" == "yes" ]]; then
        log "Configuring firewall rules..."

        if command -v ufw &> /dev/null; then
            # Add firewall rules regardless of UFW status
            # This allows the rules to be ready when UFW is enabled
            log "Adding UFW rules for DeBros Network ports..."
            
            # Add ports for node
            for port in $NODE_PORT $RQLITE_NODE_PORT $RAFT_NODE_PORT; do
                if ! sudo ufw allow $port; then
                    error "Failed to allow port $port"
                    exit 1
                fi
                log "Added UFW rule: allow port $port"
            done
            
            # Check UFW status and inform user
            UFW_STATUS=$(sudo ufw status | grep -o "Status: [a-z]\+" | awk '{print $2}' || echo "inactive")
            
            if [[ "$UFW_STATUS" == "active" ]]; then
                success "Firewall rules added and active"
            else
                success "Firewall rules added (UFW is inactive - rules will take effect when UFW is enabled)"
                log "To enable UFW with current rules: sudo ufw enable"
            fi
        else
            warning "UFW not found. Please configure firewall manually."
            log "Required ports to allow:"
            log "  - Port $NODE_PORT (Node)"
            log "  - Port $RQLITE_NODE_PORT (RQLite)"
            log "  - Port $RAFT_NODE_PORT (Raft)"
        fi
    fi
}

# Create systemd service
create_systemd_service() {
    local service_file="/etc/systemd/system/debros-$NODE_TYPE.service"
    
    # Always clean up any existing service files to ensure fresh start
    for service in debros-bootstrap debros-node; do
        if [ -f "/etc/systemd/system/$service.service" ]; then
            log "Cleaning up existing $service service..."
            sudo systemctl stop $service.service 2>/dev/null || true
            sudo systemctl disable $service.service 2>/dev/null || true
            sudo rm -f /etc/systemd/system/$service.service
        fi
    done
    
    sudo systemctl daemon-reload
    log "Creating new systemd service..."

    # Determine the correct ExecStart command based on node type
    local exec_start=""
exec_start="$INSTALL_DIR/bin/node -data $INSTALL_DIR/data/node -port $NODE_PORT"

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
ExecStart=$exec_start
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

    sudo mv /tmp/debros-$NODE_TYPE.service "$service_file"
    sudo systemctl daemon-reload
    sudo systemctl enable debros-$NODE_TYPE.service

    success "Systemd service ready"
}

# Start the service
start_service() {
    log "Starting DeBros Network service..."

    sudo systemctl start debros-$NODE_TYPE.service
    sleep 3

    if systemctl is-active --quiet debros-$NODE_TYPE.service; then
        success "DeBros Network service started successfully"
    else
        error "Failed to start DeBros Network service"
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
    check_existing_installation
    
    # Skip port check in update mode since services are already running
    if [ "$UPDATE_MODE" != true ]; then
        check_ports
    else
        log "Update mode: skipping port availability check"
    fi
    
    # Check and install Go if needed
    if ! check_go_installation; then
        install_go
    fi
    
    install_dependencies
    install_rqlite
    
    # Skip configuration wizard in update mode
    if [ "$UPDATE_MODE" != true ]; then
        configuration_wizard
    else
        log "Update mode: skipping configuration wizard"
        # Force node type to 'node' for consistent terminology
        NODE_TYPE="node"
        log "Using node type: $NODE_TYPE (standardized from any previous bootstrap configuration)"
    fi
    
    setup_directories
    setup_source_code
    generate_identity
    build_binaries
    
    # Only generate new configs if not in update mode
    if [ "$UPDATE_MODE" != true ]; then
        generate_configs
        configure_firewall
    else
        log "Update mode: keeping existing configuration"
    fi
    
    create_systemd_service
    start_service

    # Display completion information
    log "${BLUE}==================================================${NOCOLOR}"
    if [ "$UPDATE_MODE" = true ]; then
        log "${GREEN}        Update Complete!                          ${NOCOLOR}"
    else
        log "${GREEN}        Installation Complete!                    ${NOCOLOR}"
    fi
    log "${BLUE}==================================================${NOCOLOR}"
    
    log "${GREEN}Installation Directory:${NOCOLOR} ${CYAN}$INSTALL_DIR${NOCOLOR}"
    log "${GREEN}Configuration:${NOCOLOR} ${CYAN}$INSTALL_DIR/configs/$NODE_TYPE.yaml${NOCOLOR}"
    log "${GREEN}Logs:${NOCOLOR} ${CYAN}$INSTALL_DIR/logs/$NODE_TYPE.log${NOCOLOR}"
    
    log "${GREEN}Node Port:${NOCOLOR} ${CYAN}$NODE_PORT${NOCOLOR}"
    log "${GREEN}RQLite Port:${NOCOLOR} ${CYAN}$RQLITE_NODE_PORT${NOCOLOR}"
    log "${GREEN}Raft Port:${NOCOLOR} ${CYAN}$RAFT_NODE_PORT${NOCOLOR}"

    log "${BLUE}==================================================${NOCOLOR}"
    log "${GREEN}Management Commands:${NOCOLOR}"
    log "${CYAN}  - sudo systemctl status debros-$NODE_TYPE${NOCOLOR} (Check status)"
    log "${CYAN}  - sudo systemctl restart debros-$NODE_TYPE${NOCOLOR} (Restart service)"
    log "${CYAN}  - sudo systemctl stop debros-$NODE_TYPE${NOCOLOR} (Stop service)"
    log "${CYAN}  - sudo systemctl start debros-$NODE_TYPE${NOCOLOR} (Start service)"
    log "${CYAN}  - sudo journalctl -u debros-$NODE_TYPE.service -f${NOCOLOR} (View logs)"
    log "${CYAN}  - $INSTALL_DIR/bin/network-cli${NOCOLOR} (Use CLI tools)"
    log "${BLUE}==================================================${NOCOLOR}"

    if [ "$UPDATE_MODE" = true ]; then
        success "DeBros Network has been updated and is running!"
    else
        success "DeBros Network is now running!"
    fi
    log "${CYAN}For documentation visit: https://docs.debros.io${NOCOLOR}"
}

# Run main function
main "$@"
