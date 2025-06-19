import { StoreType } from '../types/framework';

export interface OrbitDBInstance {
  openDB(name: string, type: string): Promise<any>;
  getOrbitDB(): any;
  init(): Promise<any>;
  stop?(): Promise<void>;
}

export interface IPFSInstance {
  init(): Promise<any>;
  getHelia(): any;
  getLibp2pInstance(): any;
  stop?(): Promise<void>;
  pubsub?: {
    publish(topic: string, data: string): Promise<void>;
    subscribe(topic: string, handler: (message: any) => void): Promise<void>;
    unsubscribe(topic: string): Promise<void>;
  };
}

export class FrameworkOrbitDBService {
  private orbitDBService: OrbitDBInstance;

  constructor(orbitDBService: OrbitDBInstance) {
    this.orbitDBService = orbitDBService;
  }

  async openDatabase(name: string, type: StoreType): Promise<any> {
    return await this.orbitDBService.openDB(name, type);
  }

  async init(): Promise<void> {
    await this.orbitDBService.init();
  }

  async stop(): Promise<void> {
    if (this.orbitDBService.stop) {
      await this.orbitDBService.stop();
    }
  }

  getOrbitDB(): any {
    return this.orbitDBService.getOrbitDB();
  }
}

export class FrameworkIPFSService {
  private ipfsService: IPFSInstance;

  constructor(ipfsService: IPFSInstance) {
    this.ipfsService = ipfsService;
  }

  async init(): Promise<void> {
    await this.ipfsService.init();
  }

  async stop(): Promise<void> {
    if (this.ipfsService.stop) {
      await this.ipfsService.stop();
    }
  }

  getHelia(): any {
    return this.ipfsService.getHelia();
  }

  getLibp2p(): any {
    return this.ipfsService.getLibp2pInstance();
  }

  async getConnectedPeers(): Promise<Map<string, any>> {
    const libp2p = this.getLibp2p();
    if (!libp2p) {
      return new Map();
    }

    const peers = libp2p.getPeers();
    const peerMap = new Map();

    for (const peerId of peers) {
      peerMap.set(peerId.toString(), peerId);
    }

    return peerMap;
  }

  async pinOnNode(nodeId: string, cid: string): Promise<void> {
    // Implementation depends on your specific pinning setup
    // This is a placeholder for the pinning functionality
    console.log(`Pinning ${cid} on node ${nodeId}`);
  }

  get pubsub() {
    return this.ipfsService.pubsub;
  }
}
