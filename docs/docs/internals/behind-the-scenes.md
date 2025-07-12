# Behind the Scenes: How DebrosFramework Works with OrbitDB

This guide explains what happens under the hood when you use DebrosFramework's high-level abstractions. Understanding these internals will help you debug issues, optimize performance, and better understand the framework's architecture.

## Overview: From Models to OrbitDB

DebrosFramework provides ORM-like abstractions over OrbitDB's peer-to-peer databases. When you define models, create relationships, and run migrations, the framework translates these operations into OrbitDB database operations.

```
Your Code → DebrosFramework → OrbitDB → IPFS
```

## Model Creation and Database Mapping

### What Happens When You Define a Model

When you define a model like this:

```typescript
@Model({
  scope: 'global',
  type: 'docstore',
  sharding: { strategy: 'hash', count: 4, key: 'id' }
})
export class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;
  
  @Field({ type: 'string', required: true })
  email: string;
}
```

**Behind the scenes:**

1. **Model Registration**: The `@Model` decorator registers the model in `ModelRegistry`
2. **Field Configuration**: `@Field` decorators are stored in a static `fields` Map on the model class
3. **Database Planning**: The framework determines what OrbitDB databases need to be created

### OrbitDB Database Creation

For the `User` model above, DebrosFramework creates:

```typescript
// Global scope, no sharding
const userDB = await orbitdb.open('global-user', 'docstore', {
  accessController: {
    type: 'orbitdb',
    write: ['*'] // Or specific access rules
  }
});

// With sharding (strategy: 'hash', count: 4)
const userShard0 = await orbitdb.open('global-user-shard-0', 'docstore');
const userShard1 = await orbitdb.open('global-user-shard-1', 'docstore');
const userShard2 = await orbitdb.open('global-user-shard-2', 'docstore');
const userShard3 = await orbitdb.open('global-user-shard-3', 'docstore');
```

### Database Naming Convention

```typescript
// Global models
`global-${modelName.toLowerCase()}`

// User-scoped models  
`${userId}-${modelName.toLowerCase()}`

// Sharded models
`${scope}-${modelName.toLowerCase()}-shard-${shardIndex}`

// System databases
`${appName}-bootstrap`           // Shard coordination
`${appName}-directory-shard-${i}` // User directory
`${userId}-mappings`             // User's database mappings
```

## CRUD Operations Under the Hood

### Creating a Record

When you call:

```typescript
const user = await User.create({
  username: 'alice',
  email: 'alice@example.com'
});
```

**Behind the scenes:**

1. **Validation**: Field validators run on the data
2. **Transformation**: Field transformers process the data
3. **ID Generation**: A unique ID is generated (typically UUID)
4. **Lifecycle Hooks**: `@BeforeCreate` hooks execute
5. **Shard Selection**: If sharded, determine which shard to use
6. **OrbitDB Operation**: Data is stored in the appropriate database

```typescript
// What actually happens in OrbitDB
const database = await this.getShardForKey(user.id); // or getGlobalDatabase()
const docHash = await database.put({
  _id: user.id,
  username: 'alice',
  email: 'alice@example.com',
  _model: 'User',
  _createdAt: Date.now(),
  _updatedAt: Date.now()
});
```

### Reading Records

When you call:

```typescript
const user = await User.findById('user123');
```

**Behind the scenes:**

1. **Shard Resolution**: Determine which shard contains the record
2. **Database Query**: Query the appropriate OrbitDB database
3. **Data Hydration**: Convert raw OrbitDB data back to model instance
4. **Field Processing**: Apply any field transformations

```typescript
// OrbitDB operations
const shard = this.getShardForKey('user123');
const doc = await shard.get('user123');

if (doc) {
  // Convert to model instance
  const user = new User(doc);
  user._isNew = false;
  return user;
}
```

### Query Operations

When you call:

```typescript
const users = await User.query().where('isActive', true).find();
```

**Behind the scenes:**

1. **Query Planning**: Determine which databases to search
2. **Parallel Queries**: Query all relevant shards simultaneously
3. **Result Aggregation**: Combine results from multiple databases
4. **Filtering**: Apply where conditions to the aggregated results

