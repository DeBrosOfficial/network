/**
 * MigrationManager - Schema Migration and Data Transformation System
 *
 * This class handles:
 * - Schema version management across distributed databases
 * - Automatic data migration and transformation
 * - Rollback capabilities for failed migrations
 * - Conflict resolution during migration
 * - Migration validation and integrity checks
 * - Cross-shard migration coordination
 */

import { FieldConfig } from '../types/models';

export interface Migration {
  id: string;
  version: string;
  name: string;
  description: string;
  targetModels: string[];
  up: MigrationOperation[];
  down: MigrationOperation[];
  dependencies?: string[]; // Migration IDs that must run before this one
  validators?: MigrationValidator[];
  createdAt: number;
  author?: string;
  tags?: string[];
}

export interface MigrationOperation {
  type:
    | 'add_field'
    | 'remove_field'
    | 'modify_field'
    | 'rename_field'
    | 'add_index'
    | 'remove_index'
    | 'transform_data'
    | 'custom';
  modelName: string;
  fieldName?: string;
  newFieldName?: string;
  fieldConfig?: FieldConfig;
  indexConfig?: any;
  transformer?: (data: any) => any;
  customOperation?: (context: MigrationContext) => Promise<void>;
  rollbackOperation?: (context: MigrationContext) => Promise<void>;
  options?: {
    batchSize?: number;
    parallel?: boolean;
    skipValidation?: boolean;
  };
}

export interface MigrationValidator {
  name: string;
  description: string;
  validate: (context: MigrationContext) => Promise<ValidationResult>;
}

export interface MigrationContext {
  migration: Migration;
  modelName: string;
  databaseManager: any;
  shardManager: any;
  currentData?: any[];
  operation: MigrationOperation;
  progress: MigrationProgress;
  logger: MigrationLogger;
}

export interface MigrationProgress {
  migrationId: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'rolled_back';
  startedAt?: number;
  completedAt?: number;
  totalRecords: number;
  processedRecords: number;
  errorCount: number;
  warnings: string[];
  errors: string[];
  currentOperation?: string;
  estimatedTimeRemaining?: number;
}

export interface MigrationResult {
  migrationId: string;
  success: boolean;
  duration: number;
  recordsProcessed: number;
  recordsModified: number;
  warnings: string[];
  errors: string[];
  rollbackAvailable: boolean;
}

export interface MigrationLogger {
  info: (message: string, meta?: any) => void;
  warn: (message: string, meta?: any) => void;
  error: (message: string, meta?: any) => void;
  debug: (message: string, meta?: any) => void;
}

export interface ValidationResult {
  valid: boolean;
  errors: string[];
  warnings: string[];
}

export class MigrationManager {
  private databaseManager: any;
  private shardManager: any;
  private migrations: Map<string, Migration> = new Map();
  private migrationHistory: Map<string, MigrationResult[]> = new Map();
  private activeMigrations: Map<string, MigrationProgress> = new Map();
  private migrationOrder: string[] = [];
  private logger: MigrationLogger;

  constructor(databaseManager: any, shardManager: any, logger?: MigrationLogger) {
    this.databaseManager = databaseManager;
    this.shardManager = shardManager;
    this.logger = logger || this.createDefaultLogger();
  }

  // Register a new migration
  registerMigration(migration: Migration): void {
    // Validate migration structure
    this.validateMigrationStructure(migration);

    // Check for version conflicts
    const existingMigration = Array.from(this.migrations.values()).find(
      (m) => m.version === migration.version,
    );

    if (existingMigration && existingMigration.id !== migration.id) {
      throw new Error(`Migration version ${migration.version} already exists with different ID`);
    }

    this.migrations.set(migration.id, migration);
    this.updateMigrationOrder();

    this.logger.info(`Registered migration: ${migration.name} (${migration.version})`, {
      migrationId: migration.id,
      targetModels: migration.targetModels,
    });
  }

  // Get all registered migrations
  getMigrations(): Migration[] {
    return Array.from(this.migrations.values()).sort((a, b) =>
      this.compareVersions(a.version, b.version),
    );
  }

  // Get migration by ID
  getMigration(migrationId: string): Migration | null {
    return this.migrations.get(migrationId) || null;
  }

