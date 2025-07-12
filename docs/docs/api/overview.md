---
sidebar_position: 1
---

# API Reference Overview

The DebrosFramework API provides a comprehensive set of classes, methods, and interfaces for building decentralized applications. This reference covers all public APIs, their parameters, return types, and usage examples.

## Core Framework Classes

### Primary Classes

| Class                                           | Status | Description                  | Key Features                         |
| ----------------------------------------------- | ------ | ---------------------------- | ------------------------------------ |
| [`DebrosFramework`](./debros-framework)         | ✅ Stable | Main framework class         | Initialization, lifecycle management |
| [`BaseModel`](./base-model)                     | ✅ Stable | Abstract base for all models | CRUD operations, validation, hooks   |
| [`DatabaseManager`](./database-manager)         | ✅ Stable | Database management          | User/global databases, lifecycle     |
| [`ShardManager`](./shard-manager)               | ✅ Stable | Data sharding                | Distribution strategies, routing     |
| [`QueryBuilder`](./query-builder)               | 🚧 Partial | Query construction           | Basic queries, advanced features in dev |
| [`QueryExecutor`](./query-executor)             | 🚧 Partial | Query execution              | Basic execution, optimization in dev |
| [`RelationshipManager`](./relationship-manager) | 🚧 Partial | Relationship handling        | Basic loading, full features in dev  |
| [`MigrationManager`](./migration-manager)       | ✅ Stable | Schema migrations            | Version control, rollbacks           |
| [`MigrationBuilder`](./migration-builder)       | ✅ Stable | Migration creation           | Fluent API, validation               |

### Utility Classes

| Class               | Description              | Use Case                       |
| ------------------- | ------------------------ | ------------------------------ |
| `ModelRegistry`     | Model registration       | Framework initialization       |
| `ConfigManager`     | Configuration management | Environment-specific settings  |
| `PinningManager`    | Automatic pinning        | Data availability optimization |
| `PubSubManager`     | Event publishing         | Real-time features             |
| `QueryCache`        | Query result caching     | Performance optimization       |
| `QueryOptimizer`    | Query optimization       | Automatic performance tuning   |
| `RelationshipCache` | Relationship caching     | Relationship performance       |
| `LazyLoader`        | Lazy loading             | On-demand data loading         |

## Decorators

### Model Decorators

| Decorator                      | Purpose                    | Usage              |
| ------------------------------ | -------------------------- | ------------------ |
| [`@Model`](./decorators/model) | Define model configuration | Class decorator    |
| [`@Field`](./decorators/field) | Define field properties    | Property decorator |

### Relationship Decorators

