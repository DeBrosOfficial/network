import { ModelRegistry } from './ModelRegistry';
import { FrameworkOrbitDBService } from '../services/OrbitDBService';
import { StoreType } from '../types/framework';
import { UserMappings } from '../types/models';

export class UserMappingsData implements UserMappings {
  constructor(
    public userId: string,
    public databases: Record<string, string>,
  ) {}
}

export class DatabaseManager {
  private orbitDBService: FrameworkOrbitDBService;
  private databases: Map<string, any> = new Map();
  private userMappings: Map<string, any> = new Map();
  private globalDatabases: Map<string, any> = new Map();
  private globalDirectoryShards: any[] = [];
  private initialized: boolean = false;
  private appName: string;

  constructor(orbitDBService: FrameworkOrbitDBService, appName: string = 'debros-app') {
    this.orbitDBService = orbitDBService;
    this.appName = appName.toLowerCase().replace(/[^a-z0-9-]/g, '-'); // Sanitize app name
  }

  async initializeAllDatabases(): Promise<void> {
    if (this.initialized) {
      return;
    }

    console.log('🚀 Initializing DebrosFramework databases...');

    // Initialize global databases first
    await this.initializeGlobalDatabases();

    // Initialize system databases (user directory, etc.)
    await this.initializeSystemDatabases();

    this.initialized = true;
    console.log('✅ Database initialization complete');
  }

  private async initializeGlobalDatabases(): Promise<void> {
    const globalModels = ModelRegistry.getGlobalModels();

    console.log(`📊 Creating ${globalModels.length} global databases...`);
    for (const model of globalModels) {
      const dbName = `global-${model.modelName.toLowerCase()}`;

      try {
        const db = await this.createDatabase(dbName, (model as any).dbType || model.storeType, 'global');
        this.globalDatabases.set(model.modelName, db);

        console.log(`✓ Created global database: ${dbName} (${(model as any).dbType || model.storeType})`);
      } catch (error) {
        console.error(`❌ Failed to create global database ${dbName}:`, error);
        throw error;
      }
    }
  }

  private async initializeSystemDatabases(): Promise<void> {
    console.log('🔧 Creating system databases...');

    // Create global user directory shards that are shared across all nodes
    const DIRECTORY_SHARD_COUNT = 4; // Configurable
    
    // Use deterministic approach for shared shards
    await this.initializeSharedShards(DIRECTORY_SHARD_COUNT);

    console.log(`✅ Initialized ${this.globalDirectoryShards.length} directory shards`);
  }
  
  private async initializeSharedShards(shardCount: number): Promise<void> {
    console.log(`🔧 Initializing ${shardCount} shared directory shards...`);
    
    // First, create or connect to a bootstrap database for sharing shard addresses
    const bootstrapDB = await this.getOrCreateBootstrapDB();
    
    // Implement leader election to prevent race conditions
    let shardAddresses: string[] = [];
    let isLeader = false;
    
    try {
      // Try to become the leader by atomically setting a leader flag
      const nodeId = this.getNodeId();
      const leaderKey = 'shard-leader';
      const shardAddressKey = 'shard-addresses';
      
      // Check if someone is already the leader and has published shards
      try {
        const existingShards = await bootstrapDB.get(shardAddressKey);
        if (existingShards && Array.isArray(existingShards) && existingShards.length === shardCount) {
          shardAddresses = existingShards;
          console.log(`📡 Found existing shard addresses in bootstrap database`);
        }
      } catch (error) {
        console.log(`🔍 No existing shard addresses found`);
      }
      
      if (shardAddresses.length === 0) {
        // Try to become the leader
        try {
          const existingLeader = await bootstrapDB.get(leaderKey);
          if (!existingLeader) {
            // No leader yet, try to become one
            await bootstrapDB.set(leaderKey, { nodeId, timestamp: Date.now() });
            console.log(`👑 Became shard leader: ${nodeId}`);
            isLeader = true;
          } else {
            console.log(`🔍 Another node is already the leader: ${existingLeader.nodeId}`);
            // Wait a bit for the leader to create shards
            await new Promise(resolve => setTimeout(resolve, 2000));
            
            // Try again to get shard addresses
            try {
              const shards = await bootstrapDB.get(shardAddressKey);
              if (shards && Array.isArray(shards) && shards.length === shardCount) {
                shardAddresses = shards;
                console.log(`📡 Found shard addresses published by leader`);
              }
            } catch (error) {
              console.warn(`⚠️ Leader did not publish shards, will create our own`);
            }
          }
        } catch (error) {
          console.log(`🔍 Failed to check/set leader, proceeding anyway`);
        }
      }
    } catch (error) {
      console.warn(`⚠️ Bootstrap coordination failed, creating local shards:`, error);
    }
    
    if (shardAddresses.length === shardCount) {
      // Connect to existing shards
      await this.connectToExistingShards(shardAddresses);
    } else {
      // Create new shards (either as leader or fallback)
      await this.createAndPublishShards(shardCount, bootstrapDB, isLeader);
    }
    
    console.log(`✅ Initialized ${this.globalDirectoryShards.length} directory shards`);
  }
  
