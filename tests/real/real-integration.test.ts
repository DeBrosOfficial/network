import { describe, beforeAll, afterAll, beforeEach, it, expect, jest } from '@jest/globals';
import { DebrosFramework } from '../../src/framework/DebrosFramework';
import { BaseModel } from '../../src/framework/models/BaseModel';
import { Model, Field, BeforeCreate } from '../../src/framework/models/decorators';
import { realTestHelpers, RealTestNetwork } from './setup/test-lifecycle';

// Test model for real integration testing
@Model({
  scope: 'global',
  type: 'docstore'
})
class RealTestUser extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  declare username: string;

  @Field({ type: 'string', required: true })
  declare email: string;

  @Field({ type: 'boolean', required: false, default: true })
  declare isActive: boolean;

  @Field({ type: 'number', required: false })
  declare createdAt: number;

  @BeforeCreate()
  setCreatedAt() {
    this.createdAt = Date.now();
  }
}

@Model({
  scope: 'user',
  type: 'docstore'
})
class RealTestPost extends BaseModel {
  @Field({ type: 'string', required: true })
  declare title: string;

  @Field({ type: 'string', required: true })
  declare content: string;

  @Field({ type: 'string', required: true })
  declare authorId: string;

  @Field({ type: 'number', required: false })
  declare createdAt: number;

  @BeforeCreate()
  setCreatedAt() {
    this.createdAt = Date.now();
  }
}

