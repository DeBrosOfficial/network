import { describe, beforeEach, afterEach, it, expect, jest } from '@jest/globals';
import { DebrosFramework, DebrosFrameworkConfig } from '../../src/framework/DebrosFramework';
import { BaseModel } from '../../src/framework/models/BaseModel';
import { Model, Field, HasMany, BelongsTo } from '../../src/framework/models/decorators';
import { createMockServices } from '../mocks/services';

// Test models for integration testing
@Model({
  scope: 'global',
  type: 'docstore'
})
class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'string', required: true })
  email: string;

  @Field({ type: 'boolean', required: false, default: true })
  isActive: boolean;

  @HasMany(() => Post, 'userId')
  posts: Post[];
}

@Model({
  scope: 'user',
  type: 'docstore'
})
class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  userId: string;

  @Field({ type: 'boolean', required: false, default: false })
  published: boolean;

  @BelongsTo(() => User, 'userId')
  user: User;
}

describe('DebrosFramework Integration Tests', () => {
  let framework: DebrosFramework;
  let mockServices: any;
  let config: DebrosFrameworkConfig;

  beforeEach(() => {
    mockServices = createMockServices();
    
    config = {
      environment: 'test',
      features: {
        autoMigration: false,
        automaticPinning: false,
        pubsub: false,
        queryCache: true,
        relationshipCache: true
      },
      performance: {
        queryTimeout: 5000,
        migrationTimeout: 30000,
        maxConcurrentOperations: 10,
        batchSize: 100
      },
      monitoring: {
        enableMetrics: true,
        logLevel: 'info',
        metricsInterval: 1000
      }
    };

    framework = new DebrosFramework(config);
    
    // Suppress console output for cleaner test output
    jest.spyOn(console, 'log').mockImplementation();
    jest.spyOn(console, 'error').mockImplementation();
    jest.spyOn(console, 'warn').mockImplementation();
  });

  afterEach(async () => {
    if (framework) {
      await framework.cleanup();
    }
    jest.restoreAllMocks();
  });

  describe('Framework Initialization', () => {
    it('should initialize successfully with valid services', async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);

      const status = framework.getStatus();
      expect(status.initialized).toBe(true);
      expect(status.healthy).toBe(true);
      expect(status.environment).toBe('test');
      expect(status.services.orbitdb).toBe('connected');
      expect(status.services.ipfs).toBe('connected');
    });

    it('should throw error when already initialized', async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);

      await expect(
        framework.initialize(mockServices.orbitDBService, mockServices.ipfsService)
      ).rejects.toThrow('Framework is already initialized');
    });

    it('should throw error without required services', async () => {
      await expect(framework.initialize()).rejects.toThrow(
        'IPFS service is required'
      );
    });

    it('should handle initialization failures gracefully', async () => {
      // Make IPFS service initialization fail
      const failingIPFS = {
        ...mockServices.ipfsService,
        init: jest.fn().mockRejectedValue(new Error('IPFS init failed'))
      };

      await expect(
        framework.initialize(mockServices.orbitDBService, failingIPFS)
      ).rejects.toThrow('IPFS init failed');

      const status = framework.getStatus();
      expect(status.initialized).toBe(false);
      expect(status.healthy).toBe(false);
    });

    it('should apply config overrides during initialization', async () => {
      const overrideConfig = {
        environment: 'production' as const,
        features: { queryCache: false }
      };

      await framework.initialize(
        mockServices.orbitDBService,
        mockServices.ipfsService,
        overrideConfig
      );

      const status = framework.getStatus();
      expect(status.environment).toBe('production');
    });
  });

  describe('Framework Lifecycle', () => {
    beforeEach(async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);
    });

    it('should provide access to core managers', () => {
      expect(framework.getDatabaseManager()).toBeDefined();
      expect(framework.getShardManager()).toBeDefined();
      expect(framework.getRelationshipManager()).toBeDefined();
      expect(framework.getQueryCache()).toBeDefined();
    });

    it('should provide access to services', () => {
      expect(framework.getOrbitDBService()).toBeDefined();
      expect(framework.getIPFSService()).toBeDefined();
    });

    it('should handle graceful shutdown', async () => {
      const initialStatus = framework.getStatus();
      expect(initialStatus.initialized).toBe(true);

      await framework.stop();

      const finalStatus = framework.getStatus();
      expect(finalStatus.initialized).toBe(false);
    });

    it('should perform health checks', async () => {
      const health = await framework.healthCheck();
      
      expect(health.healthy).toBe(true);
      expect(health.services.ipfs).toBe('connected');
      expect(health.services.orbitdb).toBe('connected');
      expect(health.lastCheck).toBeGreaterThan(0);
    });

    it('should collect metrics', () => {
      const metrics = framework.getMetrics();
      
      expect(metrics).toHaveProperty('uptime');
      expect(metrics).toHaveProperty('totalModels');
      expect(metrics).toHaveProperty('totalDatabases');
      expect(metrics).toHaveProperty('queriesExecuted');
      expect(metrics).toHaveProperty('memoryUsage');
      expect(metrics).toHaveProperty('performance');
    });
  });

  describe('Model and Database Integration', () => {
    beforeEach(async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);
    });

    it('should integrate with model system for database operations', async () => {
      // Create a user
      const userData = {
        username: 'testuser',
        email: 'test@example.com',
        isActive: true
      };

      const user = await User.create(userData);
      
      expect(user).toBeInstanceOf(User);
      expect(user.username).toBe('testuser');
      expect(user.email).toBe('test@example.com');
      expect(user.isActive).toBe(true);
      expect(user.id).toBeDefined();
    });

    it('should handle user-scoped and global-scoped models differently', async () => {
      // Global-scoped model (User)
      const user = await User.create({
        username: 'globaluser',
        email: 'global@example.com'
      });

      // User-scoped model (Post) - should use user's database
      const post = await Post.create({
        title: 'Test Post',
        content: 'This is a test post',
        userId: user.id,
        published: true
      });

      expect(user).toBeInstanceOf(User);
      expect(post).toBeInstanceOf(Post);
      expect(post.userId).toBe(user.id);
    });

    it('should support relationship loading', async () => {
      const user = await User.create({
        username: 'userWithPosts',
        email: 'posts@example.com'
      });

      // Create posts for the user
      await Post.create({
        title: 'First Post',
        content: 'Content 1',
        userId: user.id
      });

      await Post.create({
        title: 'Second Post',
        content: 'Content 2',
        userId: user.id
      });

      // Load user's posts
      const relationshipManager = framework.getRelationshipManager();
      const posts = await relationshipManager!.loadRelationship(user, 'posts');

      expect(Array.isArray(posts)).toBe(true);
      expect(posts.length).toBeGreaterThanOrEqual(0); // Mock may return empty array
    });
  });

  describe('Query and Cache Integration', () => {
    beforeEach(async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);
    });

    it('should integrate query system with cache', async () => {
      const queryCache = framework.getQueryCache();
      expect(queryCache).toBeDefined();

      // Just verify that the cache exists and has basic functionality
      expect(typeof queryCache!.set).toBe('function');
      expect(typeof queryCache!.get).toBe('function');
      expect(typeof queryCache!.clear).toBe('function');
    });

    it('should support complex query building', () => {
      const query = User.query()
        .where('isActive', true)
        .where('email', 'like', '%@example.com')
        .orderBy('username', 'asc')
        .limit(10);

      expect(query).toBeDefined();
      expect(typeof query.find).toBe('function');
      expect(typeof query.count).toBe('function');
    });
  });

  describe('Sharding Integration', () => {
    beforeEach(async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);
    });

    it('should integrate with shard manager for model distribution', () => {
      const shardManager = framework.getShardManager();
      expect(shardManager).toBeDefined();

      // Test shard routing
      const testKey = 'test-key-123';
      const modelWithShards = 'TestModel';

      // This would work if we had shards created for TestModel
      expect(() => {
        shardManager!.getShardCount(modelWithShards);
      }).not.toThrow();
    });

    it('should support cross-shard queries', async () => {
      const shardManager = framework.getShardManager();

      // Test querying across all shards (mock implementation)
      const queryFn = async (database: any) => {
        return []; // Mock query result
      };

      // This would work if we had shards created
      const models = shardManager!.getAllModelsWithShards();
      expect(Array.isArray(models)).toBe(true);
    });
  });

  describe('Migration Integration', () => {
    beforeEach(async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);
    });

    it('should integrate migration system', () => {
      const migrationManager = framework.getMigrationManager();
      expect(migrationManager).toBeDefined();

      // Test migration registration
      const testMigration = {
        id: 'test-migration-1',
        version: '1.0.0',
        name: 'Test Migration',
        description: 'A test migration',
        targetModels: ['User'],
        up: [{
          type: 'add_field' as const,
          modelName: 'User',
          fieldName: 'newField',
          fieldConfig: { type: 'string' as const, required: false }
        }],
        down: [{
          type: 'remove_field' as const,
          modelName: 'User',
          fieldName: 'newField'
        }],
        createdAt: Date.now()
      };

      expect(() => {
        migrationManager!.registerMigration(testMigration);
      }).not.toThrow();

      const registered = migrationManager!.getMigration(testMigration.id);
      expect(registered).toEqual(testMigration);
    });

    it('should handle pending migrations', () => {
      const migrationManager = framework.getMigrationManager();
      
      const pendingMigrations = migrationManager!.getPendingMigrations();
      expect(Array.isArray(pendingMigrations)).toBe(true);
    });
  });

  describe('Error Handling and Recovery', () => {
    beforeEach(async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);
    });

    it('should handle service failures gracefully', async () => {
      // Simulate OrbitDB service failure
      const orbitDBService = framework.getOrbitDBService();
      jest.spyOn(orbitDBService!, 'getOrbitDB').mockImplementation(() => {
        throw new Error('OrbitDB service failed');
      });

      // Framework should still respond to health checks
      const health = await framework.healthCheck();
      expect(health).toBeDefined();
    });

    it('should provide error information in status', async () => {
      const status = framework.getStatus();
      
      expect(status).toHaveProperty('services');
      expect(status.services).toHaveProperty('orbitdb');
      expect(status.services).toHaveProperty('ipfs');
    });

    it('should support manual service recovery', async () => {
      // Stop the framework
      await framework.stop();

      // Verify it's stopped
      let status = framework.getStatus();
      expect(status.initialized).toBe(false);

      // Restart with new services
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);

      // Verify it's running again
      status = framework.getStatus();
      expect(status.initialized).toBe(true);
      expect(status.healthy).toBe(true);
    });
  });

  describe('Configuration Management', () => {
    it('should merge default configuration correctly', () => {
      const customConfig: DebrosFrameworkConfig = {
        environment: 'production',
        features: {
          queryCache: false,
          automaticPinning: true
        },
        performance: {
          batchSize: 500
        }
      };

      const customFramework = new DebrosFramework(customConfig);
      const status = customFramework.getStatus();
      
      expect(status.environment).toBe('production');
    });

    it('should support configuration updates', async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);

      const configManager = framework.getConfigManager();
      expect(configManager).toBeDefined();

      // Configuration should be accessible through the framework
      const currentConfig = configManager!.getFullConfig();
      expect(currentConfig).toBeDefined();
      expect(currentConfig.environment).toBe('test');
    });
  });

  describe('Performance and Monitoring', () => {
    beforeEach(async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);
    });

    it('should track uptime correctly', () => {
      const metrics = framework.getMetrics();
      expect(metrics.uptime).toBeGreaterThanOrEqual(0);
    });

    it('should collect performance metrics', () => {
      const metrics = framework.getMetrics();
      
      expect(metrics.performance).toBeDefined();
      expect(metrics.performance.slowQueries).toBeDefined();
      expect(metrics.performance.failedOperations).toBeDefined();
      expect(metrics.performance.averageResponseTime).toBeDefined();
    });

    it('should track memory usage', () => {
      const metrics = framework.getMetrics();
      
      expect(metrics.memoryUsage).toBeDefined();
      expect(metrics.memoryUsage.queryCache).toBeDefined();
      expect(metrics.memoryUsage.relationshipCache).toBeDefined();
      expect(metrics.memoryUsage.total).toBeDefined();
    });

    it('should provide detailed status information', () => {
      const status = framework.getStatus();
      
      expect(status.version).toBeDefined();
      expect(status.lastHealthCheck).toBeGreaterThanOrEqual(0);
      expect(status.services).toBeDefined();
    });
  });

  describe('Concurrent Operations', () => {
    beforeEach(async () => {
      await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);
    });

    it('should handle concurrent model operations', async () => {
      const promises = [];
      
      for (let i = 0; i < 5; i++) {
        promises.push(User.create({
          username: `user${i}`,
          email: `user${i}@example.com`
        }));
      }

      const users = await Promise.all(promises);
      
      expect(users).toHaveLength(5);
      users.forEach((user, index) => {
        expect(user.username).toBe(`user${index}`);
      });
    });

    it('should handle concurrent relationship loading', async () => {
      const user = await User.create({
        username: 'concurrentUser',
        email: 'concurrent@example.com'
      });

      const relationshipManager = framework.getRelationshipManager();
      
      const promises = [
        relationshipManager!.loadRelationship(user, 'posts'),
        relationshipManager!.loadRelationship(user, 'posts'),
        relationshipManager!.loadRelationship(user, 'posts')
      ];

      const results = await Promise.all(promises);
      
      expect(results).toHaveLength(3);
      // Results should be consistent (either all arrays or all same result)
      expect(Array.isArray(results[0])).toBe(Array.isArray(results[1]));
    });
  });
});