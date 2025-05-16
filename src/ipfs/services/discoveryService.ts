import type { PubSub } from '@libp2p/interface';
import { config } from '../../config';
import { ipfsConfig } from '../config/ipfsConfig';
import { createServiceLogger } from '../../utils/logger';

// Create loggers for service discovery and heartbeat
const discoveryLogger = createServiceLogger('SERVICE-DISCOVERY');
const heartbeatLogger = createServiceLogger('HEARTBEAT');

// Node metadata
const fingerprint = config.env.fingerprint;

const connectedPeers: Map<
  string,
  { lastSeen: number; load: number; publicAddress: string; fingerprint: string }
> = new Map();
const SERVICE_DISCOVERY_TOPIC = ipfsConfig.serviceDiscovery.topic;
const HEARTBEAT_INTERVAL = ipfsConfig.serviceDiscovery.heartbeatInterval;
let heartbeatInterval: NodeJS.Timeout;
let nodeLoad = 0;

export const setupServiceDiscovery = async (pubsub: PubSub) => {
  await pubsub.subscribe(SERVICE_DISCOVERY_TOPIC);
  discoveryLogger.info(`Subscribed to topic: ${SERVICE_DISCOVERY_TOPIC}`);

  // Listen for other peers heartbeats
  pubsub.addEventListener('message', (event: any) => {
    try {
      const message = JSON.parse(event.detail.data.toString());
      if (message.type === 'heartbeat' && message.fingerprint !== fingerprint) {
        const peerId = event.detail.from.toString();
        const existingPeer = connectedPeers.has(peerId);

        connectedPeers.set(peerId, {
          lastSeen: Date.now(),
          load: message.load,
          publicAddress: message.publicAddress,
          fingerprint: message.fingerprint,
        });

        if (!existingPeer) {
          discoveryLogger.info(
            `New peer discovered: ${peerId} (fingerprint=${message.fingerprint})`,
          );
        }
        heartbeatLogger.info(
          `Received from ${peerId}: load=${message.load}, addr=${message.publicAddress}`,
        );
      }
    } catch (err) {
      discoveryLogger.error(`Error processing message:`, err);
    }
  });

  // Send periodic heartbeats with our load information
  heartbeatInterval = setInterval(async () => {
    try {
      nodeLoad = calculateNodeLoad();
      const heartbeatMsg = {
        type: 'heartbeat',
        fingerprint,
        load: nodeLoad,
        timestamp: Date.now(),
        publicAddress: ipfsConfig.serviceDiscovery.publicAddress,
      };

      await pubsub.publish(
        SERVICE_DISCOVERY_TOPIC,
        new TextEncoder().encode(JSON.stringify(heartbeatMsg)),
      );
      heartbeatLogger.info(
        `Sent: fingerprint=${fingerprint}, load=${nodeLoad}, addr=${heartbeatMsg.publicAddress}`,
      );

      const now = Date.now();
      const staleTime = ipfsConfig.serviceDiscovery.staleTimeout;

      for (const [peerId, peerData] of connectedPeers.entries()) {
        if (now - peerData.lastSeen > staleTime) {
          discoveryLogger.info(
            `Peer ${peerId.substring(0, 15)}... is stale, removing from load balancer`,
          );
          connectedPeers.delete(peerId);
        }
      }

      if (Date.now() % 60000 < HEARTBEAT_INTERVAL) {
        logPeersStatus();
      }
    } catch (err) {
      discoveryLogger.error(`Error sending heartbeat:`, err);
    }
  }, HEARTBEAT_INTERVAL);

  discoveryLogger.info(`Service initialized with fingerprint: ${fingerprint}`);
};

/**
 * Calculates the current node load
 */
export const calculateNodeLoad = (): number => {
  // This is a simple implementation and could be enhanced with
  // actual metrics like CPU usage, memory, active connections, etc.
  return Math.floor(Math.random() * 100); // Placeholder implementation
};

/**
 * Logs the status of connected peers
 */
export const logPeersStatus = () => {
  const peerCount = connectedPeers.size;
  discoveryLogger.info(`Connected peers: ${peerCount}`);
  discoveryLogger.info(`Current node load: ${nodeLoad}`);

  if (peerCount > 0) {
    discoveryLogger.info('Peer status:');
    connectedPeers.forEach((data, peerId) => {
      discoveryLogger.debug(
        `  - ${peerId} Load: ${data.load}% Last seen: ${new Date(data.lastSeen).toISOString()}`,
      );
    });
  }
};

export const getOptimalPeer = (): string | null => {
  if (connectedPeers.size === 0) return null;

  let lowestLoad = Number.MAX_SAFE_INTEGER;
  let optimalPeer: string | null = null;

  connectedPeers.forEach((data, peerId) => {
    if (data.load < lowestLoad) {
      lowestLoad = data.load;
      optimalPeer = peerId;
    }
  });

  return optimalPeer;
};

export const updateNodeLoad = (load: number) => {
  nodeLoad = load;
};

export const getConnectedPeers = () => {
  return connectedPeers;
};

export const stopDiscoveryService = async (pubsub: PubSub | null) => {
  if (heartbeatInterval) {
    clearInterval(heartbeatInterval);
  }

  if (pubsub) {
    try {
      await pubsub.unsubscribe(SERVICE_DISCOVERY_TOPIC);
      discoveryLogger.info(`Unsubscribed from topic: ${SERVICE_DISCOVERY_TOPIC}`);
    } catch (err) {
      discoveryLogger.error(`Error unsubscribing from topic:`, err);
    }
  }
};
