/**
 * DebrosFramework - Main Framework Class
 *
 * This is the primary entry point for the DebrosFramework, providing a unified
 * API that integrates all framework components:
 * - Model system with decorators and validation
 * - Database management and sharding
 * - Query system with optimization
 * - Relationship management with lazy/eager loading
 * - Automatic pinning and PubSub features
 * - Migration system for schema evolution
 * - Configuration and lifecycle management
 */

import { BaseModel } from './models/BaseModel';
import { ModelRegistry } from './core/ModelRegistry';
import { DatabaseManager } from './core/DatabaseManager';
import { ShardManager } from './sharding/ShardManager';
import { ConfigManager } from './core/ConfigManager';
import { FrameworkOrbitDBService, FrameworkIPFSService } from './services/OrbitDBService';
import { QueryCache } from './query/QueryCache';
import { RelationshipManager } from './relationships/RelationshipManager';
import { PinningManager } from './pinning/PinningManager';
import { PubSubManager } from './pubsub/PubSubManager';
import { MigrationManager } from './migrations/MigrationManager';
import { FrameworkConfig } from './types/framework';

export interface DebrosFrameworkConfig extends FrameworkConfig {
  // Environment settings
  environment?: 'development' | 'production' | 'test';

  // Service configurations
  orbitdb?: {
    directory?: string;
    options?: any;
  };

  ipfs?: {
    config?: any;
    options?: any;
  };

  // Feature toggles
  features?: {
    autoMigration?: boolean;
    automaticPinning?: boolean;
    pubsub?: boolean;
    queryCache?: boolean;
    relationshipCache?: boolean;
  };

  // Performance settings
  performance?: {
    queryTimeout?: number;
    migrationTimeout?: number;
    maxConcurrentOperations?: number;
    batchSize?: number;
  };

  // Monitoring and logging
  monitoring?: {
    enableMetrics?: boolean;
    logLevel?: 'error' | 'warn' | 'info' | 'debug';
    metricsInterval?: number;
  };
}

export interface FrameworkMetrics {
  uptime: number;
  totalModels: number;
  totalDatabases: number;
  totalShards: number;
  queriesExecuted: number;
  migrationsRun: number;
  cacheHitRate: number;
  averageQueryTime: number;
  memoryUsage: {
    queryCache: number;
    relationshipCache: number;
    total: number;
  };
  performance: {
    slowQueries: number;
    failedOperations: number;
    averageResponseTime: number;
  };
}

export interface FrameworkStatus {
  initialized: boolean;
  healthy: boolean;
  version: string;
  environment: string;
  services: {
    orbitdb: 'connected' | 'disconnected' | 'error';
    ipfs: 'connected' | 'disconnected' | 'error';
    pinning: 'active' | 'inactive' | 'error';
    pubsub: 'active' | 'inactive' | 'error';
  };
  lastHealthCheck: number;
}

export class DebrosFramework {
  private config: DebrosFrameworkConfig;
  private configManager: ConfigManager;

  // Core services
  private orbitDBService: FrameworkOrbitDBService | null = null;
  private ipfsService: FrameworkIPFSService | null = null;

  // Framework components
  private databaseManager: DatabaseManager | null = null;
  private shardManager: ShardManager | null = null;
  private queryCache: QueryCache | null = null;
  private relationshipManager: RelationshipManager | null = null;
  private pinningManager: PinningManager | null = null;
  private pubsubManager: PubSubManager | null = null;
  private migrationManager: MigrationManager | null = null;

  // Framework state
  private initialized: boolean = false;
  private startTime: number = 0;
  private healthCheckInterval: any = null;
  private metricsCollector: any = null;
  private status: FrameworkStatus;
  private metrics: FrameworkMetrics;

  constructor(config: DebrosFrameworkConfig = {}) {
    this.config = this.mergeDefaultConfig(config);
    this.configManager = new ConfigManager(this.config);

    this.status = {
      initialized: false,
      healthy: false,
      version: '1.0.0', // This would come from package.json
      environment: this.config.environment || 'development',
      services: {
        orbitdb: 'disconnected',
        ipfs: 'disconnected',
        pinning: 'inactive',
        pubsub: 'inactive',
      },
      lastHealthCheck: 0,
    };

    this.metrics = {
      uptime: 0,
      totalModels: 0,
      totalDatabases: 0,
      totalShards: 0,
      queriesExecuted: 0,
      migrationsRun: 0,
      cacheHitRate: 0,
      averageQueryTime: 0,
      memoryUsage: {
        queryCache: 0,
        relationshipCache: 0,
        total: 0,
      },
      performance: {
        slowQueries: 0,
        failedOperations: 0,
        averageResponseTime: 0,
      },
    };
  }