  // Get pending migrations for a model or all models
  getPendingMigrations(modelName?: string): Migration[] {
    const allMigrations = this.getMigrations();
    const appliedMigrations = this.getAppliedMigrations(modelName);
    const appliedIds = new Set(appliedMigrations.map((m) => m.migrationId));

    return allMigrations.filter((migration) => {
      if (!appliedIds.has(migration.id)) {
        return modelName ? migration.targetModels.includes(modelName) : true;
      }
      return false;
    });
  }

  // Run a specific migration
  async runMigration(
    migrationId: string,
    options: {
      dryRun?: boolean;
      batchSize?: number;
      parallelShards?: boolean;
      skipValidation?: boolean;
    } = {},
  ): Promise<MigrationResult> {
    const migration = this.migrations.get(migrationId);
    if (!migration) {
      throw new Error(`Migration ${migrationId} not found`);
    }

    // Check if migration is already running
    if (this.activeMigrations.has(migrationId)) {
      throw new Error(`Migration ${migrationId} is already running`);
    }

    // Check dependencies
    await this.validateDependencies(migration);

    const startTime = Date.now();
    const progress: MigrationProgress = {
      migrationId,
      status: 'running',
      startedAt: startTime,
      totalRecords: 0,
      processedRecords: 0,
      errorCount: 0,
      warnings: [],
      errors: [],
    };

    this.activeMigrations.set(migrationId, progress);

    try {
      this.logger.info(`Starting migration: ${migration.name}`, {
        migrationId,
        dryRun: options.dryRun,
        options,
      });

      if (options.dryRun) {
        return await this.performDryRun(migration, options);
      }

      // Pre-migration validation
      if (!options.skipValidation) {
        await this.runPreMigrationValidation(migration);
      }

      // Execute migration operations
      const result = await this.executeMigration(migration, options, progress);

      // Post-migration validation
      if (!options.skipValidation) {
        await this.runPostMigrationValidation(migration);
      }

      // Record successful migration
      progress.status = 'completed';
      progress.completedAt = Date.now();

      await this.recordMigrationResult(result);

      this.logger.info(`Migration completed: ${migration.name}`, {
        migrationId,
        duration: result.duration,
        recordsProcessed: result.recordsProcessed,
      });

      return result;
    } catch (error: any) {
      progress.status = 'failed';
      progress.errors.push(error.message);

      this.logger.error(`Migration failed: ${migration.name}`, {
        migrationId,
        error: error.message,
        stack: error.stack,
      });

      // Attempt rollback if possible
      const rollbackResult = await this.attemptRollback(migration, progress);

      const result: MigrationResult = {
        migrationId,
        success: false,
        duration: Date.now() - startTime,
        recordsProcessed: progress.processedRecords,
        recordsModified: 0,
        warnings: progress.warnings,
        errors: progress.errors,
        rollbackAvailable: rollbackResult.success,
      };

      await this.recordMigrationResult(result);
      throw error;
    } finally {
      this.activeMigrations.delete(migrationId);
    }
  }

  // Run all pending migrations
  async runPendingMigrations(
    options: {
      modelName?: string;
      dryRun?: boolean;
      stopOnError?: boolean;
      batchSize?: number;
    } = {},
  ): Promise<MigrationResult[]> {
    const pendingMigrations = this.getPendingMigrations(options.modelName);
    const results: MigrationResult[] = [];

    this.logger.info(`Running ${pendingMigrations.length} pending migrations`, {
      modelName: options.modelName,
      dryRun: options.dryRun,
    });

    for (const migration of pendingMigrations) {
      try {
        const result = await this.runMigration(migration.id, {
          dryRun: options.dryRun,
          batchSize: options.batchSize,
        });
        results.push(result);

        if (!result.success && options.stopOnError) {
          this.logger.warn('Stopping migration run due to error', {
            failedMigration: migration.id,
            stopOnError: options.stopOnError,
          });
          break;
        }
      } catch (error) {
        if (options.stopOnError) {
          throw error;
        }
        this.logger.error(`Skipping failed migration: ${migration.id}`, { error });
      }
    }

    return results;
  }

