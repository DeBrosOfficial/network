#!/bin/bash
set -euo pipefail

# DeBros Network Production Security Setup
# This script configures secure RQLite clustering with authentication

DEBROS_DIR="/opt/debros"
CONFIG_DIR="$DEBROS_DIR/configs"
KEYS_DIR="$DEBROS_DIR/keys"

echo "🔐 Setting up DeBros Network Production Security..."

# Create security directories
sudo mkdir -p "$CONFIG_DIR" "$KEYS_DIR"
sudo chown debros:debros "$CONFIG_DIR" "$KEYS_DIR"
sudo chmod 750 "$KEYS_DIR"

# Generate cluster authentication credentials
CLUSTER_USER="debros_cluster"
CLUSTER_PASS=$(openssl rand -base64 32)
API_USER="debros_api" 
API_PASS=$(openssl rand -base64 32)

echo "🔑 Generated cluster credentials:"
echo "  Cluster User: $CLUSTER_USER"
echo "  API User: $API_USER"

# Create RQLite users configuration
cat > "$CONFIG_DIR/rqlite-users.json" << EOF
[
  {
    "username": "$CLUSTER_USER",
    "password": "$CLUSTER_PASS",
    "perms": ["*"]
  },
  {
    "username": "$API_USER", 
    "password": "$API_PASS",
    "perms": ["status", "ready", "nodes", "db:*"]
  }
]
EOF

sudo chown debros:debros "$CONFIG_DIR/rqlite-users.json"
sudo chmod 600 "$CONFIG_DIR/rqlite-users.json"

# Store credentials securely
cat > "$KEYS_DIR/rqlite-cluster-auth" << EOF
RQLITE_CLUSTER_USER="$CLUSTER_USER"
RQLITE_CLUSTER_PASS="$CLUSTER_PASS"
RQLITE_API_USER="$API_USER"
RQLITE_API_PASS="$API_PASS"
EOF

sudo chown debros:debros "$KEYS_DIR/rqlite-cluster-auth"
sudo chmod 600 "$KEYS_DIR/rqlite-cluster-auth"

# Configure firewall for production
echo "🛡️ Configuring production firewall rules..."

# Reset UFW to defaults
sudo ufw --force reset

# Default policies
sudo ufw default deny incoming
sudo ufw default allow outgoing

# SSH (adjust port as needed)
sudo ufw allow 22/tcp comment "SSH"

# LibP2P P2P networking (public, encrypted)
sudo ufw allow 4001/tcp comment "LibP2P P2P"
sudo ufw allow 4001/udp comment "LibP2P QUIC"

# RQLite ports (restrict to cluster IPs only)
BOOTSTRAP_IPS=("57.129.81.31" "38.242.250.186")
for ip in "${BOOTSTRAP_IPS[@]}"; do
    sudo ufw allow from "$ip" to any port 5001 comment "RQLite HTTP from $ip"
    sudo ufw allow from "$ip" to any port 7001 comment "RQLite Raft from $ip"
done

# Enable firewall
sudo ufw --force enable

echo "🔧 Configuring RQLite cluster authentication..."

# Update RQLite join addresses with authentication
AUTHENTICATED_JOIN_ADDRESS="http://$CLUSTER_USER:$CLUSTER_PASS@57.129.81.31:5001"

# Create environment file for authenticated connections
cat > "$CONFIG_DIR/rqlite-env" << EOF
# RQLite cluster authentication
RQLITE_JOIN_AUTH_USER="$CLUSTER_USER"
RQLITE_JOIN_AUTH_PASS="$CLUSTER_PASS"
RQLITE_JOIN_ADDRESS_AUTH="$AUTHENTICATED_JOIN_ADDRESS"
EOF

sudo chown debros:debros "$CONFIG_DIR/rqlite-env"
sudo chmod 600 "$CONFIG_DIR/rqlite-env"

# Create connection helper script
cat > "$DEBROS_DIR/bin/rqlite-connect" << 'EOF'
#!/bin/bash
# Helper script for authenticated RQLite connections

source /opt/debros/keys/rqlite-cluster-auth

if [ "$1" = "cluster" ]; then
    rqlite -H localhost -p 5001 -u "$RQLITE_CLUSTER_USER" -p "$RQLITE_CLUSTER_PASS"
elif [ "$1" = "api" ]; then
    rqlite -H localhost -p 5001 -u "$RQLITE_API_USER" -p "$RQLITE_API_PASS"
else
    echo "Usage: $0 {cluster|api}"
    exit 1
fi
EOF

sudo chown debros:debros "$DEBROS_DIR/bin/rqlite-connect"
sudo chmod 755 "$DEBROS_DIR/bin/rqlite-connect"

echo "✅ Production security setup complete!"
echo ""
echo "📋 Security Summary:"
echo "  - RQLite authentication enabled"
echo "  - Firewall configured with IP restrictions"
echo "  - Cluster credentials generated and stored"
echo "  - Port 4001: Public LibP2P (encrypted P2P)"
echo "  - Port 5001/7001: RQLite cluster (IP-restricted)"
echo ""
echo "🔐 Credentials stored in:"
echo "  - Users: $CONFIG_DIR/rqlite-users.json"
echo "  - Auth: $KEYS_DIR/rqlite-cluster-auth"
echo ""
echo "🔌 Connect to RQLite:"
echo "  - Cluster admin: $DEBROS_DIR/bin/rqlite-connect cluster"
echo "  - API access: $DEBROS_DIR/bin/rqlite-connect api"
echo ""
echo "⚠️  IMPORTANT: Save these credentials securely!"
echo "   Cluster User: $CLUSTER_USER"
echo "   Cluster Pass: $CLUSTER_PASS"
