# DebrosFramework Technical Reference

## Core Architecture Components

### DebrosFramework Main Class
**Location**: `src/framework/DebrosFramework.ts`

The main framework class that orchestrates all components:

```typescript
export class DebrosFramework {
  // Core services
  private orbitDBService: FrameworkOrbitDBService;
  private ipfsService: FrameworkIPFSService;
  
  // Framework components
  private databaseManager: DatabaseManager;
  private shardManager: ShardManager;
  private queryCache: QueryCache;
  private relationshipManager: RelationshipManager;
  private pinningManager: PinningManager;
  private pubsubManager: PubSubManager;
  private migrationManager: MigrationManager;
  
  // Lifecycle methods
  async initialize(orbitDBService?, ipfsService?, config?): Promise<void>
  async start(): Promise<void>
  async stop(): Promise<void>
  getStatus(): FrameworkStatus
  getMetrics(): FrameworkMetrics
}
```

### BaseModel Class
**Location**: `src/framework/models/BaseModel.ts`

Abstract base class for all data models:

```typescript
export abstract class BaseModel {
  // Instance properties
  public id: string;
  public _loadedRelations: Map<string, any>;
  protected _isDirty: boolean;
  protected _isNew: boolean;
  
  // Static configuration
  static modelName: string;
  static storeType: StoreType;
  static scope: 'user' | 'global';
  static sharding?: ShardingConfig;
  static fields: Map<string, FieldConfig>;
  static relationships: Map<string, RelationshipConfig>;
  
  // CRUD operations
  async save(): Promise<this>
  static async create<T>(data: any): Promise<T>
  static async findById<T>(id: string): Promise<T | null>
  static query<T>(): QueryBuilder<T>
  async delete(): Promise<void>
  
  // Lifecycle hooks
  async beforeCreate(): Promise<void>
  async afterCreate(): Promise<void>
  async beforeUpdate(): Promise<void>
  async afterUpdate(): Promise<void>
  async beforeDelete(): Promise<void>
  async afterDelete(): Promise<void>
}
```

## Decorator System

### @Model Decorator
**Location**: `src/framework/models/decorators/Model.ts`

Configures model behavior and database storage:

```typescript
interface ModelConfig {
  scope: 'user' | 'global';        // Database scope
  type: StoreType;                 // OrbitDB store type
  sharding?: ShardingConfig;       // Data distribution strategy
  pinning?: PinningConfig;         // Automatic pinning configuration
  pubsub?: PubSubConfig;          // Event publishing configuration
  validation?: ValidationConfig;   // Model-level validation
}

@Model({
  scope: 'user',
  type: 'docstore',
  sharding: { strategy: 'hash', count: 4, key: 'userId' }
})
```

### @Field Decorator
**Location**: `src/framework/models/decorators/Field.ts`

Defines field properties and validation:

```typescript
interface FieldConfig {
  type: FieldType;                    // Data type
  required?: boolean;                 // Required field
  unique?: boolean;                   // Unique constraint
  default?: any | (() => any);        // Default value
  validate?: (value: any) => boolean; // Custom validation
  transform?: (value: any) => any;    // Data transformation
  serialize?: boolean;                // Include in serialization
  index?: boolean;                    // Create index for field
  virtual?: boolean;                  // Virtual field (not stored)
}

@Field({
  type: 'string',
  required: true,
  unique: true,
  validate: (value: string) => value.length >= 3,
  transform: (value: string) => value.trim().toLowerCase()
})
```

### Relationship Decorators
**Location**: `src/framework/models/decorators/relationships.ts`

```typescript
// Many-to-one relationship
@BelongsTo(() => User, 'userId')
author: User;

// One-to-many relationship
@HasMany(() => Post, 'authorId')
posts: Post[];

// One-to-one relationship
@HasOne(() => Profile, 'userId')
profile: Profile;

// Many-to-many relationship
@ManyToMany(() => Tag, 'post_tags', 'tag_id', 'post_id')
tags: Tag[];
```

### Lifecycle Hook Decorators
**Location**: `src/framework/models/decorators/hooks.ts`