  // Rollback a migration
  async rollbackMigration(migrationId: string): Promise<MigrationResult> {
    const migration = this.migrations.get(migrationId);
    if (!migration) {
      throw new Error(`Migration ${migrationId} not found`);
    }

    const appliedMigrations = this.getAppliedMigrations();
    const isApplied = appliedMigrations.some((m) => m.migrationId === migrationId && m.success);

    if (!isApplied) {
      throw new Error(`Migration ${migrationId} has not been applied`);
    }

    const startTime = Date.now();
    const progress: MigrationProgress = {
      migrationId,
      status: 'running',
      startedAt: startTime,
      totalRecords: 0,
      processedRecords: 0,
      errorCount: 0,
      warnings: [],
      errors: [],
    };

    try {
      this.logger.info(`Starting rollback: ${migration.name}`, { migrationId });

      const result = await this.executeRollback(migration, progress);

      result.rollbackAvailable = false;
      await this.recordMigrationResult(result);

      this.logger.info(`Rollback completed: ${migration.name}`, {
        migrationId,
        duration: result.duration,
      });

      return result;
    } catch (error: any) {
      this.logger.error(`Rollback failed: ${migration.name}`, {
        migrationId,
        error: error.message,
      });
      throw error;
    }
  }

  // Execute migration operations
  private async executeMigration(
    migration: Migration,
    options: any,
    progress: MigrationProgress,
  ): Promise<MigrationResult> {
    const startTime = Date.now();
    let totalProcessed = 0;
    let totalModified = 0;

    for (const modelName of migration.targetModels) {
      for (const operation of migration.up) {
        if (operation.modelName !== modelName) continue;

        progress.currentOperation = `${operation.type} on ${operation.modelName}.${operation.fieldName || 'N/A'}`;

        this.logger.debug(`Executing operation: ${progress.currentOperation}`, {
          migrationId: migration.id,
          operation: operation.type,
        });

        const context: MigrationContext = {
          migration,
          modelName,
          databaseManager: this.databaseManager,
          shardManager: this.shardManager,
          operation,
          progress,
          logger: this.logger,
        };

        const operationResult = await this.executeOperation(context, options);
        totalProcessed += operationResult.processed;
        totalModified += operationResult.modified;
        progress.processedRecords = totalProcessed;
      }
    }

    return {
      migrationId: migration.id,
      success: true,
      duration: Date.now() - startTime,
      recordsProcessed: totalProcessed,
      recordsModified: totalModified,
      warnings: progress.warnings,
      errors: progress.errors,
      rollbackAvailable: migration.down.length > 0,
    };
  }

  // Execute a single migration operation
  private async executeOperation(
    context: MigrationContext,
    options: any,
  ): Promise<{ processed: number; modified: number }> {
    const { operation } = context;

    switch (operation.type) {
      case 'add_field':
        return await this.executeAddField(context, options);

      case 'remove_field':
        return await this.executeRemoveField(context, options);

      case 'modify_field':
        return await this.executeModifyField(context, options);

      case 'rename_field':
        return await this.executeRenameField(context, options);

      case 'transform_data':
        return await this.executeDataTransformation(context, options);

      case 'custom':
        return await this.executeCustomOperation(context, options);

      default:
        throw new Error(`Unsupported operation type: ${operation.type}`);
    }
  }

  // Execute add field operation
  private async executeAddField(
    context: MigrationContext,
    options: any,
  ): Promise<{ processed: number; modified: number }> {
    const { operation } = context;

    if (!operation.fieldName || !operation.fieldConfig) {
      throw new Error('Add field operation requires fieldName and fieldConfig');
    }

    // Update model metadata (in a real implementation, this would update the model registry)
    this.logger.info(`Adding field ${operation.fieldName} to ${operation.modelName}`, {
      fieldConfig: operation.fieldConfig,
    });

    // Get all records for this model
    const records = await this.getAllRecordsForModel(operation.modelName);
    let modified = 0;

    // Add default value to existing records
    const batchSize = options.batchSize || 100;
    for (let i = 0; i < records.length; i += batchSize) {
      const batch = records.slice(i, i + batchSize);

      for (const record of batch) {
        if (!(operation.fieldName in record)) {
          record[operation.fieldName] = operation.fieldConfig.default || null;
          await this.updateRecord(operation.modelName, record);
          modified++;
        }
      }

      context.progress.processedRecords += batch.length;
    }

    return { processed: records.length, modified };
  }

  // Execute remove field operation
  private async executeRemoveField(
    context: MigrationContext,
    options: any,
  ): Promise<{ processed: number; modified: number }> {
    const { operation } = context;

    if (!operation.fieldName) {
      throw new Error('Remove field operation requires fieldName');
    }

    this.logger.info(`Removing field ${operation.fieldName} from ${operation.modelName}`);

    const records = await this.getAllRecordsForModel(operation.modelName);
    let modified = 0;

    const batchSize = options.batchSize || 100;
    for (let i = 0; i < records.length; i += batchSize) {
      const batch = records.slice(i, i + batchSize);

      for (const record of batch) {
        if (operation.fieldName in record) {
          delete record[operation.fieldName];
          await this.updateRecord(operation.modelName, record);
          modified++;
        }
      }

      context.progress.processedRecords += batch.length;
    }

    return { processed: records.length, modified };
  }

