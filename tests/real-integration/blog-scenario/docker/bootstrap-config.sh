#!/bin/sh

echo "Configuring bootstrap IPFS node..."

# Set IPFS path
export IPFS_PATH=/data/ipfs

# Copy swarm key for private network
if [ -f "/data/ipfs/swarm.key" ]; then
    echo "Using existing swarm key"
else
    echo "Swarm key not found"
    exit 1
fi

# Configure IPFS for private network
ipfs config --json API.HTTPHeaders.Access-Control-Allow-Origin '["*"]'
ipfs config --json API.HTTPHeaders.Access-Control-Allow-Methods '["PUT", "POST", "GET"]'
ipfs config --json API.HTTPHeaders.Access-Control-Allow-Headers '["Authorization"]'

# Remove default bootstrap nodes (for private network)
ipfs bootstrap rm --all

# Configure addresses
ipfs config Addresses.API "/ip4/0.0.0.0/tcp/5001"
ipfs config Addresses.Gateway "/ip4/0.0.0.0/tcp/8080"
ipfs config --json Addresses.Swarm '["/ip4/0.0.0.0/tcp/4001"]'

# Enable PubSub
ipfs config --json Pubsub.Enabled true

echo "Starting IPFS daemon..."
exec ipfs daemon --enable-gc