| Decorator                                              | Relationship Type | Usage              |
| ------------------------------------------------------ | ----------------- | ------------------ |
| [`@BelongsTo`](./decorators/relationships#belongsto)   | Many-to-one       | Property decorator |
| [`@HasMany`](./decorators/relationships#hasmany)       | One-to-many       | Property decorator |
| [`@HasOne`](./decorators/relationships#hasone)         | One-to-one        | Property decorator |
| [`@ManyToMany`](./decorators/relationships#manytomany) | Many-to-many      | Property decorator |

### Hook Decorators

| Decorator                                          | Trigger                     | Usage            |
| -------------------------------------------------- | --------------------------- | ---------------- |
| [`@BeforeCreate`](./decorators/hooks#beforecreate) | Before creating record      | Method decorator |
| [`@AfterCreate`](./decorators/hooks#aftercreate)   | After creating record       | Method decorator |
| [`@BeforeUpdate`](./decorators/hooks#beforeupdate) | Before updating record      | Method decorator |
| [`@AfterUpdate`](./decorators/hooks#afterupdate)   | After updating record       | Method decorator |
| [`@BeforeDelete`](./decorators/hooks#beforedelete) | Before deleting record      | Method decorator |
| [`@AfterDelete`](./decorators/hooks#afterdelete)   | After deleting record       | Method decorator |
| [`@BeforeSave`](./decorators/hooks#beforesave)     | Before save (create/update) | Method decorator |
| [`@AfterSave`](./decorators/hooks#aftersave)       | After save (create/update)  | Method decorator |

## Type Definitions

### Core Types

```typescript
// Model configuration
interface ModelConfig {
  scope: 'user' | 'global';
  type: StoreType;
  sharding?: ShardingConfig;
  pinning?: PinningConfig;
  pubsub?: PubSubConfig;
  validation?: ValidationConfig;
}

// Field configuration
interface FieldConfig {
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

// Query types
interface QueryOptions {
  where?: WhereClause[];
  orderBy?: OrderByClause[];
  limit?: number;
  offset?: number;
  with?: string[];
  cache?: boolean | number;
}
```

### Enum Types

```typescript
// Store types
type StoreType = 'docstore' | 'eventlog' | 'keyvalue' | 'counter' | 'feed';

// Field types
type FieldType = 'string' | 'number' | 'boolean' | 'array' | 'object' | 'date';

// Sharding strategies
type ShardingStrategy = 'hash' | 'range' | 'user';

// Query operators
type QueryOperator =
  | 'eq'
  | 'ne'
  | 'gt'
  | 'gte'
  | 'lt'
  | 'lte'
  | 'in'
  | 'not in'
  | 'like'
  | 'regex'
  | 'is null'
  | 'is not null'
  | 'includes'
  | 'includes any'
  | 'includes all';
```

## Configuration Interfaces

### Framework Configuration

```typescript
interface DebrosFrameworkConfig {
  // Cache configuration
  cache?: {
    enabled?: boolean;
    maxSize?: number;
    ttl?: number;
  };

  // Query optimization
  queryOptimization?: {
    enabled?: boolean;
    cacheQueries?: boolean;
    parallelExecution?: boolean;
  };

  // Automatic features
  automaticPinning?: {
    enabled?: boolean;
    strategy?: 'popularity' | 'fixed' | 'tiered';
  };

  // PubSub configuration
  pubsub?: {
    enabled?: boolean;
    bufferSize?: number;
  };

  // Development settings
  development?: {
    logLevel?: 'debug' | 'info' | 'warn' | 'error';
    enableMetrics?: boolean;
  };
}
```

### Sharding Configuration

```typescript
interface ShardingConfig {
  strategy: ShardingStrategy;
  count: number;
  key: string;
  ranges?: ShardRange[];
}

interface ShardRange {
  min: any;
  max: any;
  shard: number;
}
```

### Pinning Configuration

```typescript
interface PinningConfig {
  strategy: 'fixed' | 'popularity' | 'tiered';
  factor?: number;
  maxPins?: number;
  ttl?: number;
}
```

## Error Types

### Framework Errors

```typescript
class DebrosFrameworkError extends Error {
  code: string;
  details?: any;
}

class ValidationError extends DebrosFrameworkError {
  field: string;
  value: any;
  constraint: string;
}

class QueryError extends DebrosFrameworkError {
  query: string;
  parameters?: any[];
}

class RelationshipError extends DebrosFrameworkError {
  modelName: string;
  relationshipName: string;
  relatedModel: string;
}
```

## Response Types

### Query Results

```typescript
interface QueryResult<T> {
  data: T[];
  total: number;
  page?: number;
  perPage?: number;
  totalPages?: number;
  hasMore?: boolean;
}

interface PaginationInfo {
  page: number;
  perPage: number;
  total: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
}
```

### Operation Results

```typescript
interface CreateResult<T> {
  model: T;
  created: boolean;
  errors?: ValidationError[];
}

interface UpdateResult<T> {
  model: T;
  updated: boolean;
  changes: string[];
  errors?: ValidationError[];
}

interface DeleteResult {
  deleted: boolean;
  id: string;
}
```

## Event Types

### Model Events

```typescript
interface ModelEvent {
  type: 'create' | 'update' | 'delete';
  modelName: string;
  modelId: string;
  data?: any;
  changes?: string[];
  timestamp: number;
  userId?: string;
}

interface RelationshipEvent {
  type: 'attach' | 'detach';
  modelName: string;
  modelId: string;
  relationshipName: string;
  relatedModelName: string;
  relatedModelId: string;
  timestamp: number;
}
```

### Framework Events

```typescript
interface FrameworkEvent {
  type: 'initialized' | 'stopped' | 'error';
  message?: string;
  error?: Error;
  timestamp: number;
}

interface DatabaseEvent {
  type: 'created' | 'opened' | 'closed';
  databaseName: string;
  scope: 'user' | 'global';
  userId?: string;
  timestamp: number;
}
```

## Migration Types

### Migration Configuration

```typescript
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
  createdAt: number;
}

interface MigrationOperation {
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
  transformer?: (data: any) => any;
  customOperation?: (context: MigrationContext) => Promise<void>;
}
```

## Constants

### Default Values

```typescript
const DEFAULT_CONFIG = {
  CACHE_SIZE: 1000,
  CACHE_TTL: 300000, // 5 minutes
  QUERY_TIMEOUT: 30000, // 30 seconds
  MAX_CONCURRENT_QUERIES: 10,
  DEFAULT_PAGE_SIZE: 20,
  MAX_PAGE_SIZE: 1000,
  SHARD_COUNT: 4,
  PIN_FACTOR: 2,
};
```

### Status Codes

```typescript
enum StatusCodes {
  SUCCESS = 200,
  CREATED = 201,
  NO_CONTENT = 204,
  BAD_REQUEST = 400,
  NOT_FOUND = 404,
  VALIDATION_ERROR = 422,
  INTERNAL_ERROR = 500,
}
```

## Utility Functions

### Helper Functions

```typescript
// Model utilities
function getModelConfig(modelClass: typeof BaseModel): ModelConfig;
function getFieldConfig(modelClass: typeof BaseModel, fieldName: string): FieldConfig;
function getRelationshipConfig(modelClass: typeof BaseModel): RelationshipConfig[];

// Query utilities
function buildWhereClause(field: string, operator: QueryOperator, value: any): WhereClause;
function optimizeQuery(query: QueryBuilder): QueryBuilder;
function cacheKey(query: QueryBuilder): string;

// Validation utilities
function validateFieldValue(value: any, config: FieldConfig): ValidationResult;
function sanitizeInput(value: any): any;
function normalizeEmail(email: string): string;
```

## Version Information

Current API version: **1.0.0**

### Version Compatibility

| Framework Version | API Version | Breaking Changes            |
| ----------------- | ----------- | --------------------------- |
| 1.0.x             | 1.0.0       | None                        |
| 1.1.x             | 1.0.0       | None (backwards compatible) |

### Deprecation Policy

- Deprecated APIs are marked with `@deprecated` tags
- Deprecated features are supported for at least 2 minor versions
- Breaking changes only occur in major version updates
- Migration guides are provided for breaking changes

## Getting Help

### Documentation

- **[Getting Started Guide](../getting-started)** - Basic setup and usage
- **[Core Concepts](../core-concepts/architecture)** - Framework architecture
- **[Examples](../examples/basic-usage)** - Practical examples

### Support

- **GitHub Issues** - Bug reports and feature requests
- **Discord Community** - Real-time help and discussion
- **Stack Overflow** - Tagged questions with `debros-framework`

### Contributing

- **[Contributing Guide](https://github.com/debros/network/blob/main/CONTRIBUTING.md)** - How to contribute
- **[API Design Guidelines](https://github.com/debros/network/blob/main/API_DESIGN.md)** - API design principles
- **[Development Setup](https://github.com/debros/network/blob/main/DEVELOPMENT.md)** - Local development setup

This API reference provides comprehensive documentation for all public interfaces in DebrosFramework. For detailed information about specific classes and methods, explore the individual API documentation pages.