```typescript
@BeforeCreate()
setupDefaults() {
  this.createdAt = Date.now();
}

@AfterCreate()
async sendNotification() {
  await this.notifyUsers();
}

@BeforeUpdate()
updateTimestamp() {
  this.updatedAt = Date.now();
}

@AfterUpdate()
async invalidateCache() {
  await this.clearRelatedCache();
}

@BeforeDelete()
async checkPermissions() {
  if (!this.canDelete()) {
    throw new Error('Cannot delete this record');
  }
}

@AfterDelete()
async cleanupRelations() {
  await this.removeRelatedData();
}
```

## Query System

### QueryBuilder Class
**Location**: `src/framework/query/QueryBuilder.ts`

Fluent interface for building queries:

```typescript
export class QueryBuilder<T> {
  // Filter methods
  where(field: string, value: any): QueryBuilder<T>
  where(field: string, operator: string, value: any): QueryBuilder<T>
  where(callback: (query: QueryBuilder<T>) => void): QueryBuilder<T>
  orWhere(field: string, value: any): QueryBuilder<T>
  whereIn(field: string, values: any[]): QueryBuilder<T>
  whereNotIn(field: string, values: any[]): QueryBuilder<T>
  whereNull(field: string): QueryBuilder<T>
  whereNotNull(field: string): QueryBuilder<T>
  whereLike(field: string, pattern: string): QueryBuilder<T>
  
  // Relationship methods
  with(relations: string[]): QueryBuilder<T>
  withCount(relations: string[]): QueryBuilder<T>
  
  // Ordering and limiting
  orderBy(field: string, direction?: 'asc' | 'desc'): QueryBuilder<T>
  limit(count: number): QueryBuilder<T>
  offset(count: number): QueryBuilder<T>
  
  // Field selection
  select(fields: string[]): QueryBuilder<T>
  distinct(field?: string): QueryBuilder<T>
  
  // Caching
  cache(ttl?: number): QueryBuilder<T>
  
  // Execution methods
  find(): Promise<T[]>
  findOne(): Promise<T | null>
  first(): Promise<T | null>
  count(): Promise<number>
  exists(): Promise<boolean>
  paginate(page: number, perPage: number): Promise<PaginationResult<T>>
  
  // Aggregation
  sum(field: string): Promise<number>
  avg(field: string): Promise<number>
  min(field: string): Promise<any>
  max(field: string): Promise<any>
}
```

### Query Operators
```typescript
type QueryOperator = 
  | 'eq'          // Equal to
  | 'ne'          // Not equal to
  | 'gt'          // Greater than
  | 'gte'         // Greater than or equal
  | 'lt'          // Less than
  | 'lte'         // Less than or equal
  | 'in'          // In array
  | 'not in'      // Not in array
  | 'like'        // Pattern matching
  | 'regex'       // Regular expression
  | 'is null'     // Is null
  | 'is not null' // Is not null
  | 'includes'    // Array includes value
  | 'includes any'// Array includes any of values
  | 'includes all'// Array includes all values
```

## Database Management

### DatabaseManager Class
**Location**: `src/framework/core/DatabaseManager.ts`

Handles database creation and lifecycle:

```typescript
export class DatabaseManager {
  // Database operations
  async getGlobalDatabase(modelName: string): Promise<Database>
  async getUserDatabase(userId: string, modelName: string): Promise<Database>
  async createDatabase(name: string, type: StoreType, options?: any): Promise<Database>
  async closeDatabase(name: string): Promise<void>
  
  // Document operations
  async getDocument(database: Database, storeType: StoreType, id: string): Promise<any>
  async putDocument(database: Database, storeType: StoreType, id: string, data: any): Promise<void>
  async deleteDocument(database: Database, storeType: StoreType, id: string): Promise<void>
  async queryDocuments(database: Database, storeType: StoreType, query: any): Promise<any[]>
  
  // Lifecycle
  async initialize(orbitDBService: FrameworkOrbitDBService): Promise<void>
  async stop(): Promise<void>
}
```

### ShardManager Class
**Location**: `src/framework/sharding/ShardManager.ts`

Handles data distribution across shards:

