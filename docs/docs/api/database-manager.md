---
sidebar_position: 3
---

# DatabaseManager

The `DatabaseManager` class is responsible for managing database connections, lifecycle, and operations for both user-scoped and global databases in DebrosFramework.

## Overview

The DatabaseManager provides a unified interface for database operations across different database types and scopes. It handles:

- Database creation and initialization
- Connection management and pooling
- User-scoped vs global database routing
- Database lifecycle management
- Performance optimization and caching

## Class Definition

```typescript
class DatabaseManager {
  constructor(
    private orbitDBService: FrameworkOrbitDBService,
    private shardManager: ShardManager,
    private configManager: ConfigManager
  );
}
```

## Core Methods

### Database Creation

#### `getDatabaseForModel<T>(modelClass, userId?)`

Gets or creates a database for a specific model.

```typescript
async getDatabaseForModel<T extends BaseModel>(
  modelClass: ModelConstructor<T>,
  userId?: string
): Promise<Database>
```

**Parameters:**
- `modelClass` - The model class to get database for
- `userId` - User ID for user-scoped databases

**Returns:** Promise resolving to the database instance

**Example:**
```typescript
const userDB = await databaseManager.getDatabaseForModel(User, 'user123');
const globalDB = await databaseManager.getDatabaseForModel(GlobalConfig);
```

#### `createDatabase(name, type, options)`

Creates a new database with specified configuration.

```typescript
async createDatabase(
  name: string,
  type: StoreType,
  options: DatabaseOptions
): Promise<Database>
```

**Parameters:**
- `name` - Database name
- `type` - Database type ('docstore', 'eventlog', 'keyvalue', 'counter', 'feed')
- `options` - Database configuration options

### Database Management

#### `ensureDatabaseExists(name, type)`

Ensures a database exists, creating it if necessary.

```typescript
async ensureDatabaseExists(
  name: string,
  type: StoreType
): Promise<Database>
```

#### `closeDatabaseForModel(modelClass, userId?)`

Closes a database for a specific model.

```typescript
async closeDatabaseForModel<T extends BaseModel>(
  modelClass: ModelConstructor<T>,
  userId?: string
): Promise<void>
```

#### `closeAllDatabases()`

Closes all open databases.

```typescript
async closeAllDatabases(): Promise<void>
```

### User Management

#### `createUserDatabase(userId, modelClass)`

Creates a user-specific database.

```typescript
async createUserDatabase<T extends BaseModel>(
  userId: string,
  modelClass: ModelConstructor<T>
): Promise<Database>
```

#### `getUserDatabases(userId)`

Gets all databases for a specific user.

```typescript
async getUserDatabases(userId: string): Promise<Database[]>
```

## Database Types

### Store Types

```typescript
type StoreType = 'docstore' | 'eventlog' | 'keyvalue' | 'counter' | 'feed';
```

- **docstore** - Document-based storage for structured data
- **eventlog** - Append-only log for events and transactions
- **keyvalue** - Key-value storage for simple data
- **counter** - Conflict-free replicated counters
- **feed** - Sequential feed of items

### Database Options

```typescript
interface DatabaseOptions {
  scope: 'user' | 'global';
  sharding?: ShardingConfig;
  replication?: ReplicationConfig;
  indexing?: IndexingConfig;
  caching?: CachingConfig;
}
```

## Database Scoping

### User-Scoped Databases

User-scoped databases are isolated per user:

```typescript
// User-scoped model
@Model({
  scope: 'user',
  type: 'docstore'
})
class UserPost extends BaseModel {
  // Model definition
}

// Each user gets their own database
const aliceDB = await databaseManager.getDatabaseForModel(UserPost, 'alice');
const bobDB = await databaseManager.getDatabaseForModel(UserPost, 'bob');
```

### Global Databases

Global databases are shared across all users:

```typescript
// Global model
@Model({
  scope: 'global',
  type: 'docstore'
})
class GlobalConfig extends BaseModel {
  // Model definition
}

// Single shared database
const globalDB = await databaseManager.getDatabaseForModel(GlobalConfig);
```

## Sharding Integration

The DatabaseManager integrates with the ShardManager for data distribution:

