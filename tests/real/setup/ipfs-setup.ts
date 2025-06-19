import { createHelia } from 'helia';
import { createLibp2p } from 'libp2p';
import { tcp } from '@libp2p/tcp';
import { noise } from '@chainsafe/libp2p-noise';
import { yamux } from '@chainsafe/libp2p-yamux';
import { gossipsub } from '@chainsafe/libp2p-gossipsub';
import { identify } from '@libp2p/identify';
import { FsBlockstore } from 'blockstore-fs';
import { FsDatastore } from 'datastore-fs';
import { join } from 'path';
import { PrivateSwarmSetup } from './swarm-setup';
import { IPFSInstance } from '../../../src/framework/services/OrbitDBService';

export class RealIPFSService implements IPFSInstance {
  private helia: any;
  private libp2p: any;
  private nodeIndex: number;
  private swarmSetup: PrivateSwarmSetup;
  private dataDir: string;

  constructor(nodeIndex: number, swarmSetup: PrivateSwarmSetup) {
    this.nodeIndex = nodeIndex;
    this.swarmSetup = swarmSetup;
    this.dataDir = swarmSetup.getNodeDataDir(nodeIndex);
  }

  async init(): Promise<any> {
    console.log(`🚀 Initializing IPFS node ${this.nodeIndex}...`);

    try {
      // Create libp2p instance with private swarm configuration
      this.libp2p = await createLibp2p({
        addresses: {
          listen: [`/ip4/127.0.0.1/tcp/${this.swarmSetup.getNodePort(this.nodeIndex)}`],
        },
        transports: [tcp()],
        connectionEncrypters: [noise()],
        streamMuxers: [yamux()],
        services: {
          identify: identify(),
          pubsub: gossipsub({
            allowPublishToZeroTopicPeers: true,
            canRelayMessage: true,
            emitSelf: false,
          }),
        },
        connectionManager: {
          maxConnections: 10,
          dialTimeout: 10000,
          inboundUpgradeTimeout: 10000,
        },
        start: false, // Don't auto-start, we'll start manually
      });

      // Create blockstore and datastore
      const blockstore = new FsBlockstore(join(this.dataDir, 'blocks'));
      const datastore = new FsDatastore(join(this.dataDir, 'datastore'));

      // Create Helia instance
      this.helia = await createHelia({
        libp2p: this.libp2p,
        blockstore,
        datastore,
        start: false,
      });

      // Start the node
      await this.helia.start();

      console.log(
        `✅ IPFS node ${this.nodeIndex} started with Peer ID: ${this.libp2p.peerId.toString()}`,
      );
      console.log(
        `📡 Listening on: ${this.libp2p
          .getMultiaddrs()
          .map((ma) => ma.toString())
          .join(', ')}`,
      );

      return this.helia;
    } catch (error) {
      console.error(`❌ Failed to initialize IPFS node ${this.nodeIndex}:`, error);
      throw error;
    }
  }

  async connectToPeers(peerNodes: RealIPFSService[]): Promise<void> {
    if (!this.libp2p) {
      throw new Error('IPFS node not initialized');
    }

    for (const peerNode of peerNodes) {
      if (peerNode.nodeIndex === this.nodeIndex) continue; // Don't connect to self

      try {
        const peerAddrs = peerNode.getMultiaddrs();

        for (const addr of peerAddrs) {
          try {
            console.log(
              `🔗 Node ${this.nodeIndex} connecting to node ${peerNode.nodeIndex} at ${addr}`,
            );
            await this.libp2p.dial(addr);
            console.log(`✅ Node ${this.nodeIndex} connected to node ${peerNode.nodeIndex}`);
            break; // Successfully connected, no need to try other addresses
          } catch (dialError) {
            console.log(`⚠️ Failed to dial ${addr}: ${dialError.message}`);
          }
        }
      } catch (error) {
        console.warn(
          `⚠️ Could not connect node ${this.nodeIndex} to node ${peerNode.nodeIndex}:`,
          error.message,
        );
      }
    }
  }