```typescript
export class ShardManager {
  // Shard operations
  getShardForKey(modelName: string, key: string): Shard
  getShardForRange(modelName: string, value: any): Shard
  getAllShards(modelName: string): Shard[]
  
  // Shard management
  async createShards(modelName: string, config: ShardingConfig): Promise<void>
  async redistributeData(modelName: string, newShardCount: number): Promise<void>
  
  // Configuration
  setShardingConfig(modelName: string, config: ShardingConfig): void
  getShardingConfig(modelName: string): ShardingConfig | undefined
}

interface ShardingConfig {
  strategy: 'hash' | 'range' | 'user';
  count: number;
  key: string;
  ranges?: ShardRange[];
}

interface Shard {
  id: string;
  database: Database;
  range?: { min: any; max: any };
}
```

## Relationship Management

### RelationshipManager Class
**Location**: `src/framework/relationships/RelationshipManager.ts`

Handles loading and caching of model relationships:

```typescript
export class RelationshipManager {
  // Relationship loading
  async loadRelationship(model: BaseModel, relationshipName: string): Promise<any>
  async loadRelationships(model: BaseModel, relationshipNames: string[]): Promise<void>
  async eagerLoadRelationships(models: BaseModel[], relationshipNames: string[]): Promise<void>
  
  // Relationship operations
  async attachRelationship(model: BaseModel, relationshipName: string, relatedModel: BaseModel): Promise<void>
  async detachRelationship(model: BaseModel, relationshipName: string, relatedModel: BaseModel): Promise<void>
  async syncRelationship(model: BaseModel, relationshipName: string, relatedModels: BaseModel[]): Promise<void>
  
  // Caching
  getCachedRelationship(model: BaseModel, relationshipName: string): any
  setCachedRelationship(model: BaseModel, relationshipName: string, data: any): void
  clearRelationshipCache(model: BaseModel, relationshipName?: string): void
}
```

## Migration System

### MigrationManager Class
**Location**: `src/framework/migrations/MigrationManager.ts`

Handles schema evolution and data transformation:

```typescript
export class MigrationManager {
  // Migration operations
  async runMigration(migrationId: string): Promise<MigrationResult>
  async rollbackMigration(migrationId: string): Promise<MigrationResult>
  async runPendingMigrations(): Promise<MigrationResult[]>
  
  // Migration management
  registerMigration(migration: Migration): void
  getPendingMigrations(): Migration[]
  getAppliedMigrations(): Promise<string[]>
  
  // Status
  getMigrationStatus(): Promise<MigrationStatus>
}

interface Migration {
  id: string;
  version: string;
  name: string;
  description: string;
  targetModels: string[];
  up: MigrationOperation[];
  down: MigrationOperation[];
  dependencies?: string[];
  validators?: MigrationValidator[];
}
```

### MigrationBuilder Class
**Location**: `src/framework/migrations/MigrationBuilder.ts`

Fluent interface for creating migrations:

```typescript
export function createMigration(name: string, version: string): MigrationBuilder {
  return new MigrationBuilder(name, version);
}

export class MigrationBuilder {
  // Field operations
  addField(modelName: string, fieldName: string, config: FieldConfig): MigrationBuilder
  removeField(modelName: string, fieldName: string): MigrationBuilder
  modifyField(modelName: string, fieldName: string, config: FieldConfig): MigrationBuilder
  renameField(modelName: string, oldName: string, newName: string): MigrationBuilder
  
  // Index operations
  addIndex(modelName: string, fields: string[], options?: IndexOptions): MigrationBuilder
  removeIndex(modelName: string, indexName: string): MigrationBuilder
  
  // Data transformation
  transformData(modelName: string, transformer: (data: any) => any): MigrationBuilder
  
  // Validation
  addValidator(name: string, validator: MigrationValidator): MigrationBuilder
  
  // Build migration
  build(): Migration
}
```

## Caching System

### QueryCache Class
**Location**: `src/framework/query/QueryCache.ts`

Intelligent caching of query results:

```typescript
export class QueryCache {
  // Cache operations
  get(key: string): Promise<any>
  set(key: string, value: any, ttl?: number): Promise<void>
  delete(key: string): Promise<void>
  clear(): Promise<void>
  
  // Cache management
  invalidateModelCache(modelName: string): Promise<void>
  invalidateUserCache(userId: string): Promise<void>
  
  // Statistics
  getStats(): CacheStats
  getHitRate(): number
}
```

### RelationshipCache Class
**Location**: `src/framework/relationships/RelationshipCache.ts`

