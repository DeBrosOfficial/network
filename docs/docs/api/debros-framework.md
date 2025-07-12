---
sidebar_position: 2
---

# DebrosFramework Class

The `DebrosFramework` class is the main entry point for the framework. It handles initialization, configuration, and lifecycle management of all framework components.

## Class Definition

```typescript
class DebrosFramework {
  constructor(config?: Partial<DebrosFrameworkConfig>);

  // Initialization
  async initialize(
    orbitDBService?: any,
    ipfsService?: any,
    overrideConfig?: Partial<DebrosFrameworkConfig>,
  ): Promise<void>;

  // Lifecycle management
  async start(): Promise<void>;
  async stop(): Promise<void>;

  // Component access
  getDatabaseManager(): DatabaseManager;
  getShardManager(): ShardManager;
  getQueryExecutor(): QueryExecutor;
  getRelationshipManager(): RelationshipManager;
  getMigrationManager(): MigrationManager;

  // Configuration
  getConfig(): DebrosFrameworkConfig;
  updateConfig(config: Partial<DebrosFrameworkConfig>): void;

  // Status
  isInitialized(): boolean;
  isRunning(): boolean;
  getStatus(): FrameworkStatus;
}
```

## Constructor

### new DebrosFramework(config?)

Creates a new instance of the DebrosFramework.

**Parameters:**

- `config` (optional): Partial framework configuration

**Example:**

```typescript
import { DebrosFramework } from 'debros-framework';

// Default configuration
const framework = new DebrosFramework();

// Custom configuration
const framework = new DebrosFramework({
  cache: {
    enabled: true,
    maxSize: 5000,
    ttl: 600000, // 10 minutes
  },
  queryOptimization: {
    enabled: true,
    cacheQueries: true,
    parallelExecution: true,
  },
  development: {
    logLevel: 'debug',
    enableMetrics: true,
  },
});
```

## Initialization Methods

### initialize(orbitDBService?, ipfsService?, overrideConfig?)

Initializes the framework with OrbitDB and IPFS services.

**Parameters:**

- `orbitDBService` (optional): OrbitDB service instance
- `ipfsService` (optional): IPFS service instance
- `overrideConfig` (optional): Configuration overrides

**Returns:** `Promise<void>`

**Throws:** `DebrosFrameworkError` if initialization fails

**Example:**

```typescript
import { DebrosFramework } from 'debros-framework';
import { setupOrbitDB, setupIPFS } from './services';

async function initializeFramework() {
  const framework = new DebrosFramework();

  // Setup external services
  const orbitDBService = await setupOrbitDB();
  const ipfsService = await setupIPFS();

  // Initialize framework
  await framework.initialize(orbitDBService, ipfsService, {
    cache: { enabled: true },
    development: { logLevel: 'info' },
  });

  console.log('Framework initialized successfully');
  return framework;
}
```

**With existing services:**

```typescript
async function initializeWithExistingServices() {
  const framework = new DebrosFramework();

  // Use existing services from your application
  const existingOrbitDB = app.orbitDB;
  const existingIPFS = app.ipfs;

  await framework.initialize(existingOrbitDB, existingIPFS);

  return framework;
}
```

**Environment-specific configuration:**

```typescript
async function initializeForEnvironment(env: 'development' | 'production' | 'test') {
  const framework = new DebrosFramework();

  const configs = {
    development: {
      development: { logLevel: 'debug', enableMetrics: true },
      cache: { enabled: true, maxSize: 1000 },
    },
    production: {
      development: { logLevel: 'error', enableMetrics: false },
      cache: { enabled: true, maxSize: 10000 },
      queryOptimization: { enabled: true, parallelExecution: true },
    },
    test: {
      development: { logLevel: 'warn', enableMetrics: false },
      cache: { enabled: false },
    },
  };

  await framework.initialize(await setupOrbitDB(env), await setupIPFS(env), configs[env]);

  return framework;
}
```

## Lifecycle Methods

### start()

Starts the framework and all its components.

**Returns:** `Promise<void>`

**Example:**

```typescript
const framework = new DebrosFramework();
await framework.initialize(orbitDBService, ipfsService);
await framework.start();

console.log('Framework is now running');
```

### stop()

Gracefully stops the framework and cleans up resources.

**Returns:** `Promise<void>`

**Example:**

```typescript
// Graceful shutdown
process.on('SIGINT', async () => {
  console.log('Shutting down framework...');
  await framework.stop();
  console.log('Framework stopped');
  process.exit(0);
});

// Manual stop
await framework.stop();
```