  getMultiaddrs(): string[] {
    if (!this.libp2p) return [];
    return this.libp2p.getMultiaddrs().map((ma: any) => ma.toString());
  }

  getPeerId(): string {
    if (!this.libp2p) return '';
    return this.libp2p.peerId.toString();
  }

  getConnectedPeers(): string[] {
    if (!this.libp2p) return [];
    return this.libp2p.getPeers().map((peer: any) => peer.toString());
  }

  async stop(): Promise<void> {
    console.log(`🛑 Stopping IPFS node ${this.nodeIndex}...`);

    try {
      if (this.helia) {
        await this.helia.stop();
        console.log(`✅ IPFS node ${this.nodeIndex} stopped`);
      }
    } catch (error) {
      console.error(`❌ Error stopping IPFS node ${this.nodeIndex}:`, error);
      throw error;
    }
  }

  getHelia(): any {
    return this.helia;
  }

  getLibp2pInstance(): any {
    return this.libp2p;
  }

  // Framework interface compatibility
  get pubsub() {
    if (!this.libp2p?.services?.pubsub) {
      throw new Error('PubSub service not available');
    }

    return {
      publish: async (topic: string, data: string) => {
        const encoder = new TextEncoder();
        await this.libp2p.services.pubsub.publish(topic, encoder.encode(data));
      },
      subscribe: async (topic: string, handler: (message: any) => void) => {
        this.libp2p.services.pubsub.addEventListener('message', (evt: any) => {
          if (evt.detail.topic === topic) {
            const decoder = new TextDecoder();
            const message = {
              topic: evt.detail.topic,
              data: decoder.decode(evt.detail.data),
              from: evt.detail.from.toString(),
            };
            handler(message);
          }
        });
        this.libp2p.services.pubsub.subscribe(topic);
      },
      unsubscribe: async (topic: string) => {
        this.libp2p.services.pubsub.unsubscribe(topic);
      },
    };
  }
}

// Utility function to create multiple IPFS nodes in a private network
export async function createIPFSNetwork(nodeCount: number = 3): Promise<{
  nodes: RealIPFSService[];
  swarmSetup: PrivateSwarmSetup;
}> {
  console.log(`🌐 Creating private IPFS network with ${nodeCount} nodes...`);

  const swarmSetup = new PrivateSwarmSetup(nodeCount);
  const nodes: RealIPFSService[] = [];

  // Create all nodes
  for (let i = 0; i < nodeCount; i++) {
    const node = new RealIPFSService(i, swarmSetup);
    nodes.push(node);
  }

  // Initialize all nodes
  for (const node of nodes) {
    await node.init();
  }

  // Wait a moment for nodes to be ready
  await new Promise((resolve) => setTimeout(resolve, 1000));

  // Connect nodes in a mesh topology
  for (let i = 0; i < nodes.length; i++) {
    const currentNode = nodes[i];
    const otherNodes = nodes.filter((_, index) => index !== i);
    await currentNode.connectToPeers(otherNodes);
  }

  // Wait for connections to establish
  await new Promise((resolve) => setTimeout(resolve, 2000));

  // Report network status
  console.log(`📊 Private IPFS Network Status:`);
  for (const node of nodes) {
    const peers = node.getConnectedPeers();
    console.log(`  Node ${node.nodeIndex}: ${peers.length} peers connected`);
  }

  return { nodes, swarmSetup };
}

export async function shutdownIPFSNetwork(
  nodes: RealIPFSService[],
  swarmSetup: PrivateSwarmSetup,
): Promise<void> {
  console.log(`🛑 Shutting down IPFS network...`);

  // Stop all nodes
  await Promise.all(nodes.map((node) => node.stop()));

  // Cleanup test data
  swarmSetup.cleanup();

  console.log(`✅ IPFS network shutdown complete`);
}
