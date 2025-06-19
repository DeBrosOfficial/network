import { ShardingConfig, StoreType } from '../types/framework';
import { FrameworkOrbitDBService } from '../services/OrbitDBService';

export interface ShardInfo {
  name: string;
  index: number;
  database: any;
  address: string;
}

export class ShardManager {
  private orbitDBService?: FrameworkOrbitDBService;
  private shards: Map<string, ShardInfo[]> = new Map();
  private shardConfigs: Map<string, ShardingConfig> = new Map();

  setOrbitDBService(service: FrameworkOrbitDBService): void {
    this.orbitDBService = service;
  }

  async createShards(
    modelName: string,
    config: ShardingConfig,
    dbType: StoreType = 'docstore',
  ): Promise<void> {
    if (!this.orbitDBService) {
      throw new Error('OrbitDB service not initialized');
    }

    console.log(`🔀 Creating ${config.count} shards for model: ${modelName}`);

    const shards: ShardInfo[] = [];
    this.shardConfigs.set(modelName, config);

    for (let i = 0; i < config.count; i++) {
      const shardName = `${modelName.toLowerCase()}-shard-${i}`;

      try {
        const shard = await this.createShard(shardName, i, dbType);
        shards.push(shard);

        console.log(`✓ Created shard: ${shardName} (${shard.address})`);
      } catch (error) {
        console.error(`❌ Failed to create shard ${shardName}:`, error);
        throw error;
      }
    }

    this.shards.set(modelName, shards);
    console.log(`✅ Created ${shards.length} shards for ${modelName}`);
  }

  getShardForKey(modelName: string, key: string): ShardInfo {
    const shards = this.shards.get(modelName);
    if (!shards || shards.length === 0) {
      throw new Error(`No shards found for model ${modelName}`);
    }

    const config = this.shardConfigs.get(modelName);
    if (!config) {
      throw new Error(`No shard configuration found for model ${modelName}`);
    }

    const shardIndex = this.calculateShardIndex(key, shards.length, config.strategy);
    return shards[shardIndex];
  }

  getAllShards(modelName: string): ShardInfo[] {
    return this.shards.get(modelName) || [];
  }

  getShardByIndex(modelName: string, index: number): ShardInfo | undefined {
    const shards = this.shards.get(modelName);
    if (!shards || index < 0 || index >= shards.length) {
      return undefined;
    }
    return shards[index];
  }

  getShardCount(modelName: string): number {
    const shards = this.shards.get(modelName);
    return shards ? shards.length : 0;
  }

  private calculateShardIndex(
    key: string,
    shardCount: number,
    strategy: ShardingConfig['strategy'],
  ): number {
    switch (strategy) {
      case 'hash':
        return this.hashSharding(key, shardCount);

      case 'range':
        return this.rangeSharding(key, shardCount);

      case 'user':
        return this.userSharding(key, shardCount);

      default:
        throw new Error(`Unsupported sharding strategy: ${strategy}`);
    }
  }

  private hashSharding(key: string, shardCount: number): number {
    // Consistent hash-based sharding
    let hash = 0;
    for (let i = 0; i < key.length; i++) {
      hash = ((hash << 5) - hash + key.charCodeAt(i)) & 0xffffffff;
    }
    return Math.abs(hash) % shardCount;
  }

  private rangeSharding(key: string, shardCount: number): number {
    // Range-based sharding (alphabetical)
    const firstChar = key.charAt(0).toLowerCase();
    const charCode = firstChar.charCodeAt(0);

    // Map a-z (97-122) to shard indices
    const normalizedCode = Math.max(97, Math.min(122, charCode));
    const range = (normalizedCode - 97) / 25; // 0-1 range

    const shardIndex = Math.floor(range * shardCount);
    // Ensure the index is within bounds (handle edge case where range = 1.0)
    return Math.min(shardIndex, shardCount - 1);
  }

  private userSharding(key: string, shardCount: number): number {
    // User-based sharding - similar to hash but optimized for user IDs
    return this.hashSharding(key, shardCount);
  }