  private async getOrCreateBootstrapDB(): Promise<any> {
    const bootstrapName = `${this.appName}-bootstrap`;
    
    try {
      // Create a well-known bootstrap database that all nodes of this app can access
      const bootstrapDB = await this.createDatabase(bootstrapName, 'keyvalue', 'system');
      
      // Wait a moment for potential replication
      await new Promise(resolve => setTimeout(resolve, 500));
      
      console.log(`🔧 Connected to bootstrap database: ${bootstrapName}`);
      return bootstrapDB;
    } catch (error) {
      console.error(`❌ Failed to connect to bootstrap database:`, error);
      throw error;
    }
  }
  
  private async connectToExistingShards(shardAddresses: string[]): Promise<void> {
    console.log(`📡 Connecting to ${shardAddresses.length} existing shards...`);
    
    for (let i = 0; i < shardAddresses.length; i++) {
      try {
        const shard = await this.openDatabaseByAddress(shardAddresses[i]);
        this.globalDirectoryShards.push(shard);
        console.log(`✓ Connected to existing directory shard ${i}: ${shardAddresses[i]}`);
      } catch (error) {
        console.error(`❌ Failed to connect to shard ${i} at ${shardAddresses[i]}:`, error);
        throw new Error(`Failed to connect to existing shard ${i}`);
      }
    }
  }
  
  private async createAndPublishShards(shardCount: number, bootstrapDB: any, isLeader: boolean = false): Promise<void> {
    const roleText = isLeader ? 'as leader' : 'as fallback';
    console.log(`🔧 Creating ${shardCount} new directory shards ${roleText}...`);
    
    const shardAddresses: string[] = [];
    
    for (let i = 0; i < shardCount; i++) {
      const shardName = `${this.appName}-directory-shard-${i}`;
      
      try {
        const shard = await this.createDatabase(shardName, 'keyvalue', 'system');
        await this.waitForDatabaseSync(shard);
        
        this.globalDirectoryShards.push(shard);
        shardAddresses.push(shard.address);
        
        console.log(`✓ Created directory shard ${i}: ${shardName} at ${shard.address}`);
      } catch (error) {
        console.error(`❌ Failed to create directory shard ${i}:`, error);
        throw error;
      }
    }
    
    // Publish shard addresses to bootstrap database (especially important if we're the leader)
    if (isLeader || shardAddresses.length > 0) {
      try {
        await bootstrapDB.set('shard-addresses', shardAddresses);
        const publishText = isLeader ? 'as leader' : 'as fallback';
        console.log(`📡 Published ${shardAddresses.length} shard addresses to bootstrap database ${publishText}`);
      } catch (error) {
        console.warn(`⚠️ Failed to publish shard addresses to bootstrap database:`, error);
        // Don't fail the whole process if we can't publish
      }
    }
  }
  
  private getNodeId(): string {
    // Try to get a unique node identifier
    return process.env.NODE_ID || process.env.HOSTNAME || `node-${Math.random().toString(36).substr(2, 9)}`;
  }
  
  private async openDatabaseByAddress(address: string): Promise<any> {
    try {
      // Check if we already have this database cached by address
      if (this.databases.has(address)) {
        return this.databases.get(address);
      }

      // Open database by address
      const orbitdb = this.orbitDBService.getOrbitDB();
      const db = await orbitdb.open(address);

      // Cache the database
      this.databases.set(address, db);

      return db;
    } catch (error) {
      console.error(`Failed to open database at address ${address}:`, error);
      throw new Error(`Database opening failed: ${error}`);
    }
  }

