// Load balancer controller - Handles API routes for service discovery and load balancing
import { Request, Response, NextFunction } from 'express';
import loadBalancerService from './loadBalancerService';
import { config } from '../config';

export interface LoadBalancerControllerModule {
  getNodeInfo: (_req: Request, _res: Response, _next: NextFunction) => void;
  getOptimalPeer: (_req: Request, _res: Response, _next: NextFunction) => void;
  getAllPeers: (_req: Request, _res: Response, _next: NextFunction) => void;
}

/**
 * Get information about the node and its load
 */
const getNodeInfo = (req: Request, res: Response, next: NextFunction) => {
  try {
    const status = loadBalancerService.getNodeStatus();
    res.json({
      fingerprint: config.env.fingerprint,
      peerCount: status.peerCount,
      isLoadBalancer: config.features.enableLoadBalancing,
      loadBalancerStrategy: config.loadBalancer.strategy,
      maxConnections: config.loadBalancer.maxConnections,
    });
  } catch (error) {
    next(error);
  }
};

/**
 * Get the optimal peer for client connection
 */
const getOptimalPeer = (req: Request, res: Response, next: NextFunction) => {
  try {
    // Check if load balancing is enabled
    if (!config.features.enableLoadBalancing) {
      res.status(200).json({
        useThisNode: true,
        message: 'Load balancing is disabled, use this node',
        fingerprint: config.env.fingerprint,
        publicAddress: config.ipfs.serviceDiscovery.publicAddress,
      });
      return;
    }

    // Get the optimal peer
    const optimalPeer = loadBalancerService.getOptimalPeer();

    // If there are no peer nodes, use this node
    if (!optimalPeer) {
      res.status(200).json({
        useThisNode: true,
        message: 'No other peers available, use this node',
        fingerprint: config.env.fingerprint,
        publicAddress: config.ipfs.serviceDiscovery.publicAddress,
      });
      return;
    }

    // Check if this node is the optimal peer
    const isThisNodeOptimal = optimalPeer.peerId === config.env.fingerprint;

    if (isThisNodeOptimal) {
      res.status(200).json({
        useThisNode: true,
        message: 'This node is optimal',
        fingerprint: config.env.fingerprint,
        publicAddress: config.ipfs.serviceDiscovery.publicAddress,
      });
      return;
    }

    // Return the optimal peer information
    res.status(200).json({
      useThisNode: false,
      optimalPeer: {
        peerId: optimalPeer.peerId,
        load: optimalPeer.load,
        publicAddress: optimalPeer.publicAddress,
      },
      message: 'Found optimal peer',
    });
  } catch (error) {
    next(error);
  }
};

/**
 * Get all available peers
 */
const getAllPeers = (req: Request, res: Response, next: NextFunction) => {
  try {
    const peers = loadBalancerService.getAllPeers();
    res.status(200).json({
      peerCount: peers.length,
      peers,
    });
  } catch (error) {
    next(error);
  }
};

export default {
  getNodeInfo,
  getOptimalPeer,
  getAllPeers,
} as LoadBalancerControllerModule;
