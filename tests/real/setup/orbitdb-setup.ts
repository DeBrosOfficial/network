import { loadOrbitDBModules } from './helia-wrapper';
import { RealIPFSService } from './ipfs-setup';
import { OrbitDBInstance } from '../../../src/framework/services/OrbitDBService';

export class RealOrbitDBService implements OrbitDBInstance {
  private orbitdb: any;
  private ipfsService: RealIPFSService;
  private nodeIndex: number;
  private databases: Map<string, any> = new Map();

  constructor(nodeIndex: number, ipfsService: RealIPFSService) {
    this.nodeIndex = nodeIndex;
    this.ipfsService = ipfsService;
  }

  async init(): Promise<any> {
    console.log(`🌀 Initializing OrbitDB for node ${this.nodeIndex}...`);

    try {
      // Load OrbitDB ES modules dynamically
      const { createOrbitDB } = await loadOrbitDBModules();

      const ipfs = this.ipfsService.getHelia();
      if (!ipfs) {
        throw new Error('IPFS node must be initialized before OrbitDB');
      }

      // Create OrbitDB instance
      this.orbitdb = await createOrbitDB({
        ipfs,
        id: `orbitdb-node-${this.nodeIndex}`,
        directory: `./orbitdb-${this.nodeIndex}`, // Local directory for this node
      });

      console.log(`✅ OrbitDB initialized for node ${this.nodeIndex}`);
      console.log(`📍 OrbitDB ID: ${this.orbitdb.id}`);

      return this.orbitdb;
    } catch (error) {
      console.error(`❌ Failed to initialize OrbitDB for node ${this.nodeIndex}:`, error);
      throw error;
    }
  }

  async openDB(name: string, type: string): Promise<any> {
    if (!this.orbitdb) {
      throw new Error('OrbitDB not initialized');
    }

    const dbKey = `${name}-${type}`;

    // Check if database is already open
    if (this.databases.has(dbKey)) {
      return this.databases.get(dbKey);
    }

    try {
      console.log(`📂 Opening ${type} database '${name}' on node ${this.nodeIndex}...`);

      let database;

      switch (type.toLowerCase()) {
        case 'documents':
        case 'docstore':
          database = await this.orbitdb.open(name, {
            type: 'documents',
            AccessController: 'orbitdb',
          });
          break;

        case 'events':
        case 'eventlog':
          database = await this.orbitdb.open(name, {
            type: 'events',
            AccessController: 'orbitdb',
          });
          break;

        case 'keyvalue':
        case 'kvstore':
          database = await this.orbitdb.open(name, {
            type: 'keyvalue',
            AccessController: 'orbitdb',
          });
          break;

        default:
          // Default to documents store
          database = await this.orbitdb.open(name, {
            type: 'documents',
            AccessController: 'orbitdb',
          });
      }

      this.databases.set(dbKey, database);

      console.log(`✅ Database '${name}' opened on node ${this.nodeIndex}`);
      console.log(`🔗 Database address: ${database.address}`);

      return database;
    } catch (error) {
      console.error(`❌ Failed to open database '${name}' on node ${this.nodeIndex}:`, error);
      throw error;
    }
  }

  async stop(): Promise<void> {
    console.log(`🛑 Stopping OrbitDB for node ${this.nodeIndex}...`);

    try {
      // Close all open databases
      for (const [name, database] of this.databases) {
        try {
          await database.close();
          console.log(`📂 Closed database '${name}' on node ${this.nodeIndex}`);
        } catch (error) {
          console.warn(`⚠️ Error closing database '${name}':`, error);
        }
      }
      this.databases.clear();

      // Stop OrbitDB
      if (this.orbitdb) {
        await this.orbitdb.stop();
        console.log(`✅ OrbitDB stopped for node ${this.nodeIndex}`);
      }
    } catch (error) {
      console.error(`❌ Error stopping OrbitDB for node ${this.nodeIndex}:`, error);
      throw error;
    }
  }

  getOrbitDB(): any {
    return this.orbitdb;
  }

  // Additional utility methods for testing
  async waitForReplication(database: any, timeout: number = 30000): Promise<boolean> {
    const startTime = Date.now();

    return new Promise((resolve) => {
      const checkReplication = () => {
        if (Date.now() - startTime > timeout) {
          resolve(false);
          return;
        }

        // Check if database has received updates from other peers
        const peers = database.peers || [];
        if (peers.length > 0) {
          resolve(true);
          return;
        }

        setTimeout(checkReplication, 100);
      };

      checkReplication();
    });
  }

  async getDatabaseInfo(name: string, type: string): Promise<any> {
    const dbKey = `${name}-${type}`;
    const database = this.databases.get(dbKey);

    if (!database) {
      return null;
    }

    return {
      address: database.address,
      type: database.type,
      peers: database.peers || [],
      all: await database.all(),
      meta: database.meta || {},
    };
  }
}

// Utility function to create OrbitDB network from IPFS network
export async function createOrbitDBNetwork(
  ipfsNodes: RealIPFSService[],
): Promise<RealOrbitDBService[]> {
  console.log(`🌀 Creating OrbitDB network with ${ipfsNodes.length} nodes...`);

  const orbitdbNodes: RealOrbitDBService[] = [];

  // Create OrbitDB instances for each IPFS node
  for (let i = 0; i < ipfsNodes.length; i++) {
    const orbitdbService = new RealOrbitDBService(i, ipfsNodes[i]);
    await orbitdbService.init();
    orbitdbNodes.push(orbitdbService);
  }

  console.log(`✅ OrbitDB network created with ${orbitdbNodes.length} nodes`);
  return orbitdbNodes;
}

export async function shutdownOrbitDBNetwork(orbitdbNodes: RealOrbitDBService[]): Promise<void> {
  console.log(`🛑 Shutting down OrbitDB network...`);

  // Stop all OrbitDB nodes
  await Promise.all(orbitdbNodes.map((node) => node.stop()));

  console.log(`✅ OrbitDB network shutdown complete`);
}

// Test utilities for database operations
export async function testDatabaseReplication(
  orbitdbNodes: RealOrbitDBService[],
  dbName: string,
  dbType: string = 'documents',
): Promise<boolean> {
  console.log(`🔄 Testing database replication for '${dbName}'...`);

  if (orbitdbNodes.length < 2) {
    console.log(`⚠️ Need at least 2 nodes for replication test`);
    return false;
  }

  try {
    // Open database on first node and add data
    const db1 = await orbitdbNodes[0].openDB(dbName, dbType);
    await db1.put({ _id: 'test-doc-1', content: 'Hello from node 0', timestamp: Date.now() });

    // Open same database on second node
    const db2 = await orbitdbNodes[1].openDB(dbName, dbType);

    // Wait for replication
    await new Promise((resolve) => setTimeout(resolve, 2000));

    // Check if data replicated
    const db2Data = await db2.all();
    const hasReplicatedData = db2Data.some((doc: any) => doc._id === 'test-doc-1');

    if (hasReplicatedData) {
      console.log(`✅ Database replication successful for '${dbName}'`);
      return true;
    } else {
      console.log(`❌ Database replication failed for '${dbName}'`);
      return false;
    }
  } catch (error) {
    console.error(`❌ Error testing database replication:`, error);
    return false;
  }
}