```typescript
// Actual implementation
const shards = this.getAllShardsForModel('User');
const results = await Promise.all(
  shards.map(async (shard) => {
    const docs = shard.iterator().collect();
    return docs.filter(doc => doc.isActive === true);
  })
);

const allResults = results.flat();
return allResults.map(doc => new User(doc));
```

## Relationships and Cross-Database Operations

### How Relationships Work

When you define relationships:

```typescript
@Model({ scope: 'global', type: 'docstore' })
export class User extends BaseModel {
  @HasMany(() => Post, 'authorId')
  posts: Post[];
}

@Model({ scope: 'user', type: 'docstore' })
export class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  authorId: string;
  
  @BelongsTo(() => User, 'authorId')
  author: User;
}
```

**Behind the scenes:**

Relationships are stored as foreign keys and resolved through cross-database queries:

```typescript
// User in global database
{
  _id: 'user123',
  username: 'alice',
  _model: 'User'
}

// Posts in user-specific database
{
  _id: 'post456',
  title: 'My Post',
  authorId: 'user123', // Foreign key reference
  _model: 'Post'
}
```

### Relationship Loading

When you load relationships:

```typescript
const user = await User.findById('user123', { with: ['posts'] });
```

**Behind the scenes:**

1. **Primary Query**: Load the user from global database
2. **Relationship Resolution**: Identify related models and their databases
3. **Cross-Database Query**: Query user-specific databases for posts
4. **Data Assembly**: Attach loaded relationships to the main model

```typescript
// Implementation
const user = await globalUserDB.get('user123');
const userDB = await this.getUserDatabase('user123', 'Post');
const posts = await userDB.iterator().collect()
  .filter(doc => doc.authorId === 'user123');

user.posts = posts.map(doc => new Post(doc));
```

## Sharding Implementation

### Hash Sharding

When you configure hash sharding:

```typescript
@Model({
  sharding: { strategy: 'hash', count: 4, key: 'id' }
})
export class Post extends BaseModel {
  // ...
}
```

**Behind the scenes:**

```typescript
// Shard selection algorithm
function getShardForKey(key: string, shardCount: number): number {
  const hash = crypto.createHash('sha256').update(key).digest('hex');
  const hashInt = parseInt(hash.substring(0, 8), 16);
  return hashInt % shardCount;
}

// Creating a post
const post = new Post({ title: 'Hello' });
const shardIndex = getShardForKey(post.id, 4); // Returns 0-3
const database = await orbitdb.open(`user123-post-shard-${shardIndex}`, 'docstore');
await database.put(post.toJSON());
```

### User Sharding

For user-scoped models:

```typescript
@Model({
  scope: 'user',
  sharding: { strategy: 'user', count: 2, key: 'authorId' }
})
export class Post extends BaseModel {
  // ...
}
```

**Behind the scenes:**

```typescript
// Each user gets their own set of sharded databases
const userId = 'user123';
const shardIndex = getShardForKey(post.id, 2);
const database = await orbitdb.open(`${userId}-post-shard-${shardIndex}`, 'docstore');
```

## Migration System Implementation

### What Migrations Actually Do

Since OrbitDB doesn't have traditional schema migrations, DebrosFramework implements them differently:

```typescript
const migration = createMigration('add_user_bio', '1.1.0')
  .addField('User', 'bio', { type: 'string', required: false })
  .transformData('User', (user) => ({
    ...user,
    bio: user.bio || 'No bio provided'
  }))
  .build();
```

**Behind the scenes:**

1. **Migration Tracking**: Store migration state in a special database
2. **Data Transformation**: Read all records, transform them, and write back
3. **Schema Updates**: Update model field configurations
4. **Validation**: Ensure all data conforms to new schema

```typescript
// Migration implementation
async function runMigration(migration: Migration) {
  // Track migration state
  const migrationDB = await orbitdb.open('migrations', 'docstore');
  
  // For each target model
  for (const modelName of migration.targetModels) {
    const databases = await this.getAllDatabasesForModel(modelName);
    
    for (const database of databases) {
      // Read all documents
      const docs = await database.iterator().collect();
      
      // Transform each document
      for (const doc of docs) {
        const transformed = migration.transform(doc);
        await database.put(transformed);
      }
    }
  }
  
  // Record migration as completed
  await migrationDB.put({
    _id: migration.id,
    version: migration.version,
    appliedAt: Date.now(),
    status: 'completed'
  });
}
```

## User Directory and Database Discovery

