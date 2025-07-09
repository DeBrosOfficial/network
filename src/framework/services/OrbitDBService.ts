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
  private initialized: boolean = false;

  constructor(orbitDBService: OrbitDBInstance) {
    this.orbitDBService = orbitDBService;
    // Check if the service is already initialized by trying to get OrbitDB
    try {
      if (orbitDBService.getOrbitDB && orbitDBService.getOrbitDB()) {
        this.initialized = true;
      }
    } catch (error) {
      // Service not initialized yet
    }
  }

  async openDatabase(name: string, type: StoreType): Promise<any> {
    console.log('FrameworkOrbitDBService.openDatabase called with:', { name, type });
    console.log('this.orbitDBService:', this.orbitDBService);
    console.log('typeof this.orbitDBService.openDB:', typeof this.orbitDBService.openDB);
    console.log('this.orbitDBService methods:', Object.getOwnPropertyNames(Object.getPrototypeOf(this.orbitDBService)));
    
    if (typeof this.orbitDBService.openDB !== 'function') {
      throw new Error(`openDB is not a function. Service type: ${typeof this.orbitDBService}, methods: ${Object.getOwnPropertyNames(Object.getPrototypeOf(this.orbitDBService))}`);
    }
    
    return await this.orbitDBService.openDB(name, type);
  }

  async init(): Promise<void> {
    if (!this.initialized) {
      await this.orbitDBService.init();
      this.initialized = true;
    }
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
  private initialized: boolean = false;

  constructor(ipfsService: IPFSInstance) {
    this.ipfsService = ipfsService;
    // Check if the service is already initialized by trying to get Helia
    try {
      if (ipfsService.getHelia && ipfsService.getHelia()) {
        this.initialized = true;
      }
    } catch (error) {
      // Service not initialized yet
    }
  }

  async init(): Promise<void> {
    if (!this.initialized) {
      await this.ipfsService.init();
      this.initialized = true;
    }
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
