---
sidebar_position: 2
---

# Database Management

The `DatabaseManager` class in DebrosFramework manages all aspects of database interaction and lifecycle.

## Overview

The DatabaseManager handles:

- Database creation
- Connection pooling
- User-scoped vs global database routing
- Query execution optimization

## Key Features

1. **Connection Management** - Manages database connections and pooling.
2. **Sharding Support** - Integrates with the ShardManager to support data distribution.
3. **Performance** - Caches database instances for efficient access.

## Classes

### DatabaseManager

```typescript
class DatabaseManager {
  constructor(
    private orbitDBService: FrameworkOrbitDBService,
    private shardManager: ShardManager,
    private configManager: ConfigManager
  );
}
```

## Methods

### `getDatabaseForModel`

Finds or creates a suitable database for a given model class, considering user-scope or global-scope.

```typescript
async getDatabaseForModelcT extends BaseModele(
  modelClass: ModelConstructorcTe,
  userId?: string
): PromisecDatabasee
```

### `createUserDatabase`

Creates a user-centric database using sharding strategies.

```typescript
async createUserDatabasecT extends BaseModele(
  userId: string,
  modelClass: ModelConstructorcTe
): PromisecDatabasee
```

## Usage Examples

### Example: User-scoped Database

```typescript
@Model({
  scope: 'user',
  type: 'docstore',
  sharding: { strategy: 'hash', count: 4, key: 'id' }
})
class UserProfile extends BaseModel {
   @Field({ type: 'string', required: true })
  userId: string;

  @Field({ type: 'string' })
  bio: string;
}

// Usage
const userDB = await databaseManager.getDatabaseForModel(UserProfile, 'user123');
```

### Example: Global Database

```typescript
@Model({
  scope: 'global',
  type: 'docstore'
})
class GlobalStats extends BaseModel {
   @Field({ type: 'number' })
  totalUsers: number;

  @Field({ type: 'number' })
  totalInteractions: number;
}

// Usage
const globalDB = await databaseManager.getDatabaseForModel(GlobalStats);
```

## Configuration

### Configuration Options

```typescript
interface DatabaseConfig {
  maxDatabases: number;
  defaultType: StoreType;
  caching: {
    enabled: boolean;
    maxSize: number;
    ttl: number;
  };
}

interface ShardingConfig {
  defaultStrategy: ShardingStrategy;
  defaultCount: number;
}
```

## Related Documents

- [Shard Manager](../api/shard-manager) - Handles data sharding and distribution.
- [Query Executor](../api/query-executor) - Handles query execution and optimization.
