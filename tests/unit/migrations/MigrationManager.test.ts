import { describe, beforeEach, it, expect, jest } from '@jest/globals';
import {
  MigrationManager,
  Migration,
  MigrationOperation,
  MigrationResult,
  MigrationValidator,
  MigrationLogger
} from '../../../src/framework/migrations/MigrationManager';
import { FieldConfig } from '../../../src/framework/types/models';
import { createMockServices } from '../../mocks/services';

describe('MigrationManager', () => {
  let migrationManager: MigrationManager;
  let mockDatabaseManager: any;
  let mockShardManager: any;
  let mockLogger: MigrationLogger;

  const createTestMigration = (overrides: Partial<Migration> = {}): Migration => ({
    id: 'test-migration-1',
    version: '1.0.0',
    name: 'Test Migration',
    description: 'A test migration for unit testing',
    targetModels: ['TestModel'],
    up: [
      {
        type: 'add_field',
        modelName: 'TestModel',
        fieldName: 'newField',
        fieldConfig: {
          type: 'string',
          required: false,
          default: 'default-value'
        } as FieldConfig
      }
    ],
    down: [
      {
        type: 'remove_field',
        modelName: 'TestModel',
        fieldName: 'newField'
      }
    ],
    createdAt: Date.now(),
    ...overrides
  });

  beforeEach(() => {
    const mockServices = createMockServices();
    
    mockDatabaseManager = {
      getAllDocuments: jest.fn().mockResolvedValue([]),
      addDocument: jest.fn().mockResolvedValue('mock-id'),
      updateDocument: jest.fn().mockResolvedValue(undefined),
      deleteDocument: jest.fn().mockResolvedValue(undefined),
    };

    mockShardManager = {
      getAllShards: jest.fn().mockReturnValue([]),
      getShardForKey: jest.fn().mockReturnValue({ name: 'shard-0', database: {} }),
    };

    mockLogger = {
      info: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
      debug: jest.fn()
    };

    migrationManager = new MigrationManager(mockDatabaseManager, mockShardManager, mockLogger);
    
    jest.clearAllMocks();
  });

  describe('Migration Registration', () => {
    it('should register a valid migration', () => {
      const migration = createTestMigration();

      migrationManager.registerMigration(migration);

      const registered = migrationManager.getMigration(migration.id);
      expect(registered).toEqual(migration);
      expect(mockLogger.info).toHaveBeenCalledWith(
        `Registered migration: ${migration.name} (${migration.version})`,
        expect.objectContaining({
          migrationId: migration.id,
          targetModels: migration.targetModels
        })
      );
    });

    it('should throw error for invalid migration structure', () => {
      const invalidMigration = createTestMigration({
        id: '', // Invalid - empty ID
      });

      expect(() => migrationManager.registerMigration(invalidMigration)).toThrow(
        'Migration must have id, version, and name'
      );
    });

    it('should throw error for migration without target models', () => {
      const invalidMigration = createTestMigration({
        targetModels: [] // Invalid - empty target models
      });

      expect(() => migrationManager.registerMigration(invalidMigration)).toThrow(
        'Migration must specify target models'
      );
    });

    it('should throw error for migration without up operations', () => {
      const invalidMigration = createTestMigration({
        up: [] // Invalid - no up operations
      });

      expect(() => migrationManager.registerMigration(invalidMigration)).toThrow(
        'Migration must have at least one up operation'
      );
    });

    it('should throw error for duplicate version with different ID', () => {
      const migration1 = createTestMigration({ id: 'migration-1', version: '1.0.0' });
      const migration2 = createTestMigration({ id: 'migration-2', version: '1.0.0' });

      migrationManager.registerMigration(migration1);

      expect(() => migrationManager.registerMigration(migration2)).toThrow(
        'Migration version 1.0.0 already exists with different ID'
      );
    });

    it('should allow registering same migration with same ID', () => {
      const migration = createTestMigration();

      migrationManager.registerMigration(migration);
      migrationManager.registerMigration(migration); // Should not throw

      expect(migrationManager.getMigrations()).toHaveLength(1);
    });
  });

  describe('Migration Retrieval', () => {
    beforeEach(() => {
      const migration1 = createTestMigration({ id: 'migration-1', version: '1.0.0' });
      const migration2 = createTestMigration({ id: 'migration-2', version: '2.0.0' });
      const migration3 = createTestMigration({ id: 'migration-3', version: '1.5.0' });

      migrationManager.registerMigration(migration1);
      migrationManager.registerMigration(migration2);
      migrationManager.registerMigration(migration3);
    });

    it('should get all migrations sorted by version', () => {
      const migrations = migrationManager.getMigrations();

      expect(migrations).toHaveLength(3);
      expect(migrations[0].version).toBe('1.0.0');
      expect(migrations[1].version).toBe('1.5.0');
      expect(migrations[2].version).toBe('2.0.0');
    });

    it('should get migration by ID', () => {
      const migration = migrationManager.getMigration('migration-2');

      expect(migration).toBeDefined();
      expect(migration?.version).toBe('2.0.0');
    });

    it('should return null for non-existent migration', () => {
      const migration = migrationManager.getMigration('non-existent');

      expect(migration).toBeNull();
    });

    it('should get pending migrations', () => {
      // Mock applied migrations (empty for this test)
      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([]);

      const pending = migrationManager.getPendingMigrations();

      expect(pending).toHaveLength(3);
    });

    it('should filter pending migrations by model', () => {
      const migration4 = createTestMigration({
        id: 'migration-4',
        version: '3.0.0',
        targetModels: ['OtherModel']
      });
      migrationManager.registerMigration(migration4);

      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([]);

      const pending = migrationManager.getPendingMigrations('TestModel');

      expect(pending).toHaveLength(3); // Should exclude migration-4
      expect(pending.every(m => m.targetModels.includes('TestModel'))).toBe(true);
    });
  });

  describe('Migration Operations', () => {
    it('should validate add_field operation', () => {
      const operation: MigrationOperation = {
        type: 'add_field',
        modelName: 'TestModel',
        fieldName: 'newField',
        fieldConfig: { type: 'string', required: false }
      };

      expect(() => (migrationManager as any).validateOperation(operation)).not.toThrow();
    });

    it('should validate remove_field operation', () => {
      const operation: MigrationOperation = {
        type: 'remove_field',
        modelName: 'TestModel',
        fieldName: 'oldField'
      };

      expect(() => (migrationManager as any).validateOperation(operation)).not.toThrow();
    });

    it('should validate rename_field operation', () => {
      const operation: MigrationOperation = {
        type: 'rename_field',
        modelName: 'TestModel',
        fieldName: 'oldField',
        newFieldName: 'newField'
      };

      expect(() => (migrationManager as any).validateOperation(operation)).not.toThrow();
    });

    it('should validate transform_data operation', () => {
      const operation: MigrationOperation = {
        type: 'transform_data',
        modelName: 'TestModel',
        transformer: (data: any) => data
      };

      expect(() => (migrationManager as any).validateOperation(operation)).not.toThrow();
    });

    it('should reject invalid operation type', () => {
      const operation: MigrationOperation = {
        type: 'invalid_type' as any,
        modelName: 'TestModel'
      };

      expect(() => (migrationManager as any).validateOperation(operation)).toThrow(
        'Invalid operation type: invalid_type'
      );
    });

    it('should reject operation without model name', () => {
      const operation: MigrationOperation = {
        type: 'add_field',
        modelName: ''
      };

      expect(() => (migrationManager as any).validateOperation(operation)).toThrow(
        'Operation must specify modelName'
      );
    });
  });

  describe('Migration Execution', () => {
    let migration: Migration;

    beforeEach(() => {
      migration = createTestMigration();
      migrationManager.registerMigration(migration);

      // Mock helper methods
      jest.spyOn(migrationManager as any, 'getAllRecordsForModel').mockResolvedValue([
        { id: 'record-1', name: 'Test 1' },
        { id: 'record-2', name: 'Test 2' }
      ]);
      jest.spyOn(migrationManager as any, 'updateRecord').mockResolvedValue(undefined);
      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([]);
      jest.spyOn(migrationManager as any, 'recordMigrationResult').mockResolvedValue(undefined);
    });

    it('should run migration successfully', async () => {
      const result = await migrationManager.runMigration(migration.id);

      expect(result.success).toBe(true);
      expect(result.migrationId).toBe(migration.id);
      expect(result.recordsProcessed).toBe(2);
      expect(result.rollbackAvailable).toBe(true);
      expect(mockLogger.info).toHaveBeenCalledWith(
        `Migration completed: ${migration.name}`,
        expect.objectContaining({
          migrationId: migration.id,
          recordsProcessed: 2
        })
      );
    });

    it('should perform dry run without modifying data', async () => {
      jest.spyOn(migrationManager as any, 'countRecordsForModel').mockResolvedValue(2);

      const result = await migrationManager.runMigration(migration.id, { dryRun: true });

      expect(result.success).toBe(true);
      expect(result.warnings).toContain('This was a dry run - no data was actually modified');
      expect(migrationManager as any).not.toHaveProperty('updateRecord');
      expect(mockLogger.info).toHaveBeenCalledWith(
        `Performing dry run for migration: ${migration.name}`
      );
    });

    it('should throw error for non-existent migration', async () => {
      await expect(migrationManager.runMigration('non-existent')).rejects.toThrow(
        'Migration non-existent not found'
      );
    });

    it('should throw error for already running migration', async () => {
      // Start first migration (don't await)
      const promise1 = migrationManager.runMigration(migration.id);

      // Try to start same migration again
      await expect(migrationManager.runMigration(migration.id)).rejects.toThrow(
        `Migration ${migration.id} is already running`
      );

      // Clean up first migration
      await promise1;
    });

    it('should handle migration with dependencies', async () => {
      const dependentMigration = createTestMigration({
        id: 'dependent-migration',
        version: '2.0.0',
        dependencies: ['test-migration-1']
      });

      migrationManager.registerMigration(dependentMigration);

      // Mock that dependency is not applied
      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([]);

      await expect(migrationManager.runMigration(dependentMigration.id)).rejects.toThrow(
        'Migration dependency not satisfied: test-migration-1'
      );
    });
  });

  describe('Migration Rollback', () => {
    let migration: Migration;

    beforeEach(() => {
      migration = createTestMigration();
      migrationManager.registerMigration(migration);

      jest.spyOn(migrationManager as any, 'getAllRecordsForModel').mockResolvedValue([
        { id: 'record-1', name: 'Test 1', newField: 'default-value' },
        { id: 'record-2', name: 'Test 2', newField: 'default-value' }
      ]);
      jest.spyOn(migrationManager as any, 'updateRecord').mockResolvedValue(undefined);
      jest.spyOn(migrationManager as any, 'recordMigrationResult').mockResolvedValue(undefined);
    });

    it('should rollback applied migration', async () => {
      // Mock that migration was applied
      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([
        { migrationId: migration.id, success: true }
      ]);

      const result = await migrationManager.rollbackMigration(migration.id);

      expect(result.success).toBe(true);
      expect(result.migrationId).toBe(migration.id);
      expect(result.rollbackAvailable).toBe(false);
      expect(mockLogger.info).toHaveBeenCalledWith(
        `Rollback completed: ${migration.name}`,
        expect.objectContaining({ migrationId: migration.id })
      );
    });

    it('should throw error for non-existent migration rollback', async () => {
      await expect(migrationManager.rollbackMigration('non-existent')).rejects.toThrow(
        'Migration non-existent not found'
      );
    });

    it('should throw error for unapplied migration rollback', async () => {
      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([]);

      await expect(migrationManager.rollbackMigration(migration.id)).rejects.toThrow(
        `Migration ${migration.id} has not been applied`
      );
    });

    it('should handle migration without rollback operations', async () => {
      const migrationWithoutRollback = createTestMigration({
        id: 'no-rollback',
        down: []
      });
      migrationManager.registerMigration(migrationWithoutRollback);

      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([
        { migrationId: 'no-rollback', success: true }
      ]);

      await expect(migrationManager.rollbackMigration('no-rollback')).rejects.toThrow(
        'Migration has no rollback operations defined'
      );
    });
  });

  describe('Batch Migration Operations', () => {
    beforeEach(() => {
      const migration1 = createTestMigration({ id: 'migration-1', version: '1.0.0' });
      const migration2 = createTestMigration({ id: 'migration-2', version: '2.0.0' });
      const migration3 = createTestMigration({ id: 'migration-3', version: '3.0.0' });

      migrationManager.registerMigration(migration1);
      migrationManager.registerMigration(migration2);
      migrationManager.registerMigration(migration3);

      jest.spyOn(migrationManager as any, 'getAllRecordsForModel').mockResolvedValue([]);
      jest.spyOn(migrationManager as any, 'updateRecord').mockResolvedValue(undefined);
      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([]);
      jest.spyOn(migrationManager as any, 'recordMigrationResult').mockResolvedValue(undefined);
    });

    it('should run all pending migrations', async () => {
      const results = await migrationManager.runPendingMigrations();

      expect(results).toHaveLength(3);
      expect(results.every(r => r.success)).toBe(true);
      expect(mockLogger.info).toHaveBeenCalledWith(
        'Running 3 pending migrations',
        expect.objectContaining({ dryRun: false })
      );
    });

    it('should run pending migrations for specific model', async () => {
      const migration4 = createTestMigration({
        id: 'migration-4',
        version: '4.0.0',
        targetModels: ['OtherModel']
      });
      migrationManager.registerMigration(migration4);

      const results = await migrationManager.runPendingMigrations({ modelName: 'TestModel' });

      expect(results).toHaveLength(3); // Should exclude migration-4
    });

    it('should stop on error when specified', async () => {
      // Make second migration fail
      jest.spyOn(migrationManager, 'runMigration')
        .mockResolvedValueOnce({ success: true } as MigrationResult)
        .mockRejectedValueOnce(new Error('Migration failed'));

      await expect(
        migrationManager.runPendingMigrations({ stopOnError: true })
      ).rejects.toThrow('Migration failed');
    });

    it('should continue on error when not specified', async () => {
      // Make second migration fail
      jest.spyOn(migrationManager, 'runMigration')
        .mockResolvedValueOnce({ success: true } as MigrationResult)
        .mockRejectedValueOnce(new Error('Migration failed'))
        .mockResolvedValueOnce({ success: true } as MigrationResult);

      const results = await migrationManager.runPendingMigrations({ stopOnError: false });

      expect(results).toHaveLength(2); // Only successful migrations
      expect(mockLogger.error).toHaveBeenCalledWith(
        'Skipping failed migration: migration-2',
        expect.objectContaining({ error: expect.any(Error) })
      );
    });
  });

  describe('Migration Validation', () => {
    it('should run pre-migration validators', async () => {
      const validator: MigrationValidator = {
        name: 'Test Validator',
        description: 'Tests migration validity',
        validate: jest.fn().mockResolvedValue({
          valid: true,
          errors: [],
          warnings: ['Test warning']
        })
      };

      const migration = createTestMigration({
        validators: [validator]
      });

      migrationManager.registerMigration(migration);
      jest.spyOn(migrationManager as any, 'getAllRecordsForModel').mockResolvedValue([]);
      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([]);
      jest.spyOn(migrationManager as any, 'recordMigrationResult').mockResolvedValue(undefined);

      await migrationManager.runMigration(migration.id);

      expect(validator.validate).toHaveBeenCalled();
      expect(mockLogger.debug).toHaveBeenCalledWith(
        `Running pre-migration validator: ${validator.name}`
      );
    });

    it('should fail migration on validation error', async () => {
      const validator: MigrationValidator = {
        name: 'Failing Validator',
        description: 'Always fails',
        validate: jest.fn().mockResolvedValue({
          valid: false,
          errors: ['Validation failed'],
          warnings: []
        })
      };

      const migration = createTestMigration({
        validators: [validator]
      });

      migrationManager.registerMigration(migration);
      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([]);

      await expect(migrationManager.runMigration(migration.id)).rejects.toThrow(
        'Pre-migration validation failed: Validation failed'
      );
    });
  });

  describe('Migration Progress and Monitoring', () => {
    it('should track migration progress', async () => {
      const migration = createTestMigration();
      migrationManager.registerMigration(migration);

      jest.spyOn(migrationManager as any, 'getAllRecordsForModel').mockResolvedValue([
        { id: 'record-1' }
      ]);
      jest.spyOn(migrationManager as any, 'updateRecord').mockResolvedValue(undefined);
      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([]);
      jest.spyOn(migrationManager as any, 'recordMigrationResult').mockResolvedValue(undefined);

      const migrationPromise = migrationManager.runMigration(migration.id);
      
      // Check progress while migration is running
      const progress = migrationManager.getMigrationProgress(migration.id);
      expect(progress).toBeDefined();
      expect(progress?.status).toBe('running');

      await migrationPromise;

      // Progress should be cleared after completion
      const finalProgress = migrationManager.getMigrationProgress(migration.id);
      expect(finalProgress).toBeNull();
    });

    it('should get active migrations', async () => {
      const migration1 = createTestMigration({ id: 'migration-1' });
      const migration2 = createTestMigration({ id: 'migration-2' });

      migrationManager.registerMigration(migration1);
      migrationManager.registerMigration(migration2);

      jest.spyOn(migrationManager as any, 'getAllRecordsForModel').mockResolvedValue([]);
      jest.spyOn(migrationManager as any, 'getAppliedMigrations').mockReturnValue([]);
      jest.spyOn(migrationManager as any, 'recordMigrationResult').mockResolvedValue(undefined);

      // Start migrations but don't await
      const promise1 = migrationManager.runMigration(migration1.id);
      const promise2 = migrationManager.runMigration(migration2.id);

      const activeMigrations = migrationManager.getActiveMigrations();
      expect(activeMigrations).toHaveLength(2);
      expect(activeMigrations.every(p => p.status === 'running')).toBe(true);

      await Promise.all([promise1, promise2]);
    });

    it('should get migration history', () => {
      // Manually add some history
      const result1: MigrationResult = {
        migrationId: 'migration-1',
        success: true,
        duration: 1000,
        recordsProcessed: 10,
        recordsModified: 5,
        warnings: [],
        errors: [],
        rollbackAvailable: true
      };

      const result2: MigrationResult = {
        migrationId: 'migration-2',
        success: false,
        duration: 500,
        recordsProcessed: 5,
        recordsModified: 0,
        warnings: [],
        errors: ['Test error'],
        rollbackAvailable: false
      };

      (migrationManager as any).migrationHistory.set('migration-1', [result1]);
      (migrationManager as any).migrationHistory.set('migration-2', [result2]);

      const allHistory = migrationManager.getMigrationHistory();
      expect(allHistory).toHaveLength(2);

      const specificHistory = migrationManager.getMigrationHistory('migration-1');
      expect(specificHistory).toEqual([result1]);
    });
  });

  describe('Version Comparison', () => {
    it('should compare versions correctly', () => {
      const compareVersions = (migrationManager as any).compareVersions.bind(migrationManager);

      expect(compareVersions('1.0.0', '2.0.0')).toBe(-1);
      expect(compareVersions('2.0.0', '1.0.0')).toBe(1);
      expect(compareVersions('1.0.0', '1.0.0')).toBe(0);
      expect(compareVersions('1.2.0', '1.1.0')).toBe(1);
      expect(compareVersions('1.0.1', '1.0.0')).toBe(1);
      expect(compareVersions('1.0', '1.0.0')).toBe(0);
    });
  });

  describe('Field Value Conversion', () => {
    it('should convert field values correctly', () => {
      const convertFieldValue = (migrationManager as any).convertFieldValue.bind(migrationManager);

      expect(convertFieldValue('123', { type: 'number' })).toBe(123);
      expect(convertFieldValue(123, { type: 'string' })).toBe('123');
      expect(convertFieldValue('true', { type: 'boolean' })).toBe(true);
      expect(convertFieldValue('test', { type: 'array' })).toEqual(['test']);
      expect(convertFieldValue(['test'], { type: 'array' })).toEqual(['test']);
      expect(convertFieldValue(null, { type: 'string' })).toBeNull();
    });
  });

  describe('Cleanup', () => {
    it('should cleanup resources', async () => {
      await migrationManager.cleanup();

      expect(migrationManager.getActiveMigrations()).toHaveLength(0);
      expect(mockLogger.info).toHaveBeenCalledWith('Cleaning up migration manager');
    });
  });
});