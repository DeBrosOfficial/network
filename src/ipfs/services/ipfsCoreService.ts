import fs from 'fs';
import { createHelia } from 'helia';
import { FsBlockstore } from 'blockstore-fs';
import { createLibp2p } from 'libp2p';
import { gossipsub } from '@chainsafe/libp2p-gossipsub';
import { tcp } from '@libp2p/tcp';
import { noise } from '@chainsafe/libp2p-noise';
import { yamux } from '@chainsafe/libp2p-yamux';
import { identify } from '@libp2p/identify';
import { bootstrap } from '@libp2p/bootstrap';
import type { Libp2p } from 'libp2p';
import { FaultTolerance, PubSub } from '@libp2p/interface';

import { ipfsConfig } from '../config/ipfsConfig';
import { getPrivateKey } from '../utils/crypto';
import { setupServiceDiscovery, stopDiscoveryService } from './discoveryService';
import { createServiceLogger } from '../../utils/logger';

const logger = createServiceLogger('IPFS');
const p2pLogger = createServiceLogger('P2P');

let helia: any;
let proxyAgent: any;
let libp2pNode: Libp2p;
let reconnectInterval: NodeJS.Timeout;

export const initIpfsNode = async (externalProxyAgent: any = null) => {
  try {
    proxyAgent = externalProxyAgent;

    const blockstorePath = ipfsConfig.blockstorePath;
    if (!fs.existsSync(blockstorePath)) {
      fs.mkdirSync(blockstorePath, { recursive: true });
      logger.info(`Created blockstore directory: ${blockstorePath}`);
    }

    const blockstore = new FsBlockstore(blockstorePath);

    const currentNodeIp = process.env.HOSTNAME || '';
    logger.info(`Current node public IP: ${currentNodeIp}`);

    const bootstrapList = getBootstrapList();
    logger.info(`Bootstrap peers: ${JSON.stringify(bootstrapList)}`);

    const bootStrap = bootstrap({
      list: bootstrapList,
    }) as unknown as any;

    logger.info(`Configuring bootstrap with peers: ${JSON.stringify(bootstrapList)}`);

    const ipfsPort = ipfsConfig.port;
    logger.info(`Using port ${ipfsPort} for IPFS/libp2p`);

    libp2pNode = await createLibp2p({
      transports: [tcp()],
      streamMuxers: [yamux()],
      connectionEncrypters: [noise()],
      services: {
        identify: identify(),
        pubsub: gossipsub({
          allowPublishToZeroTopicPeers: true,
          emitSelf: false,
        }),
      },
      peerDiscovery: [bootStrap],
      addresses: {
        listen: [`/ip4/0.0.0.0/tcp/${ipfsPort}`],
      },
      transportManager: {
        faultTolerance: FaultTolerance.NO_FATAL,
      },
      privateKey: await getPrivateKey(),
    });

    p2pLogger.info(`PEER ID: ${libp2pNode.peerId.toString()}`);
    logger.info(
      `Listening on: ${libp2pNode
        .getMultiaddrs()
        .map((addr: any) => addr.toString())
        .join(', ')}`
    );

    helia = await createHelia({
      blockstore,
      libp2p: libp2pNode,
    });

    const pubsub = libp2pNode.services.pubsub as PubSub;
    await setupServiceDiscovery(pubsub);

    setupPeerEventListeners(libp2pNode);

    connectToSpecificPeers(libp2pNode);

    return helia;
  } catch (error) {
    logger.error('Failed to initialize node:', error);
    throw error;
  }
};

function getBootstrapList(): string[] {
  let bootstrapList: string[] = [];
  bootstrapList = process.env.BOOTSTRAP_NODES?.split(',').map((node) => node.trim()) || [];

  return bootstrapList;
}

function setupPeerEventListeners(node: Libp2p) {
  node.addEventListener('peer:discovery', (event) => {
    const peerId = event.detail.id.toString();
    logger.info(`Discovered peer: ${peerId}`);
  });

  node.addEventListener('peer:connect', (event) => {
    const peerId = event.detail.toString();
    logger.info(`Peer connection succeeded: ${peerId}`);
    node.peerStore
      .get(event.detail)
      .then((peerInfo) => {
        const multiaddrs = peerInfo?.addresses.map((addr) => addr.multiaddr.toString()) || ['unknown'];
        logger.info(`Peer multiaddrs: ${multiaddrs.join(', ')}`);
      })
      .catch((error) => {
        logger.error(`Error fetching peer info for ${peerId}: ${error.message}`);
      });
  });

  node.addEventListener('peer:disconnect', (event) => {
    const peerId = event.detail.toString();
    logger.info(`Disconnected from peer: ${peerId}`);
  });

  node.addEventListener('peer:reconnect-failure', (event) => {
    const peerId = event.detail.toString();
    logger.error(`Peer reconnection failed: ${peerId}`);
    node.peerStore
      .get(event.detail)
      .then((peerInfo) => {
        const multiaddrs = peerInfo?.addresses.map((addr) => addr.multiaddr.toString()) || ['unknown'];
        logger.error(`Peer multiaddrs: ${multiaddrs.join(', ')}`);
      })
      .catch((error) => {
        logger.error(`Error fetching peer info for ${peerId}: ${error.message}`);
      });
  });

  node.addEventListener('connection:close', (event) => {
    const connection = event.detail;
    const peerId = connection.remotePeer.toString();
    const remoteAddr = connection.remoteAddr.toString();
    logger.info(`Connection closed for peer: ${peerId}`);
    logger.info(`Remote address: ${remoteAddr}`);
  });
}

export const stopIpfsNode = async () => {
  if (reconnectInterval) {
    clearInterval(reconnectInterval);
  }

  if (libp2pNode) {
    const pubsub = libp2pNode.services.pubsub as PubSub;
    await stopDiscoveryService(pubsub);
  } else {
    await stopDiscoveryService(null);
  }

  if (helia) {
    await helia.stop();
  }
};

export const getHeliaInstance = () => {
  return helia;
};

export const getLibp2pInstance = () => {
  return libp2pNode;
};

export const getProxyAgentInstance = () => {
  return proxyAgent;
};

function connectToSpecificPeers(node: Libp2p) {
  setTimeout(async () => {
    await attemptPeerConnections(node);

    reconnectInterval = setInterval(async () => {
      await attemptPeerConnections(node);
    }, 120000);
  }, 5000);
}

async function attemptPeerConnections(node: Libp2p) {
  logger.info('Current peer connections:');
  const peers = node.getPeers();
  if (peers.length === 0) {
    logger.info('  - No connected peers');
  } else {
    for (const peerId of peers) {
      try {
        // Get peer info including addresses
        const peerInfo = await node.peerStore.get(peerId);
        const addresses = peerInfo?.addresses.map((addr) => addr.multiaddr.toString()).join(', ') || 'unknown';
        logger.info(`  - Connected to peer: ${peerId.toString()}`);
        logger.info(`    Addresses: ${addresses}`);
      } catch (_error) {
        // Fallback to just showing the peer ID if we can't get address info
        logger.info(`  - Connected to peer: ${peerId.toString()}`);
      }
    }
  }
}
