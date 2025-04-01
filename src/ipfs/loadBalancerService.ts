// Load balancer service - Implements load balancing strategies for distributing connections
import * as ipfsService from './ipfsService';
import { config } from '../config';

// Track last peer chosen for round-robin strategy
let lastPeerIndex = -1;

interface PeerInfo {
  peerId: string;
  load: number;
  publicAddress: string;
}

interface PeerStatus extends PeerInfo {
  lastSeen: number;
}

interface NodeStatus {
  fingerprint: string;
  peerCount: number;
  isHealthy: boolean;
}

interface LoadBalancerServiceModule {
  getOptimalPeer: () => PeerInfo | null;
  getAllPeers: () => PeerStatus[];
  getNodeStatus: () => NodeStatus;
}

/**
 * Get the optimal peer based on the configured load balancing strategy
 * @returns Object containing the selected peer information or null if no peers available
 */
export const getOptimalPeer = (): { peerId: string; load: number; publicAddress: string } | null => {
  // Get all available peers
  const connectedPeers = ipfsService.getConnectedPeers();

  // If no peers are available, return null
  if (connectedPeers.size === 0) {
    console.log('[LOAD-BALANCER] No peers available for load balancing');
    return null;
  }

  // Convert Map to Array for easier manipulation
  const peersArray = Array.from(connectedPeers.entries()).map(([peerId, data]) => {
    return {
      peerId,
      load: data.load,
      lastSeen: data.lastSeen,
      publicAddress: data.publicAddress,
    };
  });

  // Apply the load balancing strategy
  const strategy = config.loadBalancer.strategy;
  let selectedPeer;

  switch (strategy) {
    case 'least-loaded':
      // Find the peer with the lowest load
      selectedPeer = peersArray.reduce((min, current) => (current.load < min.load ? current : min), peersArray[0]);
      console.log(
        `[LOAD-BALANCER] Selected least loaded peer: ${selectedPeer.peerId.substring(0, 15)}... with load ${
          selectedPeer.load
        }%`
      );
      break;

    case 'round-robin':
      // Simple round-robin strategy
      lastPeerIndex = (lastPeerIndex + 1) % peersArray.length;
      selectedPeer = peersArray[lastPeerIndex];
      console.log(
        `[LOAD-BALANCER] Selected round-robin peer: ${selectedPeer.peerId.substring(0, 15)}... with load ${
          selectedPeer.load
        }%`
      );
      break;

    case 'random':
      // Random selection
      const randomIndex = Math.floor(Math.random() * peersArray.length);
      selectedPeer = peersArray[randomIndex];
      console.log(
        `[LOAD-BALANCER] Selected random peer: ${selectedPeer.peerId.substring(0, 15)}... with load ${
          selectedPeer.load
        }%`
      );
      break;

    default:
      // Default to least-loaded if unknown strategy
      selectedPeer = peersArray.reduce((min, current) => (current.load < min.load ? current : min), peersArray[0]);
      console.log(
        `[LOAD-BALANCER] Selected least loaded peer: ${selectedPeer.peerId.substring(0, 15)}... with load ${
          selectedPeer.load
        }%`
      );
  }

  return {
    peerId: selectedPeer.peerId,
    load: selectedPeer.load,
    publicAddress: selectedPeer.publicAddress,
  };
};

/**
 * Get all available peers with their load information
 * @returns Array of peer information objects
 */
export const getAllPeers = () => {
  const connectedPeers = ipfsService.getConnectedPeers();
  const result: any = [];

  connectedPeers.forEach((data, peerId) => {
    result.push({
      peerId,
      load: data.load,
      lastSeen: data.lastSeen,
    });
  });

  return result;
};

/**
 * Get information about the current node's load
 */
export const getNodeStatus = () => {
  const connectedPeers = ipfsService.getConnectedPeers();
  return {
    fingerprint: config.env.fingerprint,
    peerCount: connectedPeers.size,
    isHealthy: true,
  };
};

export default {
  getOptimalPeer,
  getAllPeers,
  getNodeStatus,
} as LoadBalancerServiceModule;