  // Main initialization method
  async initialize(
    existingOrbitDBService?: any,
    existingIPFSService?: any,
    overrideConfig?: Partial<DebrosFrameworkConfig>,
  ): Promise<void> {
    if (this.initialized) {
      throw new Error('Framework is already initialized');
    }

    try {
      this.startTime = Date.now();
      console.log('🚀 Initializing DebrosFramework...');

      // Apply config overrides
      if (overrideConfig) {
        this.config = { ...this.config, ...overrideConfig };
        this.configManager = new ConfigManager(this.config);
        // Update status to reflect config changes
        this.status.environment = this.config.environment || 'development';
      }

      // Initialize services
      await this.initializeServices(existingOrbitDBService, existingIPFSService);

      // Initialize core components
      await this.initializeCoreComponents();

      // Initialize feature components
      await this.initializeFeatureComponents();

      // Setup global framework access
      this.setupGlobalAccess();

      // Start background processes
      await this.startBackgroundProcesses();

      // Run automatic migrations if enabled
      if (this.config.features?.autoMigration && this.migrationManager) {
        await this.runAutomaticMigrations();
      }

      this.initialized = true;
      this.status.initialized = true;
      this.status.healthy = true;

      console.log('✅ DebrosFramework initialized successfully');
      this.logFrameworkInfo();
    } catch (error) {
      console.error('❌ Framework initialization failed:', error);
      await this.cleanup();
      throw error;
    }
  }

  // Service initialization
  private async initializeServices(
    existingOrbitDBService?: any,
    existingIPFSService?: any,
  ): Promise<void> {
    console.log('📡 Initializing core services...');

    try {
      // Initialize IPFS service
      if (existingIPFSService) {
        this.ipfsService = new FrameworkIPFSService(existingIPFSService);
      } else {
        // In a real implementation, create IPFS instance
        throw new Error('IPFS service is required. Please provide an existing IPFS instance.');
      }

      await this.ipfsService.init();
      this.status.services.ipfs = 'connected';
      console.log('✅ IPFS service initialized');

      // Initialize OrbitDB service
      if (existingOrbitDBService) {
        this.orbitDBService = new FrameworkOrbitDBService(existingOrbitDBService);
      } else {
        // In a real implementation, create OrbitDB instance
        throw new Error(
          'OrbitDB service is required. Please provide an existing OrbitDB instance.',
        );
      }

      await this.orbitDBService.init();
      this.status.services.orbitdb = 'connected';
      console.log('✅ OrbitDB service initialized');
    } catch (error) {
      this.status.services.ipfs = 'error';
      this.status.services.orbitdb = 'error';
      throw new Error(`Service initialization failed: ${error}`);
    }
  }

  // Core component initialization
  private async initializeCoreComponents(): Promise<void> {
    console.log('🔧 Initializing core components...');

    // Database Manager
    this.databaseManager = new DatabaseManager(this.orbitDBService!);
    await this.databaseManager.initializeAllDatabases();
    console.log('✅ DatabaseManager initialized');

    // Shard Manager
    this.shardManager = new ShardManager();
    this.shardManager.setOrbitDBService(this.orbitDBService!);

    // Initialize shards for registered models
    const globalModels = ModelRegistry.getGlobalModels();
    for (const model of globalModels) {
      if (model.sharding) {
        await this.shardManager.createShards(model.modelName, model.sharding, model.storeType);
      }
    }
    console.log('✅ ShardManager initialized');

    // Query Cache
    if (this.config.features?.queryCache !== false) {
      const cacheConfig = this.configManager.cacheConfig;
      this.queryCache = new QueryCache(cacheConfig?.maxSize || 1000, cacheConfig?.ttl || 300000);
      console.log('✅ QueryCache initialized');
    }

    // Relationship Manager
    this.relationshipManager = new RelationshipManager({
      databaseManager: this.databaseManager,
      shardManager: this.shardManager,
      queryCache: this.queryCache,
    });
    console.log('✅ RelationshipManager initialized');
  }