## Component Access Methods

### getDatabaseManager()

Returns the database manager instance.

**Returns:** `DatabaseManager | null` - Database manager instance or null if not initialized

**Throws:** None - This method does not throw errors

**Example:**

```typescript
const databaseManager = framework.getDatabaseManager();

// Get user database
const userDB = await databaseManager.getUserDatabase('user123', 'Post');

// Get global database
const globalDB = await databaseManager.getGlobalDatabase('User');
```

### getShardManager()

Returns the shard manager instance.

**Returns:** `ShardManager`

**Example:**

```typescript
const shardManager = framework.getShardManager();

// Get shard for data
const shard = shardManager.getShardForData('Post', { userId: 'user123' });

// Get all shards for model
const shards = shardManager.getShardsForModel('Post');
```

### getQueryExecutor()

Returns the query executor instance.

**Returns:** `QueryExecutor`

**Example:**

```typescript
const queryExecutor = framework.getQueryExecutor();

// Execute custom query
const results = await queryExecutor.execute(queryBuilder, {
  timeout: 10000,
  useCache: true,
});
```

### getRelationshipManager()

Returns the relationship manager instance.

**Returns:** `RelationshipManager`

**Example:**

```typescript
const relationshipManager = framework.getRelationshipManager();

// Load relationship
const posts = await relationshipManager.loadRelationship(user, 'posts', { eager: true });
```

### getMigrationManager()

Returns the migration manager instance.

**Returns:** `MigrationManager`

**Example:**

```typescript
const migrationManager = framework.getMigrationManager();

// Run pending migrations
await migrationManager.runPendingMigrations();

// Get migration status
const status = migrationManager.getActiveMigrations();
```

## Configuration Methods

### getConfig()

Returns the current framework configuration.

**Returns:** `DebrosFrameworkConfig`

**Example:**

```typescript
const config = framework.getConfig();
console.log('Cache enabled:', config.cache?.enabled);
console.log('Log level:', config.development?.logLevel);
```

### updateConfig(config)

Updates the framework configuration.

**Parameters:**

- `config`: Partial configuration to merge

**Example:**

```typescript
// Update cache settings
framework.updateConfig({
  cache: {
    enabled: true,
    maxSize: 2000,
    ttl: 300000,
  },
});

// Update development settings
framework.updateConfig({
  development: {
    logLevel: 'debug',
  },
});
```

## Status Methods

### isInitialized()

Checks if the framework has been initialized.

**Returns:** `boolean`

**Example:**

```typescript
if (!framework.isInitialized()) {
  await framework.initialize(orbitDBService, ipfsService);
}
```

### isRunning()

Checks if the framework is currently running.

**Returns:** `boolean`

**Example:**

```typescript
if (framework.isRunning()) {
  console.log('Framework is active');
} else {
  await framework.start();
}
```

### getStatus()

Returns detailed framework status information.

**Returns:** `FrameworkStatus`

**Example:**

```typescript
const status = framework.getStatus();

console.log('Framework Status:', {
  initialized: status.initialized,
  running: status.running,
  uptime: status.uptime,
  version: status.version,
  components: status.components,
  metrics: status.metrics,
});
```

## Configuration Interface

### DebrosFrameworkConfig

```typescript
interface DebrosFrameworkConfig {
  // Cache configuration
  cache?: {
    enabled?: boolean; // Enable/disable caching
    maxSize?: number; // Maximum cache entries
    ttl?: number; // Time to live in milliseconds
    cleanupInterval?: number; // Cleanup interval in milliseconds
  };

  // Query optimization
  queryOptimization?: {
    enabled?: boolean; // Enable query optimization
    cacheQueries?: boolean; // Cache query results
    parallelExecution?: boolean; // Execute queries in parallel
    maxConcurrent?: number; // Max concurrent queries
    timeout?: number; // Query timeout in milliseconds
  };

  // Automatic pinning
  automaticPinning?: {
    enabled?: boolean; // Enable automatic pinning
    strategy?: 'popularity' | 'fixed' | 'tiered'; // Pinning strategy
    factor?: number; // Pinning factor
    maxPins?: number; // Maximum pins
    evaluationInterval?: number; // Evaluation interval in milliseconds
  };

  // PubSub configuration
  pubsub?: {
    enabled?: boolean; // Enable PubSub
    bufferSize?: number; // Event buffer size
    maxRetries?: number; // Max retry attempts
    retryDelay?: number; // Retry delay in milliseconds
  };

  // Sharding configuration
  sharding?: {
    defaultStrategy?: 'hash' | 'range' | 'user'; // Default sharding strategy
    defaultCount?: number; // Default shard count
    maxShards?: number; // Maximum shards per model
  };

  // Development settings
  development?: {
    logLevel?: 'debug' | 'info' | 'warn' | 'error'; // Logging level
    enableMetrics?: boolean; // Enable metrics collection
    enableProfiling?: boolean; // Enable performance profiling
    mockMode?: boolean; // Enable mock mode for testing
  };

  // Network configuration
  network?: {
    connectionTimeout?: number; // Connection timeout in milliseconds
    retryAttempts?: number; // Number of retry attempts
    retryDelay?: number; // Delay between retries in milliseconds
  };
}
```