  // Execute modify field operation
  private async executeModifyField(
    context: MigrationContext,
    options: any,
  ): Promise<{ processed: number; modified: number }> {
    const { operation } = context;

    if (!operation.fieldName || !operation.fieldConfig) {
      throw new Error('Modify field operation requires fieldName and fieldConfig');
    }

    this.logger.info(`Modifying field ${operation.fieldName} in ${operation.modelName}`, {
      newConfig: operation.fieldConfig,
    });

    const records = await this.getAllRecordsForModel(operation.modelName);
    let modified = 0;

    const batchSize = options.batchSize || 100;
    for (let i = 0; i < records.length; i += batchSize) {
      const batch = records.slice(i, i + batchSize);

      for (const record of batch) {
        if (operation.fieldName in record) {
          // Apply type conversion if needed
          const oldValue = record[operation.fieldName];
          const newValue = this.convertFieldValue(oldValue, operation.fieldConfig);

          if (newValue !== oldValue) {
            record[operation.fieldName] = newValue;
            await this.updateRecord(operation.modelName, record);
            modified++;
          }
        }
      }

      context.progress.processedRecords += batch.length;
    }

    return { processed: records.length, modified };
  }

  // Execute rename field operation
  private async executeRenameField(
    context: MigrationContext,
    options: any,
  ): Promise<{ processed: number; modified: number }> {
    const { operation } = context;

    if (!operation.fieldName || !operation.newFieldName) {
      throw new Error('Rename field operation requires fieldName and newFieldName');
    }

    this.logger.info(
      `Renaming field ${operation.fieldName} to ${operation.newFieldName} in ${operation.modelName}`,
    );

    const records = await this.getAllRecordsForModel(operation.modelName);
    let modified = 0;

    const batchSize = options.batchSize || 100;
    for (let i = 0; i < records.length; i += batchSize) {
      const batch = records.slice(i, i + batchSize);

      for (const record of batch) {
        if (operation.fieldName in record) {
          record[operation.newFieldName] = record[operation.fieldName];
          delete record[operation.fieldName];
          await this.updateRecord(operation.modelName, record);
          modified++;
        }
      }

      context.progress.processedRecords += batch.length;
    }

    return { processed: records.length, modified };
  }

  // Execute data transformation operation
  private async executeDataTransformation(
    context: MigrationContext,
    options: any,
  ): Promise<{ processed: number; modified: number }> {
    const { operation } = context;

    if (!operation.transformer) {
      throw new Error('Transform data operation requires transformer function');
    }

    this.logger.info(`Transforming data for ${operation.modelName}`);

    const records = await this.getAllRecordsForModel(operation.modelName);
    let modified = 0;

    const batchSize = options.batchSize || 100;
    for (let i = 0; i < records.length; i += batchSize) {
      const batch = records.slice(i, i + batchSize);

      for (const record of batch) {
        try {
          const originalRecord = JSON.stringify(record);
          const transformedRecord = await operation.transformer(record);

          if (JSON.stringify(transformedRecord) !== originalRecord) {
            Object.assign(record, transformedRecord);
            await this.updateRecord(operation.modelName, record);
            modified++;
          }
        } catch (error: any) {
          context.progress.errors.push(`Transform error for record ${record.id}: ${error.message}`);
          context.progress.errorCount++;
        }
      }

      context.progress.processedRecords += batch.length;
    }

    return { processed: records.length, modified };
  }

  // Execute custom operation
  private async executeCustomOperation(
    context: MigrationContext,
    _options: any,
  ): Promise<{ processed: number; modified: number }> {
    const { operation } = context;

    if (!operation.customOperation) {
      throw new Error('Custom operation requires customOperation function');
    }

    this.logger.info(`Executing custom operation for ${operation.modelName}`);

    try {
      await operation.customOperation(context);
      return { processed: 1, modified: 1 }; // Custom operations handle their own counting
    } catch (error: any) {
      context.progress.errors.push(`Custom operation error: ${error.message}`);
      throw error;
    }
  }