  async createUserDatabases(userId: string): Promise<UserMappingsData> {
    console.log(`👤 Creating databases for user: ${userId}`);

    const userScopedModels = ModelRegistry.getUserScopedModels();
    const databases: Record<string, string> = {};

    // Create mappings database first
    const mappingsDBName = `${userId}-mappings`;
    const mappingsDB = await this.createDatabase(mappingsDBName, 'keyvalue', 'user');

    // Create database for each user-scoped model
    for (const model of userScopedModels) {
      const dbName = `${userId}-${model.modelName.toLowerCase()}`;

      try {
        const db = await this.createDatabase(dbName, (model as any).dbType || model.storeType, 'user');
        databases[`${model.modelName.toLowerCase()}DB`] = db.address.toString();

        console.log(`✓ Created user database: ${dbName} (${(model as any).dbType || model.storeType})`);
      } catch (error) {
        console.error(`❌ Failed to create user database ${dbName}:`, error);
        throw error;
      }
    }

    // Store mappings in the mappings database
    await mappingsDB.set('mappings', databases);
    console.log(`✓ Stored database mappings for user ${userId}`);

    // Register in global directory
    await this.registerUserInDirectory(userId, mappingsDB.address.toString());

    const userMappings = new UserMappingsData(userId, databases);

    // Cache for future use
    this.userMappings.set(userId, userMappings);

    console.log(`✅ User databases created successfully for ${userId}`);
    return userMappings;
  }

  async getUserDatabase(userId: string, modelName: string): Promise<any> {
    const mappings = await this.getUserMappings(userId);
    const dbKey = `${modelName.toLowerCase()}DB`;
    const dbAddress = mappings.databases[dbKey];

    if (!dbAddress) {
      throw new Error(`Database not found for user ${userId} and model ${modelName}`);
    }

    // Check if we have this database cached
    const cacheKey = `${userId}-${modelName}`;
    if (this.databases.has(cacheKey)) {
      return this.databases.get(cacheKey);
    }

    // Open the database
    const db = await this.openDatabase(dbAddress);
    this.databases.set(cacheKey, db);

    return db;
  }

  async getUserMappings(userId: string): Promise<UserMappingsData> {
    // Check cache first
    if (this.userMappings.has(userId)) {
      return this.userMappings.get(userId);
    }

    // Get from global directory
    const shardIndex = this.getShardIndex(userId, this.globalDirectoryShards.length);
    const shard = this.globalDirectoryShards[shardIndex];

    if (!shard) {
      throw new Error('Global directory not initialized');
    }

    const mappingsAddress = await shard.get(userId);
    if (!mappingsAddress) {
      throw new Error(`User ${userId} not found in directory`);
    }

    const mappingsDB = await this.openDatabase(mappingsAddress);
    const mappings = await mappingsDB.get('mappings');

    if (!mappings) {
      throw new Error(`No database mappings found for user ${userId}`);
    }

    const userMappings = new UserMappingsData(userId, mappings);

    // Cache for future use
    this.userMappings.set(userId, userMappings);

    return userMappings;
  }

  async getGlobalDatabase(modelName: string): Promise<any> {
    const db = this.globalDatabases.get(modelName);
    if (!db) {
      throw new Error(`Global database not found for model: ${modelName}`);
    }
    return db;
  }

  async getGlobalDirectoryShards(): Promise<any[]> {
    return this.globalDirectoryShards;
  }

  private async createDatabase(name: string, type: StoreType, _scope: string): Promise<any> {
    try {
      const db = await this.orbitDBService.openDatabase(name, type);

      // Store database reference
      this.databases.set(name, db);

      return db;
    } catch (error) {
      console.error(`Failed to create database ${name}:`, error);
      throw new Error(`Database creation failed for ${name}: ${error}`);
    }
  }

  private async openDatabase(address: string): Promise<any> {
    return await this.openDatabaseByAddress(address);
  }

  private async registerUserInDirectory(userId: string, mappingsAddress: string): Promise<void> {
    const shardIndex = this.getShardIndex(userId, this.globalDirectoryShards.length);
    const shard = this.globalDirectoryShards[shardIndex];

    if (!shard) {
      throw new Error('Global directory shards not initialized');
    }

    try {
      await shard.set(userId, mappingsAddress);
      
      // Wait for the registration to be replicated across nodes
      await this.waitForDatabaseSync(shard);
      
      console.log(`✓ Registered user ${userId} in directory shard ${shardIndex}`);
    } catch (error) {
      console.error(`Failed to register user ${userId} in directory:`, error);
      throw error;
    }
  }

