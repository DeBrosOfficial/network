import * as ipfsService from './ipfsService';
import { config } from '../config';
import { createServiceLogger } from '../utils/logger';

const logger = createServiceLogger('LOAD_BALANCER');

// Track last peer chosen for round-robin strategy
let lastPeerIndex = -1;

// Type definitions
export interface PeerInfo {
  peerId: string;
  load: number;
  publicAddress: string;
}

export interface PeerStatus extends PeerInfo {
  lastSeen: number;
}

export interface NodeStatus {
  fingerprint: string;
  peerCount: number;
  isHealthy: boolean;
}

type LoadBalancerStrategy = 'leastLoaded' | 'roundRobin' | 'random';

/**
 * Strategies for peer selection
 */
const strategies = {
  leastLoaded: (peers: PeerStatus[]): PeerStatus => {
    return peers.reduce((min, current) => (current.load < min.load ? current : min), peers[0]);
  },

  roundRobin: (peers: PeerStatus[]): PeerStatus => {
    lastPeerIndex = (lastPeerIndex + 1) % peers.length;
    return peers[lastPeerIndex];
  },

  random: (peers: PeerStatus[]): PeerStatus => {
    const randomIndex = Math.floor(Math.random() * peers.length);
    return peers[randomIndex];
  },
};

/**
 * Get the optimal peer based on the configured load balancing strategy
 */
export const getOptimalPeer = (): PeerInfo | null => {
  const connectedPeers = ipfsService.getConnectedPeers();

  if (connectedPeers.size === 0) {
    logger.info('No peers available for load balancing');
    return null;
  }

  // Convert Map to Array for easier manipulation
  const peersArray = Array.from(connectedPeers.entries()).map(([peerId, data]) => ({
    peerId,
    load: data.load,
    lastSeen: data.lastSeen,
    publicAddress: data.publicAddress,
  }));

  // Apply the selected load balancing strategy
  const strategy = config.loadBalancer.strategy as LoadBalancerStrategy;
  let selectedPeer;

  // Select strategy function or default to least loaded
  const strategyFn = strategies[strategy] || strategies.leastLoaded;
  selectedPeer = strategyFn(peersArray);

  logger.info(
    `Selected peer (${strategy}): ${selectedPeer.peerId.substring(0, 15)}... with load ${selectedPeer.load}%`,
  );

  return {
    peerId: selectedPeer.peerId,
    load: selectedPeer.load,
    publicAddress: selectedPeer.publicAddress,
  };
};

/**
 * Get all available peers with their load information
 */
export const getAllPeers = (): PeerStatus[] => {
  const connectedPeers = ipfsService.getConnectedPeers();

  return Array.from(connectedPeers.entries()).map(([peerId, data]) => ({
    peerId,
    load: data.load,
    lastSeen: data.lastSeen,
    publicAddress: data.publicAddress,
  }));
};

/**
 * Get information about the current node's load
 */
export const getNodeStatus = (): NodeStatus => {
  const connectedPeers = ipfsService.getConnectedPeers();
  return {
    fingerprint: config.env.fingerprint,
    peerCount: connectedPeers.size,
    isHealthy: true,
  };
};

export default { getOptimalPeer, getAllPeers, getNodeStatus };