  // Feature component initialization
  private async initializeFeatureComponents(): Promise<void> {
    console.log('🎛️  Initializing feature components...');

    // Pinning Manager
    if (this.config.features?.automaticPinning !== false) {
      this.pinningManager = new PinningManager(this.ipfsService!.getHelia(), {
        maxTotalPins: this.config.performance?.maxConcurrentOperations || 10000,
        cleanupIntervalMs: 60000,
      });

      // Setup default pinning rules based on config
      if (this.config.defaultPinning) {
        const globalModels = ModelRegistry.getGlobalModels();
        for (const model of globalModels) {
          this.pinningManager.setPinningRule(model.modelName, this.config.defaultPinning);
        }
      }

      this.status.services.pinning = 'active';
      console.log('✅ PinningManager initialized');
    }

    // PubSub Manager
    if (this.config.features?.pubsub !== false) {
      this.pubsubManager = new PubSubManager(this.ipfsService!.getHelia(), {
        enabled: true,
        autoPublishModelEvents: true,
        autoPublishDatabaseEvents: true,
        topicPrefix: `debros-${this.config.environment || 'dev'}`,
      });

      await this.pubsubManager.initialize();
      this.status.services.pubsub = 'active';
      console.log('✅ PubSubManager initialized');
    }

    // Migration Manager
    this.migrationManager = new MigrationManager(
      this.databaseManager,
      this.shardManager,
      this.createMigrationLogger(),
    );
    console.log('✅ MigrationManager initialized');
  }

  // Setup global framework access for models
  private setupGlobalAccess(): void {
    (globalThis as any).__debrosFramework = {
      databaseManager: this.databaseManager,
      shardManager: this.shardManager,
      configManager: this.configManager,
      queryCache: this.queryCache,
      relationshipManager: this.relationshipManager,
      pinningManager: this.pinningManager,
      pubsubManager: this.pubsubManager,
      migrationManager: this.migrationManager,
      framework: this,
    };
  }

  // Start background processes
  private async startBackgroundProcesses(): Promise<void> {
    console.log('⚙️  Starting background processes...');

    // Health check interval
    this.healthCheckInterval = setInterval(() => {
      this.performHealthCheck();
    }, 30000); // Every 30 seconds

    // Metrics collection
    if (this.config.monitoring?.enableMetrics !== false) {
      this.metricsCollector = setInterval(() => {
        this.collectMetrics();
      }, this.config.monitoring?.metricsInterval || 60000); // Every minute
    }

    console.log('✅ Background processes started');
  }

  // Automatic migration execution
  private async runAutomaticMigrations(): Promise<void> {
    if (!this.migrationManager) return;

    try {
      console.log('🔄 Running automatic migrations...');

      const pendingMigrations = this.migrationManager.getPendingMigrations();
      if (pendingMigrations.length > 0) {
        console.log(`Found ${pendingMigrations.length} pending migrations`);

        const results = await this.migrationManager.runPendingMigrations({
          stopOnError: true,
          batchSize: this.config.performance?.batchSize || 100,
        });

        const successful = results.filter((r) => r.success).length;
        console.log(`✅ Completed ${successful}/${results.length} migrations`);

        this.metrics.migrationsRun += successful;
      } else {
        console.log('No pending migrations found');
      }
    } catch (error) {
      console.error('❌ Automatic migration failed:', error);
      if (this.config.environment === 'production') {
        // In production, don't fail initialization due to migration errors
        console.warn('Continuing initialization despite migration failure');
      } else {
        throw error;
      }
    }
  }

  // Public API methods

  // Model registration
  registerModel(modelClass: typeof BaseModel, config?: any): void {
    ModelRegistry.register(modelClass.name, modelClass, config || {});
    console.log(`📝 Registered model: ${modelClass.name}`);

    this.metrics.totalModels = ModelRegistry.getModelNames().length;
  }

  // Get model instance
  getModel(modelName: string): typeof BaseModel | null {
    return ModelRegistry.get(modelName) || null;
  }

  // Database operations
  async createUserDatabase(userId: string): Promise<void> {
    if (!this.databaseManager) {
      throw new Error('Framework not initialized');
    }

    await this.databaseManager.createUserDatabases(userId);
    this.metrics.totalDatabases++;
  }

  async getUserDatabase(userId: string, modelName: string): Promise<any> {
    if (!this.databaseManager) {
      throw new Error('Framework not initialized');
    }

    return await this.databaseManager.getUserDatabase(userId, modelName);
  }

  async getGlobalDatabase(modelName: string): Promise<any> {
    if (!this.databaseManager) {
      throw new Error('Framework not initialized');
    }

    return await this.databaseManager.getGlobalDatabase(modelName);
  }