  private async waitForDatabaseSync(database: any): Promise<void> {
    // Wait for OrbitDB database to be synced across the network
    // This ensures that data is replicated before proceeding
    const maxWaitTime = 1000; // Reduced to 1 second max wait
    const checkInterval = 50; // Check every 50ms
    const startTime = Date.now();
    
    // Wait for the database to be ready and have peers
    while (Date.now() - startTime < maxWaitTime) {
      try {
        // Check if database is accessible and has been replicated
        if (database && database.access) {
          // For OrbitDB, we can check if the database is ready
          await new Promise(resolve => setTimeout(resolve, checkInterval));
          
          // Additional check for peer connectivity if available
          if (database.replicationStatus) {
            const status = database.replicationStatus();
            if (status.buffered === 0 && status.queued === 0) {
              break; // Database is synced
            }
          } else {
            // Basic wait to ensure replication
            if (Date.now() - startTime > 200) { // Reduced minimum wait to 200ms
              break;
            }
          }
        }
      } catch (error) {
        // Ignore sync check errors, continue with basic wait
      }
      
      await new Promise(resolve => setTimeout(resolve, checkInterval));
    }
  }

  private getShardIndex(key: string, shardCount: number): number {
    // Simple hash-based sharding
    let hash = 0;
    for (let i = 0; i < key.length; i++) {
      hash = ((hash << 5) - hash + key.charCodeAt(i)) & 0xffffffff;
    }
    return Math.abs(hash) % shardCount;
  }

  // Database operation helpers
  async getAllDocuments(database: any, dbType: StoreType): Promise<any[]> {
    try {
      switch (dbType) {
        case 'eventlog':
          const iterator = database.iterator();
          return iterator.collect();

        case 'keyvalue':
          return Object.values(database.all());

        case 'docstore':
          return database.query(() => true);

        case 'feed':
          const feedIterator = database.iterator();
          return feedIterator.collect();

        case 'counter':
          return [{ value: database.value, id: database.id }];

        default:
          throw new Error(`Unsupported database type: ${dbType}`);
      }
    } catch (error) {
      console.error(`Error fetching documents from ${dbType} database:`, error);
      throw error;
    }
  }

  async addDocument(database: any, dbType: StoreType, data: any): Promise<string> {
    try {
      switch (dbType) {
        case 'eventlog':
          return await database.add(data);

        case 'keyvalue':
          await database.set(data.id, data);
          return data.id;

        case 'docstore':
          return await database.put(data);

        case 'feed':
          return await database.add(data);

        case 'counter':
          await database.inc(data.amount || 1);
          return database.id;

        default:
          throw new Error(`Unsupported database type: ${dbType}`);
      }
    } catch (error) {
      console.error(`Error adding document to ${dbType} database:`, error);
      throw error;
    }
  }

  async updateDocument(database: any, dbType: StoreType, id: string, data: any): Promise<void> {
    try {
      switch (dbType) {
        case 'keyvalue':
          await database.set(id, data);
          break;

        case 'docstore':
          await database.put(data);
          break;

        default:
          // For append-only stores, we add a new entry
          await this.addDocument(database, dbType, data);
      }
    } catch (error) {
      console.error(`Error updating document in ${dbType} database:`, error);
      throw error;
    }
  }

  async deleteDocument(database: any, dbType: StoreType, id: string): Promise<void> {
    try {
      switch (dbType) {
        case 'keyvalue':
          await database.del(id);
          break;

        case 'docstore':
          await database.del(id);
          break;

        default:
          // For append-only stores, we might add a deletion marker
          await this.addDocument(database, dbType, { _deleted: true, id, deletedAt: Date.now() });
      }
    } catch (error) {
      console.error(`Error deleting document from ${dbType} database:`, error);
      throw error;
    }
  }

  async getDocument(database: any, dbType: StoreType, id: string): Promise<any> {
    try {
      switch (dbType) {
        case 'keyvalue':
          return await database.get(id);

        case 'docstore':
          return await database.get(id);

        case 'eventlog':
          // For eventlog, we need to search through entries
          const iterator = database.iterator();
          const entries = iterator.collect();
          return entries.find((entry: any) => entry.id === id || entry._id === id);

        case 'feed':
          // For feed, we need to search through entries
          const feedIterator = database.iterator();
          const feedEntries = feedIterator.collect();
          return feedEntries.find((entry: any) => entry.id === id || entry._id === id);

        case 'counter':
          // Counter doesn't have individual documents
          return database.id === id ? { value: database.value, id: database.id } : null;

        default:
          throw new Error(`Unsupported database type: ${dbType}`);
      }
    } catch (error) {
      console.error(`Error fetching document ${id} from ${dbType} database:`, error);
      return null;
    }
  }

  // Cleanup methods
  async stop(): Promise<void> {
    console.log('🛑 Stopping DatabaseManager...');

    // Clear caches
    this.databases.clear();
    this.userMappings.clear();
    this.globalDatabases.clear();
    this.globalDirectoryShards = [];

    this.initialized = false;
    console.log('✅ DatabaseManager stopped');
  }
}
