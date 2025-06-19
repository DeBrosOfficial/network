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

  constructor(orbitDBService: FrameworkOrbitDBService) {
    this.orbitDBService = orbitDBService;
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
        const db = await this.createDatabase(dbName, model.dbType, 'global');
        this.globalDatabases.set(model.modelName, db);

        console.log(`✓ Created global database: ${dbName} (${model.dbType})`);
      } catch (error) {
        console.error(`❌ Failed to create global database ${dbName}:`, error);
        throw error;
      }
    }
  }

  private async initializeSystemDatabases(): Promise<void> {
    console.log('🔧 Creating system databases...');

    // Create global user directory shards
    const DIRECTORY_SHARD_COUNT = 4; // Configurable

    for (let i = 0; i < DIRECTORY_SHARD_COUNT; i++) {
      const shardName = `global-user-directory-shard-${i}`;
      try {
        const shard = await this.createDatabase(shardName, 'keyvalue', 'system');
        this.globalDirectoryShards.push(shard);

        console.log(`✓ Created directory shard: ${shardName}`);
      } catch (error) {
        console.error(`❌ Failed to create directory shard ${shardName}:`, error);
        throw error;
      }
    }

    console.log(`✅ Created ${this.globalDirectoryShards.length} directory shards`);
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
        const db = await this.createDatabase(dbName, model.dbType, 'user');
        databases[`${model.modelName.toLowerCase()}DB`] = db.address.toString();

        console.log(`✓ Created user database: ${dbName} (${model.dbType})`);
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
    try {
      // Check if we already have this database cached by address
      if (this.databases.has(address)) {
        return this.databases.get(address);
      }

      // Open database by address (implementation may vary based on OrbitDB version)
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

  private async registerUserInDirectory(userId: string, mappingsAddress: string): Promise<void> {
    const shardIndex = this.getShardIndex(userId, this.globalDirectoryShards.length);
    const shard = this.globalDirectoryShards[shardIndex];

    if (!shard) {
      throw new Error('Global directory shards not initialized');
    }

    try {
      await shard.set(userId, mappingsAddress);
      console.log(`✓ Registered user ${userId} in directory shard ${shardIndex}`);
    } catch (error) {
      console.error(`Failed to register user ${userId} in directory:`, error);
      throw error;
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