  // Migration operations
  async runMigration(migrationId: string, options?: any): Promise<any> {
    if (!this.migrationManager) {
      throw new Error('MigrationManager not initialized');
    }

    const result = await this.migrationManager.runMigration(migrationId, options);
    this.metrics.migrationsRun++;
    return result;
  }

  async registerMigration(migration: any): Promise<void> {
    if (!this.migrationManager) {
      throw new Error('MigrationManager not initialized');
    }

    this.migrationManager.registerMigration(migration);
  }

  getPendingMigrations(modelName?: string): any[] {
    if (!this.migrationManager) {
      return [];
    }

    return this.migrationManager.getPendingMigrations(modelName);
  }

  // Cache management
  clearQueryCache(): void {
    if (this.queryCache) {
      this.queryCache.clear();
    }
  }

  clearRelationshipCache(): void {
    if (this.relationshipManager) {
      this.relationshipManager.clearRelationshipCache();
    }
  }

  async warmupCaches(): Promise<void> {
    console.log('🔥 Warming up caches...');

    if (this.queryCache) {
      // Warm up common queries
      const commonQueries: any[] = []; // Would be populated with actual queries
      await this.queryCache.warmup(commonQueries);
    }

    if (this.relationshipManager && this.pinningManager) {
      // Warm up relationship cache for popular content
      // Implementation would depend on actual models
    }

    console.log('✅ Cache warmup completed');
  }

  // Health and monitoring
  performHealthCheck(): void {
    try {
      this.status.lastHealthCheck = Date.now();

      // Check service health
      this.status.services.orbitdb = this.orbitDBService ? 'connected' : 'disconnected';
      this.status.services.ipfs = this.ipfsService ? 'connected' : 'disconnected';
      this.status.services.pinning = this.pinningManager ? 'active' : 'inactive';
      this.status.services.pubsub = this.pubsubManager ? 'active' : 'inactive';

      // Overall health check - only require core services to be healthy
      const coreServicesHealthy = 
        this.status.services.orbitdb === 'connected' && 
        this.status.services.ipfs === 'connected';

      this.status.healthy = this.initialized && coreServicesHealthy;
    } catch (error) {
      console.error('Health check failed:', error);
      this.status.healthy = false;
    }
  }

  collectMetrics(): void {
    try {
      this.metrics.uptime = Date.now() - this.startTime;
      this.metrics.totalModels = ModelRegistry.getModelNames().length;

      if (this.queryCache) {
        const cacheStats = this.queryCache.getStats();
        this.metrics.cacheHitRate = cacheStats.hitRate;
        this.metrics.averageQueryTime = 0; // Would need to be calculated from cache stats
        this.metrics.memoryUsage.queryCache = cacheStats.size * 1024; // Estimate
      }

      if (this.relationshipManager) {
        const relStats = this.relationshipManager.getRelationshipCacheStats();
        this.metrics.memoryUsage.relationshipCache = relStats.cache.memoryUsage;
      }

      this.metrics.memoryUsage.total =
        this.metrics.memoryUsage.queryCache + this.metrics.memoryUsage.relationshipCache;
    } catch (error) {
      console.error('Metrics collection failed:', error);
    }
  }

  getStatus(): FrameworkStatus {
    return { ...this.status };
  }

  getMetrics(): FrameworkMetrics {
    this.collectMetrics(); // Ensure fresh metrics
    return { ...this.metrics };
  }

  getConfig(): DebrosFrameworkConfig {
    return { ...this.config };
  }

  // Component access
  getDatabaseManager(): DatabaseManager | null {
    return this.databaseManager;
  }

  getShardManager(): ShardManager | null {
    return this.shardManager;
  }

  getRelationshipManager(): RelationshipManager | null {
    return this.relationshipManager;
  }

  getPinningManager(): PinningManager | null {
    return this.pinningManager;
  }

  getPubSubManager(): PubSubManager | null {
    return this.pubsubManager;
  }

  getMigrationManager(): MigrationManager | null {
    return this.migrationManager;
  }

  getQueryCache(): QueryCache | null {
    return this.queryCache;
  }

  getOrbitDBService(): FrameworkOrbitDBService | null {
    return this.orbitDBService;
  }

  getIPFSService(): FrameworkIPFSService | null {
    return this.ipfsService;
  }

  getConfigManager(): ConfigManager | null {
    return this.configManager;
  }

  async healthCheck(): Promise<any> {
    this.performHealthCheck();
    return {
      healthy: this.status.healthy,
      services: { ...this.status.services },
      lastCheck: this.status.lastHealthCheck
    };
  }

