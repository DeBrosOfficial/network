/**
 * DebrosFramework - Main Export File
 *
 * This file exports all framework components for easy import and usage.
 * It provides a clean API surface for consumers of the framework.
 */

// Main framework class
export { DebrosFramework as default, DebrosFramework } from './DebrosFramework';
export type { DebrosFrameworkConfig, FrameworkMetrics, FrameworkStatus } from './DebrosFramework';

// Core model system
export { BaseModel } from './models/BaseModel';
export { ModelRegistry } from './core/ModelRegistry';

// Decorators
export { Model } from './models/decorators/Model';
export { Field } from './models/decorators/Field';
export { BelongsTo, HasMany, HasOne, ManyToMany } from './models/decorators/relationships';
export {
  BeforeCreate,
  AfterCreate,
  BeforeUpdate,
  AfterUpdate,
  BeforeDelete,
  AfterDelete,
} from './models/decorators/hooks';

// Core services
export { DatabaseManager } from './core/DatabaseManager';
export { ShardManager } from './sharding/ShardManager';
export { ConfigManager } from './core/ConfigManager';
export { FrameworkOrbitDBService, FrameworkIPFSService } from './services/OrbitDBService';

// Query system
export { QueryBuilder } from './query/QueryBuilder';
export { QueryExecutor } from './query/QueryExecutor';
export { QueryOptimizer } from './query/QueryOptimizer';
export { QueryCache } from './query/QueryCache';

// Relationship system
export { RelationshipManager } from './relationships/RelationshipManager';
export { RelationshipCache } from './relationships/RelationshipCache';
export { LazyLoader } from './relationships/LazyLoader';
export type { RelationshipLoadOptions, EagerLoadPlan } from './relationships/RelationshipManager';

// Automatic features
export { PinningManager } from './pinning/PinningManager';
export { PubSubManager } from './pubsub/PubSubManager';

// Migration system
export { MigrationManager } from './migrations/MigrationManager';
export { MigrationBuilder, createMigration } from './migrations/MigrationBuilder';
export type {
  Migration,
  MigrationOperation,
  MigrationValidator,
  MigrationContext,
  MigrationProgress,
  MigrationResult,
} from './migrations/MigrationManager';

// Type definitions
export type {
  StoreType,
  FrameworkConfig,
  CacheConfig,
  PinningConfig,
  PinningStrategy,
  PinningStats,
  ShardingConfig,
  ValidationResult,
} from './types/framework';

export type { FieldConfig, RelationshipConfig, ModelConfig, ValidationError } from './types/models';

// Utility functions and helpers
// export { ValidationError } from './types/models'; // Already exported above

// Version information
export const FRAMEWORK_VERSION = '0.5.0-beta';
export const API_VERSION = '0.5';

// Feature flags for conditional exports
export const FEATURES = {
  MODELS: true,
  RELATIONSHIPS: true,
  QUERIES: true,
  MIGRATIONS: true,
  PINNING: true,
  PUBSUB: true,
  CACHING: true,
  SHARDING: true,
} as const;

// Quick setup helpers
import { DebrosFramework, DebrosFrameworkConfig } from './DebrosFramework';

export function createFramework(config?: DebrosFrameworkConfig) {
  return DebrosFramework.create(config);
}

export async function createFrameworkWithServices(
  orbitDBService: any,
  ipfsService: any,
  config?: DebrosFrameworkConfig,
) {
  return DebrosFramework.createWithServices(orbitDBService, ipfsService, config);
}

// Export default configuration presets
export const DEVELOPMENT_CONFIG: Partial<DebrosFrameworkConfig> = {
  environment: 'development',
  features: {
    autoMigration: true,
    automaticPinning: false,
    pubsub: true,
    queryCache: true,
    relationshipCache: true,
  },
  performance: {
    queryTimeout: 30000,
    batchSize: 50,
  },
  monitoring: {
    enableMetrics: true,
    logLevel: 'debug',
  },
};

export const PRODUCTION_CONFIG: Partial<DebrosFrameworkConfig> = {
  environment: 'production',
  features: {
    autoMigration: false, // Require manual migration in production
    automaticPinning: true,
    pubsub: true,
    queryCache: true,
    relationshipCache: true,
  },
  performance: {
    queryTimeout: 10000,
    batchSize: 200,
    maxConcurrentOperations: 500,
  },
  monitoring: {
    enableMetrics: true,
    logLevel: 'warn',
    metricsInterval: 30000,
  },
};

export const TEST_CONFIG: Partial<DebrosFrameworkConfig> = {
  environment: 'test',
  features: {
    autoMigration: true,
    automaticPinning: false,
    pubsub: false,
    queryCache: false,
    relationshipCache: false,
  },
  performance: {
    queryTimeout: 5000,
    batchSize: 10,
  },
  monitoring: {
    enableMetrics: false,
    logLevel: 'error',
  },
};