### How User Databases Are Discovered

DebrosFramework maintains a distributed directory system:

```typescript
// Bootstrap database (shared across network)
const bootstrap = await orbitdb.open('myapp-bootstrap', 'keyvalue');

// Directory shards for user mappings
const dirShard0 = await orbitdb.open('myapp-directory-shard-0', 'keyvalue');
const dirShard1 = await orbitdb.open('myapp-directory-shard-1', 'keyvalue');
const dirShard2 = await orbitdb.open('myapp-directory-shard-2', 'keyvalue');
const dirShard3 = await orbitdb.open('myapp-directory-shard-3', 'keyvalue');
```

**Behind the scenes:**

When a user creates their first record:

1. **User Database Creation**: Create user-specific databases
2. **Mapping Storage**: Store database addresses in user's mappings database
3. **Directory Registration**: Register user's mappings database in global directory
4. **Shard Distribution**: Use consistent hashing to distribute users across directory shards

```typescript
// Creating user databases
async function createUserDatabases(userId: string) {
  // Create mappings database
  const mappingsDB = await orbitdb.open(`${userId}-mappings`, 'keyvalue');
  
  // Create model databases
  const postDB = await orbitdb.open(`${userId}-post`, 'docstore');
  const commentDB = await orbitdb.open(`${userId}-comment`, 'docstore');
  
  // Store mappings
  await mappingsDB.set('mappings', {
    postDB: postDB.address.toString(),
    commentDB: commentDB.address.toString()
  });
  
  // Register in global directory
  const dirShard = this.getDirectoryShardForUser(userId);
  await dirShard.set(userId, mappingsDB.address.toString());
  
  return { mappingsDB, postDB, commentDB };
}
```

## Caching and Performance Optimization

### Query Caching

When you enable query caching:

```typescript
const framework = new DebrosFramework({
  features: { queryCache: true }
});
```

**Behind the scenes:**

```typescript
// Cache key generation
function generateCacheKey(query: QueryBuilder): string {
  return crypto
    .createHash('md5')
    .update(JSON.stringify({
      model: query.modelName,
      where: query.whereConditions,
      orderBy: query.orderByConditions,
      limit: query.limitValue
    }))
    .digest('hex');
}

// Query execution with caching
async function executeQuery(query: QueryBuilder) {
  const cacheKey = generateCacheKey(query);
  
  // Check cache first
  const cached = await this.queryCache.get(cacheKey);
  if (cached) {
    return cached.map(data => new query.ModelClass(data));
  }
  
  // Execute query
  const results = await this.executeQueryOnDatabases(query);
  
  // Cache results
  await this.queryCache.set(cacheKey, results.map(r => r.toJSON()), 300000);
  
  return results;
}
```

### Relationship Caching

```typescript
// Relationship cache
const relationshipCache = new Map();

async function loadRelationship(model: BaseModel, relationshipName: string) {
  const cacheKey = `${model.constructor.name}:${model.id}:${relationshipName}`;
  
  if (relationshipCache.has(cacheKey)) {
    return relationshipCache.get(cacheKey);
  }
  
  // Load relationship from databases
  const related = await this.queryRelatedModels(model, relationshipName);
  
  // Cache for 5 minutes
  relationshipCache.set(cacheKey, related);
  setTimeout(() => relationshipCache.delete(cacheKey), 300000);
  
  return related;
}
```

## Automatic Pinning Strategy

### How Pinning Works

When you enable automatic pinning:

```typescript
const framework = new DebrosFramework({
  features: { automaticPinning: true }
});
```

**Behind the scenes:**

```typescript
// Pinning manager tracks access patterns
class PinningManager {
  private accessCount = new Map<string, number>();
  private pinned = new Set<string>();
  
  async trackAccess(address: string) {
    const count = this.accessCount.get(address) || 0;
    this.accessCount.set(address, count + 1);
    
    // Pin popular content
    if (count > 10 && !this.pinned.has(address)) {
      await this.ipfs.pin.add(address);
      this.pinned.add(address);
      console.log(`📌 Pinned popular content: ${address}`);
    }
  }
  
  async evaluatePinning() {
    // Run every hour
    setInterval(() => {
      this.unpinStaleContent();
      this.pinPopularContent();
    }, 3600000);
  }
}
```

## PubSub and Real-time Updates