  // Framework lifecycle
  async stop(): Promise<void> {
    if (!this.initialized) {
      return;
    }

    console.log('🛑 Stopping DebrosFramework...');

    try {
      await this.cleanup();
      this.initialized = false;
      this.status.initialized = false;
      this.status.healthy = false;

      console.log('✅ DebrosFramework stopped successfully');
    } catch (error) {
      console.error('❌ Error during framework shutdown:', error);
      throw error;
    }
  }

  async restart(newConfig?: Partial<DebrosFrameworkConfig>): Promise<void> {
    console.log('🔄 Restarting DebrosFramework...');

    const orbitDB = this.orbitDBService?.getOrbitDB();
    const ipfs = this.ipfsService?.getHelia();

    await this.stop();

    if (newConfig) {
      this.config = { ...this.config, ...newConfig };
    }

    await this.initialize(orbitDB, ipfs);
  }

  // Cleanup method
  private async cleanup(): Promise<void> {
    // Stop background processes
    if (this.healthCheckInterval) {
      clearInterval(this.healthCheckInterval);
      this.healthCheckInterval = null;
    }

    if (this.metricsCollector) {
      clearInterval(this.metricsCollector);
      this.metricsCollector = null;
    }

    // Cleanup components
    if (this.pubsubManager) {
      await this.pubsubManager.shutdown();
    }

    if (this.pinningManager) {
      await this.pinningManager.shutdown();
    }

    if (this.migrationManager) {
      await this.migrationManager.cleanup();
    }

    if (this.queryCache) {
      this.queryCache.clear();
    }

    if (this.relationshipManager) {
      this.relationshipManager.clearRelationshipCache();
    }

    if (this.databaseManager) {
      await this.databaseManager.stop();
    }

    if (this.shardManager) {
      await this.shardManager.stop();
    }

    // Clear global access
    delete (globalThis as any).__debrosFramework;
  }

  // Utility methods
  private mergeDefaultConfig(config: DebrosFrameworkConfig): DebrosFrameworkConfig {
    return {
      environment: 'development',
      features: {
        autoMigration: true,
        automaticPinning: true,
        pubsub: true,
        queryCache: true,
        relationshipCache: true,
      },
      performance: {
        queryTimeout: 30000,
        migrationTimeout: 300000,
        maxConcurrentOperations: 100,
        batchSize: 100,
      },
      monitoring: {
        enableMetrics: true,
        logLevel: 'info',
        metricsInterval: 60000,
      },
      ...config,
    };
  }

  private createMigrationLogger(): any {
    const logLevel = this.config.monitoring?.logLevel || 'info';

    return {
      info: (message: string, meta?: any) => {
        if (['info', 'debug'].includes(logLevel)) {
          console.log(`[MIGRATION INFO] ${message}`, meta || '');
        }
      },
      warn: (message: string, meta?: any) => {
        if (['warn', 'info', 'debug'].includes(logLevel)) {
          console.warn(`[MIGRATION WARN] ${message}`, meta || '');
        }
      },
      error: (message: string, meta?: any) => {
        console.error(`[MIGRATION ERROR] ${message}`, meta || '');
      },
      debug: (message: string, meta?: any) => {
        if (logLevel === 'debug') {
          console.log(`[MIGRATION DEBUG] ${message}`, meta || '');
        }
      },
    };
  }

  private logFrameworkInfo(): void {
    console.log('\n📋 DebrosFramework Information:');
    console.log('==============================');
    console.log(`Version: ${this.status.version}`);
    console.log(`Environment: ${this.status.environment}`);
    console.log(`Models registered: ${this.metrics.totalModels}`);
    console.log(
      `Services: ${Object.entries(this.status.services)
        .map(([name, status]) => `${name}:${status}`)
        .join(', ')}`,
    );
    console.log(
      `Features enabled: ${Object.entries(this.config.features || {})
        .filter(([, enabled]) => enabled)
        .map(([feature]) => feature)
        .join(', ')}`,
    );
    console.log('');
  }

  // Static factory methods
  static async create(config: DebrosFrameworkConfig = {}): Promise<DebrosFramework> {
    const framework = new DebrosFramework(config);
    return framework;
  }

  static async createWithServices(
    orbitDBService: any,
    ipfsService: any,
    config: DebrosFrameworkConfig = {},
  ): Promise<DebrosFramework> {
    const framework = new DebrosFramework(config);
    await framework.initialize(orbitDBService, ipfsService);
    return framework;
  }
}

// Export the main framework class as default
export default DebrosFramework;