  // Helper methods for data access
  private async getAllRecordsForModel(modelName: string): Promise<any[]> {
    // In a real implementation, this would query all shards for the model
    // For now, return empty array as placeholder
    this.logger.debug(`Getting all records for model: ${modelName}`);
    return [];
  }

  private async updateRecord(modelName: string, record: any): Promise<void> {
    // In a real implementation, this would update the record in the appropriate database
    this.logger.debug(`Updating record in ${modelName}:`, { id: record.id });
  }

  private convertFieldValue(value: any, fieldConfig: FieldConfig): any {
    // Convert value based on field configuration
    switch (fieldConfig.type) {
      case 'string':
        return value != null ? String(value) : null;
      case 'number':
        return value != null ? Number(value) : null;
      case 'boolean':
        return value != null ? Boolean(value) : null;
      case 'array':
        return Array.isArray(value) ? value : [value];
      default:
        return value;
    }
  }

  // Validation methods
  private validateMigrationStructure(migration: Migration): void {
    if (!migration.id || !migration.version || !migration.name) {
      throw new Error('Migration must have id, version, and name');
    }

    if (!migration.targetModels || migration.targetModels.length === 0) {
      throw new Error('Migration must specify target models');
    }

    if (!migration.up || migration.up.length === 0) {
      throw new Error('Migration must have at least one up operation');
    }

    // Validate operations
    for (const operation of migration.up) {
      this.validateOperation(operation);
    }

    if (migration.down) {
      for (const operation of migration.down) {
        this.validateOperation(operation);
      }
    }
  }

  private validateOperation(operation: MigrationOperation): void {
    const validTypes = [
      'add_field',
      'remove_field',
      'modify_field',
      'rename_field',
      'add_index',
      'remove_index',
      'transform_data',
      'custom',
    ];

    if (!validTypes.includes(operation.type)) {
      throw new Error(`Invalid operation type: ${operation.type}`);
    }

    if (!operation.modelName) {
      throw new Error('Operation must specify modelName');
    }
  }

  private async validateDependencies(migration: Migration): Promise<void> {
    if (!migration.dependencies) return;

    const appliedMigrations = this.getAppliedMigrations();
    const appliedIds = new Set(appliedMigrations.map((m) => m.migrationId));

    for (const dependencyId of migration.dependencies) {
      if (!appliedIds.has(dependencyId)) {
        throw new Error(`Migration dependency not satisfied: ${dependencyId}`);
      }
    }
  }

  private async runPreMigrationValidation(migration: Migration): Promise<void> {
    if (!migration.validators) return;

    for (const validator of migration.validators) {
      this.logger.debug(`Running pre-migration validator: ${validator.name}`);

      const context: MigrationContext = {
        migration,
        modelName: '', // Will be set per model
        databaseManager: this.databaseManager,
        shardManager: this.shardManager,
        operation: migration.up[0], // First operation for context
        progress: this.activeMigrations.get(migration.id)!,
        logger: this.logger,
      };

      const result = await validator.validate(context);
      if (!result.valid) {
        throw new Error(`Pre-migration validation failed: ${result.errors.join(', ')}`);
      }

      if (result.warnings.length > 0) {
        context.progress.warnings.push(...result.warnings);
      }
    }
  }

  private async runPostMigrationValidation(_migration: Migration): Promise<void> {
    // Similar to pre-migration validation but runs after
    this.logger.debug('Running post-migration validation');
  }

  // Rollback operations
  private async executeRollback(
    migration: Migration,
    progress: MigrationProgress,
  ): Promise<MigrationResult> {
    if (!migration.down || migration.down.length === 0) {
      throw new Error('Migration has no rollback operations defined');
    }

    const startTime = Date.now();
    let totalProcessed = 0;
    let totalModified = 0;

    // Execute rollback operations in reverse order
    for (const modelName of migration.targetModels) {
      for (const operation of migration.down.reverse()) {
        if (operation.modelName !== modelName) continue;

        const context: MigrationContext = {
          migration,
          modelName,
          databaseManager: this.databaseManager,
          shardManager: this.shardManager,
          operation,
          progress,
          logger: this.logger,
        };

        const operationResult = await this.executeOperation(context, {});
        totalProcessed += operationResult.processed;
        totalModified += operationResult.modified;
      }
    }

    return {
      migrationId: migration.id,
      success: true,
      duration: Date.now() - startTime,
      recordsProcessed: totalProcessed,
      recordsModified: totalModified,
      warnings: progress.warnings,
      errors: progress.errors,
      rollbackAvailable: false,
    };
  }