Specialized caching for relationship data:

```typescript
export class RelationshipCache {
  // Relationship caching
  getCachedRelationship(modelId: string, relationshipName: string): any
  setCachedRelationship(modelId: string, relationshipName: string, data: any, ttl?: number): void
  invalidateRelationship(modelId: string, relationshipName: string): void
  
  // Batch operations
  preloadRelationships(modelIds: string[], relationshipNames: string[]): Promise<void>
  warmCache(modelName: string, relationshipName: string): Promise<void>
}
```

## Type Definitions

### Framework Types
**Location**: `src/framework/types/framework.ts`

```typescript
export interface FrameworkConfig {
  cache?: CacheConfig;
  queryOptimization?: QueryOptimizationConfig;
  automaticPinning?: PinningConfig;
  pubsub?: PubSubConfig;
  development?: DevelopmentConfig;
}

export interface CacheConfig {
  enabled?: boolean;
  maxSize?: number;
  ttl?: number;
}

export type StoreType = 'docstore' | 'eventlog' | 'keyvalue' | 'counter' | 'feed';
```

### Model Types
**Location**: `src/framework/types/models.ts`

```typescript
export interface FieldConfig {
  type: FieldType;
  required?: boolean;
  unique?: boolean;
  default?: any | (() => any);
  validate?: (value: any) => boolean;
  transform?: (value: any) => any;
  serialize?: boolean;
  index?: boolean;
  virtual?: boolean;
}

export type FieldType = 'string' | 'number' | 'boolean' | 'array' | 'object' | 'date';

export interface RelationshipConfig {
  type: 'belongsTo' | 'hasMany' | 'hasOne' | 'manyToMany';
  target: () => typeof BaseModel;
  foreignKey: string;
  localKey?: string;
  through?: string;
  throughForeignKey?: string;
  throughLocalKey?: string;
}
```

### Query Types
**Location**: `src/framework/types/queries.ts`

```typescript
export interface QueryOptions {
  where?: WhereClause[];
  orderBy?: OrderByClause[];
  limit?: number;
  offset?: number;
  with?: string[];
  cache?: boolean | number;
}

export interface WhereClause {
  field: string;
  operator: QueryOperator;
  value: any;
  boolean: 'and' | 'or';
}

export interface OrderByClause {
  field: string;
  direction: 'asc' | 'desc';
}

export interface PaginationResult<T> {
  data: T[];
  total: number;
  page: number;
  perPage: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
}
```

## Error Handling

### Framework Errors
```typescript
export class DebrosFrameworkError extends Error {
  code: string;
  details?: any;
  
  constructor(message: string, code?: string, details?: any) {
    super(message);
    this.name = 'DebrosFrameworkError';
    this.code = code || 'UNKNOWN_ERROR';
    this.details = details;
  }
}

export class ValidationError extends DebrosFrameworkError {
  field: string;
  value: any;
  constraint: string;
}

export class QueryError extends DebrosFrameworkError {
  query: string;
  parameters?: any[];
}

export class RelationshipError extends DebrosFrameworkError {
  modelName: string;
  relationshipName: string;
  relatedModel: string;
}
```

## Performance Optimization

### Query Optimization
**Location**: `src/framework/query/QueryOptimizer.ts`

```typescript
export class QueryOptimizer {
  // Query optimization
  optimizeQuery(query: QueryBuilder<any>): QueryBuilder<any>
  analyzeQueryPerformance(query: QueryBuilder<any>): Promise<QueryAnalysis>
  suggestIndexes(modelName: string): Promise<IndexSuggestion[]>
  
  // Statistics
  getSlowQueries(): Promise<SlowQuery[]>
  getQueryStats(): Promise<QueryStats>
}
```

### LazyLoader Class
**Location**: `src/framework/relationships/LazyLoader.ts`

```typescript
export class LazyLoader {
  // Lazy loading
  async loadOnDemand(model: BaseModel, relationshipName: string): Promise<any>
  async batchLoad(models: BaseModel[], relationshipName: string): Promise<void>
  
  // Configuration
  setBatchSize(size: number): void
  setLoadingStrategy(strategy: 'immediate' | 'batched' | 'deferred'): void
}
```

This technical reference provides the implementation details needed to work effectively with the DebrosFramework codebase.
