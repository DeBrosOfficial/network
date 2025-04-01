import { config } from '../../config';

// Determine the IPFS port to use
export const getIpfsPort = (): number => {
  if (process.env.IPFS_PORT) {
    return parseInt(process.env.IPFS_PORT);
  }
  const httpPort = parseInt(process.env.PORT || '7777');
  return httpPort + 1; // Default to HTTP port + 1
};

// Get a node-specific blockstore path
export const getBlockstorePath = (): string => {
  const basePath = config.ipfs.blockstorePath;
  const fingerprint = config.env.fingerprint;
  return `${basePath}-${fingerprint}`;
};

// IPFS configuration
export const ipfsConfig = {
  blockstorePath: getBlockstorePath(),
  port: getIpfsPort(),
  serviceDiscovery: {
    topic: config.ipfs.serviceDiscovery.topic,
    heartbeatInterval: config.ipfs.serviceDiscovery.heartbeatInterval || 2000,
    staleTimeout: config.ipfs.serviceDiscovery.staleTimeout || 30000,
    logInterval: config.ipfs.serviceDiscovery.logInterval || 60000,
    publicAddress: config.ipfs.serviceDiscovery.publicAddress,
  },
  bootstrapNodes: process.env.BOOTSTRAP_NODES,
};