### FrameworkStatus

```typescript
interface FrameworkStatus {
  initialized: boolean;
  running: boolean;
  version: string;
  uptime: number;

  components: {
    databaseManager: ComponentStatus;
    shardManager: ComponentStatus;
    queryExecutor: ComponentStatus;
    relationshipManager: ComponentStatus;
    migrationManager: ComponentStatus;
    pinningManager: ComponentStatus;
    pubsubManager: ComponentStatus;
  };

  metrics?: {
    queriesExecuted: number;
    cacheHits: number;
    cacheMisses: number;
    averageQueryTime: number;
    activeConnections: number;
    memoryUsage: number;
  };
}

interface ComponentStatus {
  status: 'active' | 'inactive' | 'error';
  lastActivity?: number;
  errorCount?: number;
  lastError?: string;
}
```

## Error Handling

### Common Errors

```typescript
try {
  await framework.initialize(orbitDBService, ipfsService);
} catch (error) {
  if (error instanceof DebrosFrameworkError) {
    switch (error.code) {
      case 'INITIALIZATION_FAILED':
        console.error('Framework initialization failed:', error.message);
        break;
      case 'SERVICE_NOT_AVAILABLE':
        console.error('Required service not available:', error.details);
        break;
      case 'CONFIGURATION_ERROR':
        console.error('Configuration error:', error.message);
        break;
      default:
        console.error('Unknown framework error:', error);
    }
  } else {
    console.error('Unexpected error:', error);
  }
}
```

## Complete Example

### Production Application Setup

```typescript
import { DebrosFramework } from 'debros-framework';
import { setupOrbitDB, setupIPFS } from './services';
import { User, Post, Comment } from './models';

class Application {
  private framework: DebrosFramework;

  async initialize() {
    // Create framework with production configuration
    this.framework = new DebrosFramework({
      cache: {
        enabled: true,
        maxSize: 10000,
        ttl: 600000, // 10 minutes
      },
      queryOptimization: {
        enabled: true,
        cacheQueries: true,
        parallelExecution: true,
        maxConcurrent: 20,
        timeout: 30000,
      },
      automaticPinning: {
        enabled: true,
        strategy: 'popularity',
        factor: 3,
        maxPins: 1000,
      },
      development: {
        logLevel: 'info',
        enableMetrics: true,
      },
    });

    // Setup services
    const orbitDBService = await setupOrbitDB();
    const ipfsService = await setupIPFS();

    // Initialize framework
    await this.framework.initialize(orbitDBService, ipfsService);
    await this.framework.start();

    console.log('Application initialized successfully');

    // Log status
    const status = this.framework.getStatus();
    console.log('Framework status:', status);
  }

  async shutdown() {
    if (this.framework) {
      await this.framework.stop();
      console.log('Application shutdown complete');
    }
  }

  getFramework(): DebrosFramework {
    return this.framework;
  }
}

// Usage
const app = new Application();

async function main() {
  try {
    await app.initialize();

    // Your application logic here
    const framework = app.getFramework();

    // Create some data
    const user = await User.create({
      username: 'alice',
      email: 'alice@example.com',
    });

    const post = await Post.create({
      title: 'Hello DebrosFramework',
      content: 'This is my first post!',
      authorId: user.id,
    });

    console.log('Created user and post successfully');
  } catch (error) {
    console.error('Application failed:', error);
  }
}

// Graceful shutdown
process.on('SIGINT', async () => {
  console.log('Received SIGINT, shutting down gracefully...');
  await app.shutdown();
  process.exit(0);
});

process.on('SIGTERM', async () => {
  console.log('Received SIGTERM, shutting down gracefully...');
  await app.shutdown();
  process.exit(0);
});

main().catch(console.error);
```

This comprehensive API reference for the DebrosFramework class covers all public methods, configuration options, and usage patterns for initializing and managing the framework in your applications.
