import { describe, beforeEach, it, expect, jest } from '@jest/globals';
import { DatabaseManager, UserMappingsData } from '../../../src/framework/core/DatabaseManager';
import { FrameworkOrbitDBService } from '../../../src/framework/services/OrbitDBService';
import { ModelRegistry } from '../../../src/framework/core/ModelRegistry';
import { createMockServices } from '../../mocks/services';
import { BaseModel } from '../../../src/framework/models/BaseModel';
import { Model, Field } from '../../../src/framework/models/decorators';

// Test models for DatabaseManager testing
@Model({
  scope: 'global',
  type: 'docstore'
})
class GlobalTestModel extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;
}

@Model({
  scope: 'user',
  type: 'keyvalue'
})
class UserTestModel extends BaseModel {
  @Field({ type: 'string', required: true })
  name: string;
}

describe('DatabaseManager', () => {
  let databaseManager: DatabaseManager;
  let mockOrbitDBService: FrameworkOrbitDBService;
  let mockDatabase: any;
  let mockOrbitDB: any;

  beforeEach(() => {
    const mockServices = createMockServices();
    mockOrbitDBService = mockServices.orbitDBService;
    
    // Create mock database
    mockDatabase = {
      address: { toString: () => 'mock-address-123' },
      set: jest.fn().mockResolvedValue(undefined),
      get: jest.fn().mockResolvedValue(null),
      put: jest.fn().mockResolvedValue('mock-hash'),
      add: jest.fn().mockResolvedValue('mock-hash'),
      del: jest.fn().mockResolvedValue(undefined),
      query: jest.fn().mockReturnValue([]),
      iterator: jest.fn().mockReturnValue({
        collect: jest.fn().mockReturnValue([])
      }),
      all: jest.fn().mockReturnValue({}),
      value: 0,
      id: 'mock-counter-id',
      inc: jest.fn().mockResolvedValue(undefined)
    };

    mockOrbitDB = {
      open: jest.fn().mockResolvedValue(mockDatabase)
    };

    // Mock OrbitDB service methods
    jest.spyOn(mockOrbitDBService, 'openDatabase').mockResolvedValue(mockDatabase);
    jest.spyOn(mockOrbitDBService, 'getOrbitDB').mockReturnValue(mockOrbitDB);

    // Mock ModelRegistry
    jest.spyOn(ModelRegistry, 'getGlobalModels').mockReturnValue([
      { modelName: 'GlobalTestModel', dbType: 'docstore' }
    ]);
    jest.spyOn(ModelRegistry, 'getUserScopedModels').mockReturnValue([
      { modelName: 'UserTestModel', dbType: 'keyvalue' }
    ]);

    databaseManager = new DatabaseManager(mockOrbitDBService);
    jest.clearAllMocks();
  });

  describe('Initialization', () => {
    it('should initialize all databases correctly', async () => {
      await databaseManager.initializeAllDatabases();

      // Should create global databases
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith(
        'global-globaltestmodel',
        'docstore'
      );

      // Should create system directory shards
      for (let i = 0; i < 4; i++) {
        expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith(
          `global-user-directory-shard-${i}`,
          'keyvalue'
        );
      }
    });

    it('should not initialize databases twice', async () => {
      await databaseManager.initializeAllDatabases();
      const firstCallCount = (mockOrbitDBService.openDatabase as jest.Mock).mock.calls.length;

      await databaseManager.initializeAllDatabases();
      const secondCallCount = (mockOrbitDBService.openDatabase as jest.Mock).mock.calls.length;

      expect(secondCallCount).toBe(firstCallCount);
    });

    it('should handle database creation errors', async () => {
      jest.spyOn(mockOrbitDBService, 'openDatabase').mockRejectedValueOnce(new Error('Creation failed'));

      await expect(databaseManager.initializeAllDatabases()).rejects.toThrow('Creation failed');
    });
  });

  describe('User Database Management', () => {
    beforeEach(async () => {
      // Initialize global databases first
      await databaseManager.initializeAllDatabases();
    });

    it('should create user databases correctly', async () => {
      const userId = 'test-user-123';

      const userMappings = await databaseManager.createUserDatabases(userId);

      expect(userMappings).toBeInstanceOf(UserMappingsData);
      expect(userMappings.userId).toBe(userId);
      expect(userMappings.databases).toHaveProperty('usertestmodelDB');

      // Should create mappings database
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith(
        `${userId}-mappings`,
        'keyvalue'
      );

      // Should create user model database
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith(
        `${userId}-usertestmodel`,
        'keyvalue'
      );

      // Should store mappings in database
      expect(mockDatabase.set).toHaveBeenCalledWith('mappings', expect.any(Object));
    });

    it('should retrieve user mappings from cache', async () => {
      const userId = 'test-user-456';

      // Create user databases first
      const originalMappings = await databaseManager.createUserDatabases(userId);
      jest.clearAllMocks();

      // Get mappings again - should come from cache
      const cachedMappings = await databaseManager.getUserMappings(userId);

      expect(cachedMappings).toBe(originalMappings);
      expect(mockDatabase.get).not.toHaveBeenCalled();
    });

    it('should retrieve user mappings from global directory', async () => {
      const userId = 'test-user-789';
      const mappingsAddress = 'mock-mappings-address';
      const mappingsData = { usertestmodelDB: 'mock-db-address' };

      // Mock directory shard return
      mockDatabase.get
        .mockResolvedValueOnce(mappingsAddress) // From directory shard
        .mockResolvedValueOnce(mappingsData);   // From mappings DB

      const userMappings = await databaseManager.getUserMappings(userId);

      expect(userMappings).toBeInstanceOf(UserMappingsData);
      expect(userMappings.userId).toBe(userId);
      expect(userMappings.databases).toEqual(mappingsData);

      // Should open mappings database
      expect(mockOrbitDB.open).toHaveBeenCalledWith(mappingsAddress);
    });

    it('should handle user not found in directory', async () => {
      const userId = 'nonexistent-user';

      // Mock directory shard returning null
      mockDatabase.get.mockResolvedValue(null);

      await expect(databaseManager.getUserMappings(userId)).rejects.toThrow(
        `User ${userId} not found in directory`
      );
    });

    it('should get user database correctly', async () => {
      const userId = 'test-user-db';
      const modelName = 'UserTestModel';

      // Create user databases first
      await databaseManager.createUserDatabases(userId);

      const userDB = await databaseManager.getUserDatabase(userId, modelName);

      expect(userDB).toBe(mockDatabase);
    });

    it('should handle missing user database', async () => {
      const userId = 'test-user-missing';
      const modelName = 'NonExistentModel';

      // Create user databases first
      await databaseManager.createUserDatabases(userId);

      await expect(databaseManager.getUserDatabase(userId, modelName)).rejects.toThrow(
        `Database not found for user ${userId} and model ${modelName}`
      );
    });
  });

  describe('Global Database Management', () => {
    beforeEach(async () => {
      await databaseManager.initializeAllDatabases();
    });

    it('should get global database correctly', async () => {
      const globalDB = await databaseManager.getGlobalDatabase('GlobalTestModel');

      expect(globalDB).toBe(mockDatabase);
    });

    it('should handle missing global database', async () => {
      await expect(databaseManager.getGlobalDatabase('NonExistentModel')).rejects.toThrow(
        'Global database not found for model: NonExistentModel'
      );
    });

    it('should get global directory shards', async () => {
      const shards = await databaseManager.getGlobalDirectoryShards();

      expect(shards).toHaveLength(4);
      expect(shards.every(shard => shard === mockDatabase)).toBe(true);
    });
  });

  describe('Database Operations', () => {
    beforeEach(async () => {
      await databaseManager.initializeAllDatabases();
    });

    describe('getAllDocuments', () => {
      it('should get all documents from eventlog', async () => {
        const mockDocs = [{ id: '1', data: 'test' }];
        mockDatabase.iterator.mockReturnValue({
          collect: jest.fn().mockReturnValue(mockDocs)
        });

        const docs = await databaseManager.getAllDocuments(mockDatabase, 'eventlog');

        expect(docs).toEqual(mockDocs);
        expect(mockDatabase.iterator).toHaveBeenCalled();
      });

      it('should get all documents from keyvalue', async () => {
        const mockData = { key1: { id: '1' }, key2: { id: '2' } };
        mockDatabase.all.mockReturnValue(mockData);

        const docs = await databaseManager.getAllDocuments(mockDatabase, 'keyvalue');

        expect(docs).toEqual([{ id: '1' }, { id: '2' }]);
        expect(mockDatabase.all).toHaveBeenCalled();
      });

      it('should get all documents from docstore', async () => {
        const mockDocs = [{ id: '1' }, { id: '2' }];
        mockDatabase.query.mockReturnValue(mockDocs);

        const docs = await databaseManager.getAllDocuments(mockDatabase, 'docstore');

        expect(docs).toEqual(mockDocs);
        expect(mockDatabase.query).toHaveBeenCalledWith(expect.any(Function));
      });

      it('should get documents from counter', async () => {
        mockDatabase.value = 42;
        mockDatabase.id = 'counter-123';

        const docs = await databaseManager.getAllDocuments(mockDatabase, 'counter');

        expect(docs).toEqual([{ value: 42, id: 'counter-123' }]);
      });

      it('should handle unsupported database type', async () => {
        await expect(
          databaseManager.getAllDocuments(mockDatabase, 'unsupported' as any)
        ).rejects.toThrow('Unsupported database type: unsupported');
      });
    });

    describe('addDocument', () => {
      it('should add document to eventlog', async () => {
        const data = { content: 'test' };
        mockDatabase.add.mockResolvedValue('hash123');

        const result = await databaseManager.addDocument(mockDatabase, 'eventlog', data);

        expect(result).toBe('hash123');
        expect(mockDatabase.add).toHaveBeenCalledWith(data);
      });

      it('should add document to keyvalue', async () => {
        const data = { id: 'key1', content: 'test' };

        const result = await databaseManager.addDocument(mockDatabase, 'keyvalue', data);

        expect(result).toBe('key1');
        expect(mockDatabase.set).toHaveBeenCalledWith('key1', data);
      });

      it('should add document to docstore', async () => {
        const data = { id: 'doc1', content: 'test' };
        mockDatabase.put.mockResolvedValue('hash123');

        const result = await databaseManager.addDocument(mockDatabase, 'docstore', data);

        expect(result).toBe('hash123');
        expect(mockDatabase.put).toHaveBeenCalledWith(data);
      });

      it('should increment counter', async () => {
        const data = { amount: 5 };
        mockDatabase.id = 'counter-123';

        const result = await databaseManager.addDocument(mockDatabase, 'counter', data);

        expect(result).toBe('counter-123');
        expect(mockDatabase.inc).toHaveBeenCalledWith(5);
      });
    });

    describe('updateDocument', () => {
      it('should update document in keyvalue', async () => {
        const data = { id: 'key1', content: 'updated' };

        await databaseManager.updateDocument(mockDatabase, 'keyvalue', 'key1', data);

        expect(mockDatabase.set).toHaveBeenCalledWith('key1', data);
      });

      it('should update document in docstore', async () => {
        const data = { id: 'doc1', content: 'updated' };

        await databaseManager.updateDocument(mockDatabase, 'docstore', 'doc1', data);

        expect(mockDatabase.put).toHaveBeenCalledWith(data);
      });

      it('should add new entry for append-only stores', async () => {
        const data = { id: 'event1', content: 'updated' };
        mockDatabase.add.mockResolvedValue('hash123');

        await databaseManager.updateDocument(mockDatabase, 'eventlog', 'event1', data);

        expect(mockDatabase.add).toHaveBeenCalledWith(data);
      });
    });

    describe('deleteDocument', () => {
      it('should delete document from keyvalue', async () => {
        await databaseManager.deleteDocument(mockDatabase, 'keyvalue', 'key1');

        expect(mockDatabase.del).toHaveBeenCalledWith('key1');
      });

      it('should delete document from docstore', async () => {
        await databaseManager.deleteDocument(mockDatabase, 'docstore', 'doc1');

        expect(mockDatabase.del).toHaveBeenCalledWith('doc1');
      });

      it('should add deletion marker for append-only stores', async () => {
        mockDatabase.add.mockResolvedValue('hash123');

        await databaseManager.deleteDocument(mockDatabase, 'eventlog', 'event1');

        expect(mockDatabase.add).toHaveBeenCalledWith({
          _deleted: true,
          id: 'event1',
          deletedAt: expect.any(Number)
        });
      });
    });
  });

  describe('Shard Index Calculation', () => {
    it('should calculate consistent shard indices', async () => {
      await databaseManager.initializeAllDatabases();

      const userId1 = 'user-123';
      const userId2 = 'user-456';

      // Create users and verify they're stored in shards
      await databaseManager.createUserDatabases(userId1);
      await databaseManager.createUserDatabases(userId2);

      // The shard index should be consistent for the same user
      const calls = (mockDatabase.set as jest.Mock).mock.calls;
      const user1Calls = calls.filter(call => call[0] === userId1);
      const user2Calls = calls.filter(call => call[0] === userId2);

      expect(user1Calls).toHaveLength(1);
      expect(user2Calls).toHaveLength(1);
    });
  });

  describe('Error Handling', () => {
    it('should handle database operation errors', async () => {
      await databaseManager.initializeAllDatabases();

      mockDatabase.put.mockRejectedValue(new Error('Database error'));

      await expect(
        databaseManager.addDocument(mockDatabase, 'docstore', { id: 'test' })
      ).rejects.toThrow('Database error');
    });

    it('should handle missing global directory', async () => {
      // Don't initialize databases
      const userId = 'test-user';

      await expect(databaseManager.getUserMappings(userId)).rejects.toThrow(
        'Global directory not initialized'
      );
    });
  });

  describe('Cleanup', () => {
    it('should stop and clear all resources', async () => {
      await databaseManager.initializeAllDatabases();
      await databaseManager.createUserDatabases('test-user');

      await databaseManager.stop();

      // After stopping, initialization should be required again
      await expect(databaseManager.getGlobalDatabase('GlobalTestModel')).rejects.toThrow();
    });
  });
});