```typescript
// Sharded model
@Model({
  scope: 'global',
  type: 'docstore',
  sharding: {
    strategy: 'hash',
    count: 4,
    key: 'id'
  }
})
class ShardedModel extends BaseModel {
  // Model definition
}

// DatabaseManager automatically routes to correct shard
const database = await databaseManager.getDatabaseForModel(ShardedModel);
```

## Performance Features

### Connection Pooling

The DatabaseManager maintains a pool of database connections:

```typescript
// Connection pool configuration
const poolConfig = {
  maxConnections: 50,
  idleTimeout: 30000,
  connectionTimeout: 5000
};
```

### Caching

Database instances are cached for performance:

```typescript
// Cached database access
const db1 = await databaseManager.getDatabaseForModel(User, 'user123');
const db2 = await databaseManager.getDatabaseForModel(User, 'user123');
// db1 and db2 are the same instance
```

## Error Handling

### Database Errors

```typescript
// Database creation error
try {
  const db = await databaseManager.getDatabaseForModel(User, 'user123');
} catch (error) {
  if (error instanceof DatabaseCreationError) {
    console.error('Failed to create database:', error.message);
  }
}
```

### Connection Errors

```typescript
// Connection error handling
try {
  await databaseManager.ensureDatabaseExists('test', 'docstore');
} catch (error) {
  if (error instanceof DatabaseConnectionError) {
    console.error('Database connection failed:', error.message);
  }
}
```

## Configuration

### Database Configuration

```typescript
interface DatabaseConfig {
  maxDatabases: number;
  defaultType: StoreType;
  caching: {
    enabled: boolean;
    maxSize: number;
    ttl: number;
  };
  sharding: {
    defaultStrategy: ShardingStrategy;
    defaultCount: number;
  };
}
```

### Environment Configuration

```typescript
// Configure database behavior
const databaseManager = new DatabaseManager(
  orbitDBService,
  shardManager,
  configManager
);

// Set configuration
await configManager.set('database', {
  maxDatabases: 100,
  defaultType: 'docstore',
  caching: {
    enabled: true,
    maxSize: 1000,
    ttl: 300000
  }
});
```

## Monitoring

### Database Metrics

```typescript
// Get database metrics
const metrics = await databaseManager.getMetrics();

console.log('Database Statistics:', {
  totalDatabases: metrics.totalDatabases,
  userDatabases: metrics.userDatabases,
  globalDatabases: metrics.globalDatabases,
  memoryUsage: metrics.memoryUsage
});
```

### Performance Monitoring

```typescript
// Monitor database performance
databaseManager.on('databaseCreated', (event) => {
  console.log('Database created:', event.name, event.type);
});

databaseManager.on('databaseClosed', (event) => {
  console.log('Database closed:', event.name);
});
```

## Best Practices

### Resource Management

```typescript
// Always close databases when done
try {
  const db = await databaseManager.getDatabaseForModel(User, 'user123');
  // Use database
} finally {
  await databaseManager.closeDatabaseForModel(User, 'user123');
}
```

### Error Handling

```typescript
// Proper error handling
async function safelyGetDatabase<T extends BaseModel>(
  modelClass: ModelConstructor<T>,
  userId?: string
): Promise<Database | null> {
  try {
    return await databaseManager.getDatabaseForModel(modelClass, userId);
  } catch (error) {
    console.error('Database access failed:', error);
    return null;
  }
}
```

### Performance Optimization

```typescript
// Batch database operations
const databases = await Promise.all([
  databaseManager.getDatabaseForModel(User, 'user1'),
  databaseManager.getDatabaseForModel(User, 'user2'),
  databaseManager.getDatabaseForModel(User, 'user3')
]);
```

## Related Classes

- [`ShardManager`](./shard-manager) - Data sharding and distribution
- [`DebrosFramework`](./debros-framework) - Main framework class
- [`BaseModel`](./base-model) - Base model class
- [`QueryExecutor`](./query-executor) - Query execution

## See Also

- [Database Management Guide](../core-concepts/database-management)
- [Architecture Overview](../core-concepts/architecture)
- [Performance Optimization](../advanced/performance)
