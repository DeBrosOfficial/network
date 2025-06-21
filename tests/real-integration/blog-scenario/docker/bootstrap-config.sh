#!/bin/sh

echo "Configuring bootstrap IPFS node..."

# Set swarm key for private network
export IPFS_PATH=/root/.ipfs
cp /data/swarm.key $IPFS_PATH/swarm.key

# Configure IPFS for private network
ipfs config --json API.HTTPHeaders.Access-Control-Allow-Origin '["*"]'
ipfs config --json API.HTTPHeaders.Access-Control-Allow-Methods '["PUT", "POST", "GET"]'
ipfs config --json API.HTTPHeaders.Access-Control-Allow-Headers '["Authorization"]'

# Remove default bootstrap nodes (for private network)
ipfs bootstrap rm --all

# Enable experimental features
ipfs config --json Experimental.Libp2pStreamMounting true
ipfs config --json Experimental.P2pHttpProxy true

# Configure addresses
ipfs config Addresses.API "/ip4/0.0.0.0/tcp/5001"
ipfs config Addresses.Gateway "/ip4/0.0.0.0/tcp/8080"
ipfs config --json Addresses.Swarm '["/ip4/0.0.0.0/tcp/4001"]'

# Start IPFS daemon
echo "Starting IPFS daemon..."
exec ipfs daemon --enable-gc --enable-pubsub-experiment