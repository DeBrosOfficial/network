import { describe, beforeEach, it, expect, jest } from '@jest/globals';
import { ShardManager, ShardInfo } from '../../../src/framework/sharding/ShardManager';
import { FrameworkOrbitDBService } from '../../../src/framework/services/OrbitDBService';
import { ShardingConfig } from '../../../src/framework/types/framework';
import { createMockServices } from '../../mocks/services';

describe('ShardManager', () => {
  let shardManager: ShardManager;
  let mockOrbitDBService: FrameworkOrbitDBService;
  let mockDatabase: any;

  beforeEach(() => {
    const mockServices = createMockServices();
    mockOrbitDBService = mockServices.orbitDBService;
    
    // Create mock database
    mockDatabase = {
      address: { toString: () => 'mock-address-123' },
      set: jest.fn().mockResolvedValue(undefined),
      get: jest.fn().mockResolvedValue(null),
      del: jest.fn().mockResolvedValue(undefined),
      put: jest.fn().mockResolvedValue('mock-hash'),
      add: jest.fn().mockResolvedValue('mock-hash'),
      query: jest.fn().mockReturnValue([])
    };

    // Mock OrbitDB service methods
    jest.spyOn(mockOrbitDBService, 'openDatabase').mockResolvedValue(mockDatabase);

    shardManager = new ShardManager();
    shardManager.setOrbitDBService(mockOrbitDBService);
    
    jest.clearAllMocks();
  });

  describe('Initialization', () => {
    it('should set OrbitDB service correctly', () => {
      const newShardManager = new ShardManager();
      newShardManager.setOrbitDBService(mockOrbitDBService);

      // No direct way to test this, but we can verify it works in other tests
      expect(newShardManager).toBeInstanceOf(ShardManager);
    });

    it('should throw error when OrbitDB service not set', async () => {
      const newShardManager = new ShardManager();
      const config: ShardingConfig = { strategy: 'hash', count: 2, key: 'id' };

      await expect(newShardManager.createShards('TestModel', config)).rejects.toThrow(
        'OrbitDB service not initialized'
      );
    });
  });

  describe('Shard Creation', () => {
    it('should create shards with hash strategy', async () => {
      const config: ShardingConfig = { strategy: 'hash', count: 3, key: 'id' };

      await shardManager.createShards('TestModel', config, 'docstore');

      // Should create 3 shards
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledTimes(3);
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith('testmodel-shard-0', 'docstore');
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith('testmodel-shard-1', 'docstore');
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith('testmodel-shard-2', 'docstore');

      const shards = shardManager.getAllShards('TestModel');
      expect(shards).toHaveLength(3);
      expect(shards[0]).toMatchObject({
        name: 'testmodel-shard-0',
        index: 0,
        address: 'mock-address-123'
      });
    });

    it('should create shards with range strategy', async () => {
      const config: ShardingConfig = { strategy: 'range', count: 2, key: 'name' };

      await shardManager.createShards('RangeModel', config, 'keyvalue');

      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledTimes(2);
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith('rangemodel-shard-0', 'keyvalue');
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith('rangemodel-shard-1', 'keyvalue');
    });

    it('should create shards with user strategy', async () => {
      const config: ShardingConfig = { strategy: 'user', count: 4, key: 'userId' };

      await shardManager.createShards('UserModel', config);

      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledTimes(4);
      
      const shards = shardManager.getAllShards('UserModel');
      expect(shards).toHaveLength(4);
    });

    it('should handle shard creation errors', async () => {
      const config: ShardingConfig = { strategy: 'hash', count: 2, key: 'id' };
      
      jest.spyOn(mockOrbitDBService, 'openDatabase').mockRejectedValueOnce(new Error('Database creation failed'));

      await expect(shardManager.createShards('FailModel', config)).rejects.toThrow('Database creation failed');
    });
  });

  describe('Shard Routing', () => {
    beforeEach(async () => {
      const config: ShardingConfig = { strategy: 'hash', count: 4, key: 'id' };
      await shardManager.createShards('TestModel', config);
    });

    it('should route keys to consistent shards with hash strategy', () => {
      const key1 = 'user-123';
      const key2 = 'user-456';
      const key3 = 'user-123'; // Same as key1

      const shard1 = shardManager.getShardForKey('TestModel', key1);
      const shard2 = shardManager.getShardForKey('TestModel', key2);
      const shard3 = shardManager.getShardForKey('TestModel', key3);

      // Same keys should route to same shards
      expect(shard1.index).toBe(shard3.index);
      
      // Different keys may route to different shards
      expect(shard1.index).toBeGreaterThanOrEqual(0);
      expect(shard1.index).toBeLessThan(4);
      expect(shard2.index).toBeGreaterThanOrEqual(0);
      expect(shard2.index).toBeLessThan(4);
    });

    it('should route keys with range strategy', async () => {
      const config: ShardingConfig = { strategy: 'range', count: 3, key: 'name' };
      await shardManager.createShards('RangeModel', config);

      const shardA = shardManager.getShardForKey('RangeModel', 'apple');
      const shardM = shardManager.getShardForKey('RangeModel', 'middle');
      const shardZ = shardManager.getShardForKey('RangeModel', 'zebra');

      // Keys starting with different letters should potentially route to different shards
      expect(shardA.index).toBeGreaterThanOrEqual(0);
      expect(shardA.index).toBeLessThan(3);
      expect(shardM.index).toBeGreaterThanOrEqual(0);
      expect(shardM.index).toBeLessThan(3);
      expect(shardZ.index).toBeGreaterThanOrEqual(0);
      expect(shardZ.index).toBeLessThan(3);
    });

    it('should handle user strategy routing', async () => {
      const config: ShardingConfig = { strategy: 'user', count: 2, key: 'userId' };
      await shardManager.createShards('UserModel', config);

      const shard1 = shardManager.getShardForKey('UserModel', 'user-abc');
      const shard2 = shardManager.getShardForKey('UserModel', 'user-def');
      const shard3 = shardManager.getShardForKey('UserModel', 'user-abc'); // Same as shard1

      expect(shard1.index).toBe(shard3.index);
      expect(shard1.index).toBeGreaterThanOrEqual(0);
      expect(shard1.index).toBeLessThan(2);
    });

    it('should throw error for unsupported sharding strategy', async () => {
      const config: ShardingConfig = { strategy: 'unsupported' as any, count: 2, key: 'id' };
      await shardManager.createShards('UnsupportedModel', config);

      expect(() => {
        shardManager.getShardForKey('UnsupportedModel', 'test-key');
      }).toThrow('Unsupported sharding strategy: unsupported');
    });

    it('should throw error when no shards exist for model', () => {
      expect(() => {
        shardManager.getShardForKey('NonExistentModel', 'test-key');
      }).toThrow('No shards found for model NonExistentModel');
    });

    it('should throw error when no shard configuration exists', async () => {
      // Manually clear the config to simulate this error
      const config: ShardingConfig = { strategy: 'hash', count: 2, key: 'id' };
      await shardManager.createShards('ConfigTestModel', config);
      
      // Access private property for testing (not ideal but necessary for this test)
      (shardManager as any).shardConfigs.delete('ConfigTestModel');

      expect(() => {
        shardManager.getShardForKey('ConfigTestModel', 'test-key');
      }).toThrow('No shard configuration found for model ConfigTestModel');
    });
  });

  describe('Shard Management', () => {
    beforeEach(async () => {
      const config: ShardingConfig = { strategy: 'hash', count: 3, key: 'id' };
      await shardManager.createShards('TestModel', config);
    });

    it('should get all shards for a model', () => {
      const shards = shardManager.getAllShards('TestModel');
      
      expect(shards).toHaveLength(3);
      expect(shards[0].name).toBe('testmodel-shard-0');
      expect(shards[1].name).toBe('testmodel-shard-1');
      expect(shards[2].name).toBe('testmodel-shard-2');
    });

    it('should return empty array for non-existent model', () => {
      const shards = shardManager.getAllShards('NonExistentModel');
      expect(shards).toEqual([]);
    });

    it('should get shard by index', () => {
      const shard0 = shardManager.getShardByIndex('TestModel', 0);
      const shard1 = shardManager.getShardByIndex('TestModel', 1);
      const shard2 = shardManager.getShardByIndex('TestModel', 2);
      const shardInvalid = shardManager.getShardByIndex('TestModel', 5);

      expect(shard0?.index).toBe(0);
      expect(shard1?.index).toBe(1);
      expect(shard2?.index).toBe(2);
      expect(shardInvalid).toBeUndefined();
    });

    it('should get shard count', () => {
      const count = shardManager.getShardCount('TestModel');
      expect(count).toBe(3);

      const nonExistentCount = shardManager.getShardCount('NonExistentModel');
      expect(nonExistentCount).toBe(0);
    });

    it('should get all models with shards', () => {
      const models = shardManager.getAllModelsWithShards();
      expect(models).toContain('TestModel');
    });
  });

  describe('Global Index Management', () => {
    it('should create global index with shards', async () => {
      await shardManager.createGlobalIndex('TestModel', 'username-index');

      // Should create 4 index shards (default)
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledTimes(4);
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith('username-index-shard-0', 'keyvalue');
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith('username-index-shard-1', 'keyvalue');
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith('username-index-shard-2', 'keyvalue');
      expect(mockOrbitDBService.openDatabase).toHaveBeenCalledWith('username-index-shard-3', 'keyvalue');

      const indexShards = shardManager.getAllShards('username-index');
      expect(indexShards).toHaveLength(4);
    });

    it('should add to global index', async () => {
      await shardManager.createGlobalIndex('TestModel', 'email-index');

      await shardManager.addToGlobalIndex('email-index', 'user@example.com', 'user-123');

      // Should call set on one of the index shards
      expect(mockDatabase.set).toHaveBeenCalledWith('user@example.com', 'user-123');
    });

    it('should get from global index', async () => {
      await shardManager.createGlobalIndex('TestModel', 'id-index');
      
      mockDatabase.get.mockResolvedValue('user-456');

      const result = await shardManager.getFromGlobalIndex('id-index', 'lookup-key');

      expect(result).toBe('user-456');
      expect(mockDatabase.get).toHaveBeenCalledWith('lookup-key');
    });

    it('should remove from global index', async () => {
      await shardManager.createGlobalIndex('TestModel', 'remove-index');

      await shardManager.removeFromGlobalIndex('remove-index', 'key-to-remove');

      expect(mockDatabase.del).toHaveBeenCalledWith('key-to-remove');
    });

    it('should handle missing global index', async () => {
      await expect(
        shardManager.addToGlobalIndex('non-existent-index', 'key', 'value')
      ).rejects.toThrow('Global index non-existent-index not found');

      await expect(
        shardManager.getFromGlobalIndex('non-existent-index', 'key')
      ).rejects.toThrow('Global index non-existent-index not found');

      await expect(
        shardManager.removeFromGlobalIndex('non-existent-index', 'key')
      ).rejects.toThrow('Global index non-existent-index not found');
    });

    it('should handle global index operation errors', async () => {
      await shardManager.createGlobalIndex('TestModel', 'error-index');

      mockDatabase.set.mockRejectedValue(new Error('Database error'));
      mockDatabase.get.mockRejectedValue(new Error('Database error'));
      mockDatabase.del.mockRejectedValue(new Error('Database error'));

      await expect(
        shardManager.addToGlobalIndex('error-index', 'key', 'value')
      ).rejects.toThrow('Database error');

      const result = await shardManager.getFromGlobalIndex('error-index', 'key');
      expect(result).toBeNull(); // Should return null on error

      await expect(
        shardManager.removeFromGlobalIndex('error-index', 'key')
      ).rejects.toThrow('Database error');
    });
  });

  describe('Query Operations', () => {
    beforeEach(async () => {
      const config: ShardingConfig = { strategy: 'hash', count: 2, key: 'id' };
      await shardManager.createShards('QueryModel', config);
    });

    it('should query all shards', async () => {
      const mockQueryFn = jest.fn()
        .mockResolvedValueOnce([{ id: '1', name: 'test1' }])
        .mockResolvedValueOnce([{ id: '2', name: 'test2' }]);

      const results = await shardManager.queryAllShards('QueryModel', mockQueryFn);

      expect(mockQueryFn).toHaveBeenCalledTimes(2);
      expect(results).toEqual([
        { id: '1', name: 'test1' },
        { id: '2', name: 'test2' }
      ]);
    });

    it('should handle query errors gracefully', async () => {
      const mockQueryFn = jest.fn()
        .mockResolvedValueOnce([{ id: '1', name: 'test1' }])
        .mockRejectedValueOnce(new Error('Query failed'));

      const results = await shardManager.queryAllShards('QueryModel', mockQueryFn);

      expect(results).toEqual([{ id: '1', name: 'test1' }]);
    });

    it('should throw error when querying non-existent model', async () => {
      const mockQueryFn = jest.fn();

      await expect(
        shardManager.queryAllShards('NonExistentModel', mockQueryFn)
      ).rejects.toThrow('No shards found for model NonExistentModel');
    });
  });

  describe('Statistics and Monitoring', () => {
    beforeEach(async () => {
      const config: ShardingConfig = { strategy: 'hash', count: 3, key: 'id' };
      await shardManager.createShards('StatsModel', config);
    });

    it('should get shard statistics', () => {
      const stats = shardManager.getShardStatistics('StatsModel');

      expect(stats).toEqual({
        modelName: 'StatsModel',
        shardCount: 3,
        shards: [
          { name: 'statsmodel-shard-0', index: 0, address: 'mock-address-123' },
          { name: 'statsmodel-shard-1', index: 1, address: 'mock-address-123' },
          { name: 'statsmodel-shard-2', index: 2, address: 'mock-address-123' }
        ]
      });
    });

    it('should return null for non-existent model statistics', () => {
      const stats = shardManager.getShardStatistics('NonExistentModel');
      expect(stats).toBeNull();
    });

    it('should list all models with shards', async () => {
      const config1: ShardingConfig = { strategy: 'hash', count: 2, key: 'id' };
      const config2: ShardingConfig = { strategy: 'range', count: 3, key: 'name' };
      
      await shardManager.createShards('Model1', config1);
      await shardManager.createShards('Model2', config2);

      const models = shardManager.getAllModelsWithShards();
      
      expect(models).toContain('StatsModel'); // From beforeEach
      expect(models).toContain('Model1');
      expect(models).toContain('Model2');
      expect(models.length).toBeGreaterThanOrEqual(3);
    });
  });

  describe('Hash Function Consistency', () => {
    it('should produce consistent hash results', () => {
      // Test the hash function directly by creating shards and checking consistency
      const testKeys = ['user-123', 'user-456', 'user-789', 'user-abc', 'user-def'];
      const shardCount = 4;

      // Get shard indices for each key multiple times
      const config: ShardingConfig = { strategy: 'hash', count: shardCount, key: 'id' };
      
      return shardManager.createShards('HashTestModel', config).then(() => {
        testKeys.forEach(key => {
          const shard1 = shardManager.getShardForKey('HashTestModel', key);
          const shard2 = shardManager.getShardForKey('HashTestModel', key);
          const shard3 = shardManager.getShardForKey('HashTestModel', key);

          // Same key should always route to same shard
          expect(shard1.index).toBe(shard2.index);
          expect(shard2.index).toBe(shard3.index);
          
          // Shard index should be within valid range
          expect(shard1.index).toBeGreaterThanOrEqual(0);
          expect(shard1.index).toBeLessThan(shardCount);
        });
      });
    });
  });

  describe('Cleanup', () => {
    it('should stop and clear all resources', async () => {
      const config: ShardingConfig = { strategy: 'hash', count: 2, key: 'id' };
      await shardManager.createShards('CleanupModel', config);
      await shardManager.createGlobalIndex('CleanupModel', 'cleanup-index');

      expect(shardManager.getAllShards('CleanupModel')).toHaveLength(2);
      expect(shardManager.getAllShards('cleanup-index')).toHaveLength(4);

      await shardManager.stop();

      expect(shardManager.getAllShards('CleanupModel')).toHaveLength(0);
      expect(shardManager.getAllShards('cleanup-index')).toHaveLength(0);
      expect(shardManager.getAllModelsWithShards()).toHaveLength(0);
    });
  });
});