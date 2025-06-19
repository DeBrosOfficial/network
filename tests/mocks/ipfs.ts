// Mock IPFS for testing
export class MockLibp2p {
  private peers = new Set<string>();

  async start() {
    // Mock start
  }

  async stop() {
    // Mock stop
  }

  getPeers() {
    return Array.from(this.peers);
  }

  async dial(peerId: string) {
    this.peers.add(peerId);
    return { remotePeer: peerId };
  }

  async hangUp(peerId: string) {
    this.peers.delete(peerId);
  }

  get peerId() {
    return { toString: () => 'mock-peer-id' };
  }

  // PubSub mock
  pubsub = {
    publish: jest.fn(async (topic: string, data: Uint8Array) => {
      // Mock publish
    }),
    subscribe: jest.fn(async (topic: string) => {
      // Mock subscribe
    }),
    unsubscribe: jest.fn(async (topic: string) => {
      // Mock unsubscribe
    }),
    getTopics: jest.fn(() => []),
    getPeers: jest.fn(() => [])
  };

  // Services mock
  services = {
    pubsub: this.pubsub
  };
}

export class MockHelia {
  public libp2p: MockLibp2p;
  private content = new Map<string, Uint8Array>();
  private pins = new Set<string>();

  constructor() {
    this.libp2p = new MockLibp2p();
  }

  async start() {
    await this.libp2p.start();
  }

  async stop() {
    await this.libp2p.stop();
  }

  get blockstore() {
    return {
      put: jest.fn(async (cid: any, block: Uint8Array) => {
        const key = cid.toString();
        this.content.set(key, block);
        return cid;
      }),
      get: jest.fn(async (cid: any) => {
        const key = cid.toString();
        const block = this.content.get(key);
        if (!block) {
          throw new Error(`Block not found: ${key}`);
        }
        return block;
      }),
      has: jest.fn(async (cid: any) => {
        return this.content.has(cid.toString());
      }),
      delete: jest.fn(async (cid: any) => {
        return this.content.delete(cid.toString());
      })
    };
  }

  get datastore() {
    return {
      put: jest.fn(async (key: any, value: Uint8Array) => {
        this.content.set(key.toString(), value);
      }),
      get: jest.fn(async (key: any) => {
        const value = this.content.get(key.toString());
        if (!value) {
          throw new Error(`Key not found: ${key}`);
        }
        return value;
      }),
      has: jest.fn(async (key: any) => {
        return this.content.has(key.toString());
      }),
      delete: jest.fn(async (key: any) => {
        return this.content.delete(key.toString());
      })
    };
  }

  get pins() {
    return {
      add: jest.fn(async (cid: any) => {
        this.pins.add(cid.toString());
      }),
      rm: jest.fn(async (cid: any) => {
        this.pins.delete(cid.toString());
      }),
      ls: jest.fn(async function* () {
        for (const pin of Array.from(this.pins)) {
          yield { cid: pin };
        }
      }.bind(this))
    };
  }

  // Add UnixFS mock
  get fs() {
    return {
      addBytes: jest.fn(async (data: Uint8Array) => {
        const cid = `mock-cid-${Date.now()}`;
        this.content.set(cid, data);
        return { toString: () => cid };
      }),
      cat: jest.fn(async function* (cid: any) {
        const data = this.content.get(cid.toString());
        if (data) {
          yield data;
        }
      }.bind(this)),
      addFile: jest.fn(async (file: any) => {
        const cid = `mock-file-cid-${Date.now()}`;
        return { toString: () => cid };
      })
    };
  }
}

export const createHelia = jest.fn(async (options: any = {}) => {
  const helia = new MockHelia();
  await helia.start();
  return helia;
});

export const createLibp2p = jest.fn(async (options: any = {}) => {
  return new MockLibp2p();
});

// Mock IPFS service for framework
export class MockIPFSService {
  private helia: MockHelia;

  constructor() {
    this.helia = new MockHelia();
  }

  async init() {
    await this.helia.start();
  }

  async stop() {
    await this.helia.stop();
  }

  getHelia() {
    return this.helia;
  }

  getLibp2pInstance() {
    return this.helia.libp2p;
  }

  async getConnectedPeers() {
    const peers = this.helia.libp2p.getPeers();
    const peerMap = new Map();
    peers.forEach(peer => peerMap.set(peer, peer));
    return peerMap;
  }

  async pinOnNode(nodeId: string, cid: string) {
    await this.helia.pins.add(cid);
  }

  get pubsub() {
    return {
      publish: jest.fn(async (topic: string, data: string) => {
        await this.helia.libp2p.pubsub.publish(topic, new TextEncoder().encode(data));
      }),
      subscribe: jest.fn(async (topic: string, handler: Function) => {
        // Mock subscribe
      }),
      unsubscribe: jest.fn(async (topic: string) => {
        // Mock unsubscribe
      })
    };
  }
}

// Mock OrbitDB service for framework
export class MockOrbitDBService {
  private orbitdb: any;

  constructor() {
    this.orbitdb = new (require('./orbitdb').MockOrbitDB)();
  }

  async init() {
    await this.orbitdb.start();
  }

  async stop() {
    await this.orbitdb.stop();
  }

  async openDB(name: string, type: string) {
    return await this.orbitdb.open(name, { type });
  }

  async openDatabase(name: string, type: string) {
    return await this.openDB(name, type);
  }

  getOrbitDB() {
    return this.orbitdb;
  }
}

// Default export
export default {
  createHelia,
  createLibp2p,
  MockHelia,
  MockLibp2p,
  MockIPFSService,
  MockOrbitDBService
};