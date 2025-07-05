import { createHelia } from 'helia';
import { createLibp2p } from 'libp2p';
import { tcp } from '@libp2p/tcp';
import { noise } from '@chainsafe/libp2p-noise';
import { yamux } from '@chainsafe/libp2p-yamux';
import { bootstrap } from '@libp2p/bootstrap';
import { mdns } from '@libp2p/mdns';
import { identify } from '@libp2p/identify';
import { gossipsub } from '@chainsafe/libp2p-gossipsub';
import fs from 'fs';
import path from 'path';

export interface IPFSConfig {
  swarmKeyFile?: string;
  bootstrap?: string[];
  ports?: {
    swarm?: number;
    api?: number;
    gateway?: number;
  };
}

export class IPFSService {
  private helia: any;
  private libp2p: any;
  private config: IPFSConfig;

  constructor(config: IPFSConfig = {}) {
    this.config = config;
  }

  async init(): Promise<void> {
    // Create libp2p instance
    const libp2pConfig: any = {
      addresses: {
        listen: [`/ip4/0.0.0.0/tcp/${this.config.ports?.swarm || 4001}`]
      },
      transports: [tcp()],
      connectionEncryption: [noise()],
      streamMuxers: [yamux()],
      services: {
        identify: identify(),
        pubsub: gossipsub({
          allowPublishToZeroTopicPeers: true
        })
      }
    };

    // Add peer discovery
    const peerDiscovery = [];
    
    // Add bootstrap peers if provided
    if (this.config.bootstrap && this.config.bootstrap.length > 0) {
      peerDiscovery.push(bootstrap({
        list: this.config.bootstrap
      }));
    }

    // Add mDNS for local discovery
    peerDiscovery.push(mdns({
      interval: 1000
    }));

    if (peerDiscovery.length > 0) {
      libp2pConfig.peerDiscovery = peerDiscovery;
    }

    this.libp2p = await createLibp2p(libp2pConfig);

    // Create Helia instance
    this.helia = await createHelia({
      libp2p: this.libp2p
    });

    console.log(`IPFS Service initialized with peer ID: ${this.libp2p.peerId}`);
  }

  async stop(): Promise<void> {
    if (this.helia) {
      await this.helia.stop();
    }
  }

  getHelia(): any {
    return this.helia;
  }

  getLibp2pInstance(): any {
    return this.libp2p;
  }

  async getConnectedPeers(): Promise<Map<string, any>> {
    if (!this.libp2p) {
      return new Map();
    }

    const peers = this.libp2p.getPeers();
    const peerMap = new Map();

    for (const peerId of peers) {
      peerMap.set(peerId.toString(), peerId);
    }

    return peerMap;
  }

  async pinOnNode(nodeId: string, cid: string): Promise<void> {
    if (this.helia && this.helia.pins) {
      await this.helia.pins.add(cid);
      console.log(`Pinned ${cid} on node ${nodeId}`);
    }
  }

  get pubsub() {
    if (!this.libp2p || !this.libp2p.services.pubsub) {
      return undefined;
    }

    return {
      publish: async (topic: string, data: string) => {
        const encoder = new TextEncoder();
        await this.libp2p.services.pubsub.publish(topic, encoder.encode(data));
      },
      subscribe: async (topic: string, handler: (message: any) => void) => {
        this.libp2p.services.pubsub.subscribe(topic);
        this.libp2p.services.pubsub.addEventListener('message', (event: any) => {
          if (event.detail.topic === topic) {
            handler(event.detail);
          }
        });
      },
      unsubscribe: async (topic: string) => {
        this.libp2p.services.pubsub.unsubscribe(topic);
      }
    };
  }
}