  private async createShard(
    shardName: string,
    index: number,
    dbType: StoreType,
  ): Promise<ShardInfo> {
    if (!this.orbitDBService) {
      throw new Error('OrbitDB service not initialized');
    }

    const database = await this.orbitDBService.openDatabase(shardName, dbType);

    return {
      name: shardName,
      index,
      database,
      address: database.address.toString(),
    };
  }

  // Global indexing support
  async createGlobalIndex(modelName: string, indexName: string): Promise<void> {
    if (!this.orbitDBService) {
      throw new Error('OrbitDB service not initialized');
    }

    console.log(`📇 Creating global index: ${indexName} for model: ${modelName}`);

    // Create sharded global index
    const INDEX_SHARD_COUNT = 4; // Configurable
    const indexShards: ShardInfo[] = [];

    for (let i = 0; i < INDEX_SHARD_COUNT; i++) {
      const indexShardName = `${indexName}-shard-${i}`;

      try {
        const shard = await this.createShard(indexShardName, i, 'keyvalue');
        indexShards.push(shard);

        console.log(`✓ Created index shard: ${indexShardName}`);
      } catch (error) {
        console.error(`❌ Failed to create index shard ${indexShardName}:`, error);
        throw error;
      }
    }

    // Store index shards
    this.shards.set(indexName, indexShards);

    console.log(`✅ Created global index ${indexName} with ${indexShards.length} shards`);
  }

  async addToGlobalIndex(indexName: string, key: string, value: any): Promise<void> {
    const indexShards = this.shards.get(indexName);
    if (!indexShards) {
      throw new Error(`Global index ${indexName} not found`);
    }

    // Determine which shard to use for this key
    const shardIndex = this.hashSharding(key, indexShards.length);
    const shard = indexShards[shardIndex];

    try {
      // For keyvalue stores, we use set
      await shard.database.set(key, value);
    } catch (error) {
      console.error(`Failed to add to global index ${indexName}:`, error);
      throw error;
    }
  }

  async getFromGlobalIndex(indexName: string, key: string): Promise<any> {
    const indexShards = this.shards.get(indexName);
    if (!indexShards) {
      throw new Error(`Global index ${indexName} not found`);
    }

    // Determine which shard contains this key
    const shardIndex = this.hashSharding(key, indexShards.length);
    const shard = indexShards[shardIndex];

    try {
      return await shard.database.get(key);
    } catch (error) {
      console.error(`Failed to get from global index ${indexName}:`, error);
      return null;
    }
  }

  async removeFromGlobalIndex(indexName: string, key: string): Promise<void> {
    const indexShards = this.shards.get(indexName);
    if (!indexShards) {
      throw new Error(`Global index ${indexName} not found`);
    }

    // Determine which shard contains this key
    const shardIndex = this.hashSharding(key, indexShards.length);
    const shard = indexShards[shardIndex];

    try {
      await shard.database.del(key);
    } catch (error) {
      console.error(`Failed to remove from global index ${indexName}:`, error);
      throw error;
    }
  }

  // Query all shards for a model
  async queryAllShards(
    modelName: string,
    queryFn: (database: any) => Promise<any[]>,
  ): Promise<any[]> {
    const shards = this.shards.get(modelName);
    if (!shards) {
      throw new Error(`No shards found for model ${modelName}`);
    }

    const results: any[] = [];

    // Query all shards in parallel
    const promises = shards.map(async (shard) => {
      try {
        return await queryFn(shard.database);
      } catch (error) {
        console.warn(`Query failed on shard ${shard.name}:`, error);
        return [];
      }
    });

    const shardResults = await Promise.all(promises);

    // Flatten results
    for (const shardResult of shardResults) {
      results.push(...shardResult);
    }

    return results;
  }

  // Statistics and monitoring
  getShardStatistics(modelName: string): any {
    const shards = this.shards.get(modelName);
    if (!shards) {
      return null;
    }

    return {
      modelName,
      shardCount: shards.length,
      shards: shards.map((shard) => ({
        name: shard.name,
        index: shard.index,
        address: shard.address,
      })),
    };
  }

  getAllModelsWithShards(): string[] {
    return Array.from(this.shards.keys());
  }

  // Cleanup
  async stop(): Promise<void> {
    console.log('🛑 Stopping ShardManager...');

    this.shards.clear();
    this.shardConfigs.clear();

    console.log('✅ ShardManager stopped');
  }
}