  private async attemptRollback(
    migration: Migration,
    progress: MigrationProgress,
  ): Promise<{ success: boolean }> {
    try {
      if (migration.down && migration.down.length > 0) {
        await this.executeRollback(migration, progress);
        progress.status = 'rolled_back';
        return { success: true };
      }
    } catch (error: any) {
      this.logger.error(`Rollback failed for migration ${migration.id}`, { error });
    }

    return { success: false };
  }

  // Dry run functionality
  private async performDryRun(migration: Migration, _options: any): Promise<MigrationResult> {
    this.logger.info(`Performing dry run for migration: ${migration.name}`);

    const startTime = Date.now();
    let estimatedRecords = 0;

    // Estimate the number of records that would be affected
    for (const modelName of migration.targetModels) {
      const modelRecords = await this.countRecordsForModel(modelName);
      estimatedRecords += modelRecords;
    }

    // Simulate operations without actually modifying data
    for (const operation of migration.up) {
      this.logger.debug(`Dry run operation: ${operation.type} on ${operation.modelName}`);
    }

    return {
      migrationId: migration.id,
      success: true,
      duration: Date.now() - startTime,
      recordsProcessed: estimatedRecords,
      recordsModified: estimatedRecords, // Estimate
      warnings: ['This was a dry run - no data was actually modified'],
      errors: [],
      rollbackAvailable: migration.down.length > 0,
    };
  }

  private async countRecordsForModel(_modelName: string): Promise<number> {
    // In a real implementation, this would count records across all shards
    return 0;
  }

  // Migration history and state management
  private getAppliedMigrations(_modelName?: string): MigrationResult[] {
    const allResults: MigrationResult[] = [];

    for (const results of this.migrationHistory.values()) {
      allResults.push(...results.filter((r) => r.success));
    }

    return allResults;
  }

  private async recordMigrationResult(result: MigrationResult): Promise<void> {
    if (!this.migrationHistory.has(result.migrationId)) {
      this.migrationHistory.set(result.migrationId, []);
    }

    this.migrationHistory.get(result.migrationId)!.push(result);

    // In a real implementation, this would persist to database
    this.logger.debug('Recorded migration result', { result });
  }

  // Version comparison
  private compareVersions(version1: string, version2: string): number {
    const v1Parts = version1.split('.').map(Number);
    const v2Parts = version2.split('.').map(Number);

    for (let i = 0; i < Math.max(v1Parts.length, v2Parts.length); i++) {
      const v1Part = v1Parts[i] || 0;
      const v2Part = v2Parts[i] || 0;

      if (v1Part < v2Part) return -1;
      if (v1Part > v2Part) return 1;
    }

    return 0;
  }

  private updateMigrationOrder(): void {
    const migrations = Array.from(this.migrations.values());
    this.migrationOrder = migrations
      .sort((a, b) => this.compareVersions(a.version, b.version))
      .map((m) => m.id);
  }

  // Utility methods
  private createDefaultLogger(): MigrationLogger {
    return {
      info: (message: string, meta?: any) => console.log(`[MIGRATION INFO] ${message}`, meta || ''),
      warn: (message: string, meta?: any) =>
        console.warn(`[MIGRATION WARN] ${message}`, meta || ''),
      error: (message: string, meta?: any) =>
        console.error(`[MIGRATION ERROR] ${message}`, meta || ''),
      debug: (message: string, meta?: any) =>
        console.log(`[MIGRATION DEBUG] ${message}`, meta || ''),
    };
  }

  // Status and monitoring
  getMigrationProgress(migrationId: string): MigrationProgress | null {
    return this.activeMigrations.get(migrationId) || null;
  }

  getActiveMigrations(): MigrationProgress[] {
    return Array.from(this.activeMigrations.values());
  }

  getMigrationHistory(migrationId?: string): MigrationResult[] {
    if (migrationId) {
      return this.migrationHistory.get(migrationId) || [];
    }

    const allResults: MigrationResult[] = [];
    for (const results of this.migrationHistory.values()) {
      allResults.push(...results);
    }

    return allResults.sort((a, b) => b.duration - a.duration);
  }

  // Cleanup and maintenance
  async cleanup(): Promise<void> {
    this.logger.info('Cleaning up migration manager');
    this.activeMigrations.clear();
  }
}
