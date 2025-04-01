import type { Libp2p } from 'libp2p';

import {
  initIpfsNode,
  stopIpfsNode,
  getHeliaInstance,
  getLibp2pInstance,
  getProxyAgentInstance,
} from './services/ipfsCoreService';

import { getConnectedPeers, getOptimalPeer, updateNodeLoad, logPeersStatus } from './services/discoveryService';
import { createServiceLogger } from '../utils/logger';

// Create logger for IPFS service
const logger = createServiceLogger('IPFS');

// Interface definition for the IPFS module
export interface IPFSModule {
  init: (externalProxyAgent?: any) => Promise<void>;
  stop: () => Promise<void>;
  getHelia: () => any;
  getProxyAgent: () => any;
  getInstance: (externalProxyAgent?: any) => Promise<{
    getHelia: () => any;
    getProxyAgent: () => any;
  }>;
  getLibp2p: () => Libp2p;
  getConnectedPeers: () => Map<string, { lastSeen: number; load: number; publicAddress: string }>;
  getOptimalPeer: () => string | null;
  updateNodeLoad: (load: number) => void;
  logPeersStatus: () => void;
}

const init = async (externalProxyAgent: any = null) => {
  try {
    await initIpfsNode(externalProxyAgent);
    logger.info('IPFS service initialized successfully');
    return getHeliaInstance();
  } catch (error) {
    logger.error('Failed to initialize IPFS service:', error);
    throw error;
  }
};

const stop = async () => {
  await stopIpfsNode();
  logger.info('IPFS service stopped');
};

const getHelia = () => {
  return getHeliaInstance();
};

const getProxyAgent = () => {
  return getProxyAgentInstance();
};

const getLibp2p = () => {
  return getLibp2pInstance();
};

const getInstance = async (externalProxyAgent: any = null) => {
  if (!getHeliaInstance()) {
    await init(externalProxyAgent);
  }

  return {
    getHelia,
    getProxyAgent,
  };
};

// Export individual functions
export {
  init,
  stop,
  getHelia,
  getProxyAgent,
  getInstance,
  getLibp2p,
  getConnectedPeers,
  getOptimalPeer,
  updateNodeLoad,
  logPeersStatus,
};

// Export as default module
export default {
  init,
  stop,
  getHelia,
  getProxyAgent,
  getInstance,
  getLibp2p,
  getConnectedPeers,
  getOptimalPeer,
  updateNodeLoad,
  logPeersStatus,
} as IPFSModule;
