// Config exports
import { config, defaultConfig, type DebrosConfig } from './config';
import { validateConfig, type ValidationResult } from './ipfs/config/configValidator';

// IPFS exports
import ipfsService, {
  init as initIpfs,
  stop as stopIpfs,
  getHelia,
  getProxyAgent,
  getInstance,
  getLibp2p,
  getConnectedPeers,
  getOptimalPeer,
  updateNodeLoad,
  logPeersStatus,
  type IPFSModule,
} from './ipfs/ipfsService';

import { ipfsConfig, getIpfsPort, getBlockstorePath } from './ipfs/config/ipfsConfig';

// OrbitDB exports
import orbitDBService, {
  init as initOrbitDB,
  openDB,
  getOrbitDB,
  db as orbitDB,
  getOrbitDBDir,
  getDBAddress,
  saveDBAddress,
} from './orbit/orbitDBService';

import loadBalancerControllerDefault from './ipfs/loadBalancerController';
export const loadBalancerController = loadBalancerControllerDefault;

// Logger exports
import logger, { createServiceLogger, createDebrosLogger, type LoggerOptions } from './utils/logger';

// Crypto exports
import { getPrivateKey } from './ipfs/utils/crypto';

// Export everything
export {
  // Config
  config,
  defaultConfig,
  validateConfig,
  type DebrosConfig,
  type ValidationResult,

  // IPFS
  ipfsService,
  initIpfs,
  stopIpfs,
  getHelia,
  getProxyAgent,
  getInstance,
  getLibp2p,
  getConnectedPeers,
  getOptimalPeer,
  updateNodeLoad,
  logPeersStatus,
  type IPFSModule,

  // IPFS Config
  ipfsConfig,
  getIpfsPort,
  getBlockstorePath,

  // OrbitDB
  orbitDBService,
  initOrbitDB,
  openDB,
  getOrbitDB,
  orbitDB,
  getOrbitDBDir,
  getDBAddress,
  saveDBAddress,

  // Logger
  logger,
  createServiceLogger,
  createDebrosLogger,
  type LoggerOptions,

  // Crypto
  getPrivateKey,
};

// Default export for convenience
export default {
  config,
  validateConfig,
  ipfsService,
  orbitDBService,
  logger,
  createServiceLogger,
};