describe('Real IPFS/OrbitDB Integration Tests', () => {
  let network: RealTestNetwork;
  let framework: DebrosFramework;

  beforeAll(async () => {
    console.log('🚀 Setting up real integration test environment...');
    
    // Setup the real network with multiple nodes
    network = await realTestHelpers.setupAll({
      nodeCount: 2, // Use 2 nodes for faster tests
      timeout: 60000,
      enableDebugLogs: true
    });

    // Create framework instance with real services
    framework = new DebrosFramework();
    
    const primaryNode = realTestHelpers.getManager().getPrimaryNode();
    await framework.initialize(primaryNode.orbitdb, primaryNode.ipfs);

    console.log('✅ Real integration test environment ready');
  }, 90000); // 90 second timeout for setup

  afterAll(async () => {
    console.log('🧹 Cleaning up real integration test environment...');
    
    try {
      if (framework) {
        await framework.stop();
      }
    } catch (error) {
      console.warn('Warning: Error stopping framework:', error);
    }

    await realTestHelpers.cleanupAll();
    console.log('✅ Real integration test cleanup complete');
  }, 30000); // 30 second timeout for cleanup

  beforeEach(async () => {
    // Wait for network to stabilize between tests
    await realTestHelpers.getManager().waitForNetworkStabilization(1000);
  });

  describe('Framework Initialization', () => {
    it('should initialize framework with real IPFS and OrbitDB services', async () => {
      expect(framework).toBeDefined();
      expect(framework.getStatus().initialized).toBe(true);
      
      const health = await framework.healthCheck();
      expect(health.healthy).toBe(true);
      expect(health.services.ipfs).toBe('connected');
      expect(health.services.orbitdb).toBe('connected');
    });

    it('should have working database manager', async () => {
      const databaseManager = framework.getDatabaseManager();
      expect(databaseManager).toBeDefined();
      
      // Test database creation
      const testDb = await databaseManager.getGlobalDatabase('test-db');
      expect(testDb).toBeDefined();
    });

    it('should verify network connectivity', async () => {
      const isConnected = await realTestHelpers.getManager().verifyNetworkConnectivity();
      expect(isConnected).toBe(true);
    });
  });

  describe('Real Model Operations', () => {
    it('should create and save models to real IPFS/OrbitDB', async () => {
      const user = await RealTestUser.create({
        username: 'real-test-user',
        email: 'real@test.com'
      });

      expect(user).toBeInstanceOf(RealTestUser);
      expect(user.id).toBeDefined();
      expect(user.username).toBe('real-test-user');
      expect(user.email).toBe('real@test.com');
      expect(user.isActive).toBe(true);
      expect(user.createdAt).toBeGreaterThan(0);
    });

    it('should find models from real storage', async () => {
      // Create a user
      const originalUser = await RealTestUser.create({
        username: 'findable-user',
        email: 'findable@test.com'
      });

      // Wait for data to be persisted
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Find the user
      const foundUser = await RealTestUser.findById(originalUser.id);
      expect(foundUser).toBeInstanceOf(RealTestUser);
      expect(foundUser?.id).toBe(originalUser.id);
      expect(foundUser?.username).toBe('findable-user');
    });

    it('should handle unique constraints with real storage', async () => {
      // Create first user
      await RealTestUser.create({
        username: 'unique-user',
        email: 'unique1@test.com'
      });

      // Wait for persistence
      await new Promise(resolve => setTimeout(resolve, 500));

      // Try to create duplicate
      await expect(RealTestUser.create({
        username: 'unique-user', // Duplicate username
        email: 'unique2@test.com'
      })).rejects.toThrow();
    });

    it('should work with user-scoped models', async () => {
      const post = await RealTestPost.create({
        title: 'Real Test Post',
        content: 'This post is stored in real IPFS/OrbitDB',
        authorId: 'test-author-123'
      });

      expect(post).toBeInstanceOf(RealTestPost);
      expect(post.title).toBe('Real Test Post');
      expect(post.authorId).toBe('test-author-123');
      expect(post.createdAt).toBeGreaterThan(0);
    });
  });

  describe('Real Data Persistence', () => {
    it('should persist data across framework restarts', async () => {
      // Create data
      const user = await RealTestUser.create({
        username: 'persistent-user',
        email: 'persistent@test.com'
      });

      const userId = user.id;

      // Wait for persistence
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Stop and restart framework (but keep the same IPFS/OrbitDB instances)
      await framework.stop();
      
      const primaryNode = realTestHelpers.getManager().getPrimaryNode();
      await framework.initialize(primaryNode.orbitdb, primaryNode.ipfs);

      // Try to find the user
      const foundUser = await RealTestUser.findById(userId);
      expect(foundUser).toBeInstanceOf(RealTestUser);
      expect(foundUser?.username).toBe('persistent-user');
    });

    it('should handle concurrent operations', async () => {
      // Create multiple users concurrently
      const userCreations = Array.from({ length: 5 }, (_, i) =>
        RealTestUser.create({
          username: `concurrent-user-${i}`,
          email: `concurrent${i}@test.com`
        })
      );

      const users = await Promise.all(userCreations);

      expect(users).toHaveLength(5);
      users.forEach((user, i) => {
        expect(user.username).toBe(`concurrent-user-${i}`);
      });

      // Verify all users can be found
      const foundUsers = await Promise.all(
        users.map(user => RealTestUser.findById(user.id))
      );

      foundUsers.forEach(user => {
        expect(user).toBeInstanceOf(RealTestUser);
      });
    });
  });

  describe('Real Network Operations', () => {
    it('should use real IPFS for content addressing', async () => {
      const ipfsService = realTestHelpers.getManager().getPrimaryNode().ipfs;
      const helia = ipfsService.getHelia();
      
      expect(helia).toBeDefined();
      
      // Test basic IPFS operations
      const testData = new TextEncoder().encode('Hello, real IPFS!');
      const { cid } = await helia.blockstore.put(testData);
      
      expect(cid).toBeDefined();
      
      const retrievedData = await helia.blockstore.get(cid);
      expect(new TextDecoder().decode(retrievedData)).toBe('Hello, real IPFS!');
    });

    it('should use real OrbitDB for distributed databases', async () => {
      const orbitdbService = realTestHelpers.getManager().getPrimaryNode().orbitdb;
      const orbitdb = orbitdbService.getOrbitDB();
      
      expect(orbitdb).toBeDefined();
      expect(orbitdb.id).toBeDefined();
      
      // Test basic OrbitDB operations
      const testDb = await orbitdbService.openDB('real-test-db', 'documents');
      expect(testDb).toBeDefined();
      
      const docId = await testDb.put({ message: 'Hello, real OrbitDB!' });
      expect(docId).toBeDefined();
      
      const doc = await testDb.get(docId);
      expect(doc.message).toBe('Hello, real OrbitDB!');
    });

    it('should verify peer connections exist', async () => {
      const nodes = realTestHelpers.getManager().getMultipleNodes();
      
      // Each node should have connections to other nodes
      for (const node of nodes) {
        const peers = node.ipfs.getConnectedPeers();
        expect(peers.length).toBeGreaterThan(0);
      }
    });
  });
}, 120000); // 2 minute timeout for the entire suite