### Event Publishing

When models change:

```typescript
// After successful database operation
async function publishModelEvent(event: ModelEvent) {
  const topic = `debros:${event.modelName}:${event.type}`;
  await this.pubsub.publish(topic, JSON.stringify(event));
}

// Model creation
await User.create({ username: 'alice' });
// Publishes to: "debros:User:create"

// Model update  
await user.save();
// Publishes to: "debros:User:update"
```

### Event Subscription

```typescript
// Subscribe to model events
pubsub.subscribe('debros:User:*', (message) => {
  const event = JSON.parse(message);
  console.log(`User ${event.type}: ${event.modelId}`);
  
  // Invalidate related caches
  this.invalidateUserCache(event.modelId);
});
```

## Database Synchronization

### How Peers Sync Data

OrbitDB handles the low-level synchronization, but DebrosFramework optimizes it:

```typescript
// Replication configuration
const database = await orbitdb.open('global-user', 'docstore', {
  replicate: true,
  sync: true,
  accessController: {
    type: 'orbitdb',
    write: ['*']
  }
});

// Custom sync logic
database.events.on('peer', (peer) => {
  console.log(`👥 Peer connected: ${peer}`);
});

database.events.on('replicated', (address) => {
  console.log(`🔄 Replicated data from: ${address}`);
  // Invalidate caches, trigger UI updates
  this.invalidateCaches();
});
```

## Error Handling and Recovery

### Database Connection Failures

```typescript
// Retry logic for database operations
async function withRetry<T>(operation: () => Promise<T>, maxRetries = 3): Promise<T> {
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await operation();
    } catch (error) {
      if (attempt === maxRetries) throw error;
      
      const delay = Math.pow(2, attempt) * 1000; // Exponential backoff
      await new Promise(resolve => setTimeout(resolve, delay));
    }
  }
  throw new Error('Max retries exceeded');
}

// Usage
const user = await withRetry(() => User.findById('user123'));
```

### Data Corruption Recovery

```typescript
// Validate data integrity
async function validateDatabase(database: Database) {
  const docs = await database.iterator().collect();
  
  for (const doc of docs) {
    try {
      // Validate against model schema
      const model = new modelClass(doc);
      await model.validate();
    } catch (error) {
      console.warn(`⚠️ Invalid document found: ${doc._id}`, error);
      // Mark for repair or removal
    }
  }
}
```

## Performance Monitoring

### Internal Metrics

```typescript
// Performance tracking
class MetricsCollector {
  private queryTimes = new Map<string, number[]>();
  private operationCounts = new Map<string, number>();
  
  recordQueryTime(operation: string, duration: number) {
    const times = this.queryTimes.get(operation) || [];
    times.push(duration);
    this.queryTimes.set(operation, times.slice(-100)); // Keep last 100
  }
  
  getAverageQueryTime(operation: string): number {
    const times = this.queryTimes.get(operation) || [];
    return times.reduce((a, b) => a + b, 0) / times.length;
  }
  
  getMetrics() {
    return {
      queryTimes: Object.fromEntries(this.queryTimes),
      operationCounts: Object.fromEntries(this.operationCounts),
      cacheHitRates: this.getCacheHitRates(),
      databaseSizes: this.getDatabaseSizes()
    };
  }
}
```

## Debugging and Introspection

### Debug Mode

```typescript
const framework = new DebrosFramework({
  monitoring: { logLevel: 'debug' }
});

// Enables detailed logging
// 🔍 Query: User.findById(user123) -> shard-2
// 📊 Cache miss: query-hash-abc123
// 🔄 Database operation: put user123 -> completed in 45ms
// 📡 PubSub: published User:create event
```

### Database Inspection

```typescript
// Access raw OrbitDB databases for debugging
const framework = getFramework();
const databaseManager = framework.getDatabaseManager();

// List all databases
const databases = await databaseManager.getAllDatabases();
console.log('Databases:', Array.from(databases.keys()));

// Inspect database contents
const userDB = await databaseManager.getGlobalDatabase('User');
const docs = await userDB.iterator().collect();
console.log('User documents:', docs);

// Check database addresses
console.log('Database address:', userDB.address.toString());
```

This behind-the-scenes documentation helps developers understand the complexity that DebrosFramework abstracts away while providing the transparency needed for debugging and optimization.
