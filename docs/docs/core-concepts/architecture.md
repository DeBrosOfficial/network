---
sidebar_position: 1
---

# Architecture Overview

DebrosFramework is designed with a modular architecture that provides powerful abstractions over OrbitDB and IPFS while maintaining scalability and performance. This guide explains how the framework components work together.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Your Application                         │
├─────────────────────────────────────────────────────────────┤
│                  DebrosFramework                            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │   Models    │ │   Queries   │ │ Migrations  │            │
│  │ & Decorators│ │   System    │ │   System    │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │  Database   │ │    Shard    │ │ Relationship│            │
│  │  Manager    │ │   Manager   │ │   Manager   │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │  Pinning    │ │   PubSub    │ │    Cache    │            │
│  │  Manager    │ │   Manager   │ │   System    │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
├─────────────────────────────────────────────────────────────┤
│                   OrbitDB Layer                             │
├─────────────────────────────────────────────────────────────┤
│                    IPFS Layer                               │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Models & Decorators Layer

The foundation of DebrosFramework is the model layer, which provides:

- **BaseModel**: Abstract base class with CRUD operations
- **Decorators**: Type-safe decorators for defining models, fields, and relationships
- **Model Registry**: Central registry for model management
- **Validation System**: Built-in validation with custom validators

```typescript
// Example: Model definition with decorators
@Model({
  scope: 'user',
  type: 'docstore',
  sharding: { strategy: 'hash', count: 4, key: 'userId' },
})
class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @BelongsTo(() => User, 'userId')
  user: User;
}
```

### 2. Database Management Layer

Handles the complexity of distributed database operations:

#### Database Manager

- **User-Scoped Databases**: Each user gets their own database instance
- **Global Databases**: Shared databases for global data
- **Automatic Creation**: Databases are created on-demand
- **Lifecycle Management**: Handles database initialization and cleanup

#### Shard Manager

- **Distribution Strategy**: Distributes data across multiple databases
- **Hash-based Sharding**: Uses consistent hashing for data distribution
- **Range-based Sharding**: Distributes data based on value ranges
- **User-based Sharding**: Dedicated shards per user or user group

```typescript
// Example: Database scoping
@Model({ scope: 'user' }) // Each user gets their own database
class UserPost extends BaseModel {}

@Model({ scope: 'global' }) // Shared across all users
class GlobalNews extends BaseModel {}
```

### 3. Query System

Provides powerful querying capabilities with optimization:

#### Query Builder

- **Chainable API**: Fluent interface for building queries
- **Type Safety**: Full TypeScript support with auto-completion
- **Complex Conditions**: Support for complex where clauses
- **Relationship Loading**: Eager and lazy loading of relationships

#### Query Executor

- **Smart Routing**: Routes queries to appropriate databases/shards
- **Optimization**: Automatically optimizes query execution
- **Parallel Execution**: Executes queries across shards in parallel
- **Result Aggregation**: Combines results from multiple sources

#### Query Cache

- **Intelligent Caching**: Caches frequently accessed data
- **Cache Invalidation**: Automatic cache invalidation on updates
- **Memory Management**: Efficient memory usage with LRU eviction

```typescript
// Example: Complex query with optimization
const posts = await Post.query()
  .where('userId', currentUser.id)
  .where('isPublic', true)
  .where('createdAt', '>', Date.now() - 30 * 24 * 60 * 60 * 1000)
  .with(['user', 'comments.user'])
  .orderBy('popularity', 'desc')
  .limit(20)
  .cache(300) // Cache for 5 minutes
  .find();
```

### 4. Relationship Management

Handles complex relationships between distributed models:

#### Relationship Manager

- **Lazy Loading**: Load relationships on-demand
- **Eager Loading**: Pre-load relationships to reduce queries
- **Cross-Database**: Handle relationships across different databases
- **Performance Optimization**: Batch loading and caching

#### Relationship Cache

- **Intelligent Caching**: Cache relationship data based on access patterns
- **Consistency**: Maintain consistency across cached relationships
- **Memory Efficiency**: Optimize memory usage for large datasets

```typescript
// Example: Complex relationships
@Model({ scope: 'global' })
class User extends BaseModel {
  @HasMany(() => Post, 'userId')
  posts: Post[];

  @ManyToMany(() => User, 'followers', 'following')
  followers: User[];
}

// Load user with all relationships
const user = await User.findById(userId, {
  with: ['posts.comments', 'followers.posts'],
});
```

### 5. Automatic Features

Provides built-in optimization and convenience features:

#### Pinning Manager

- **Automatic Pinning**: Intelligently pin important data
- **Popularity-based**: Pin data based on access frequency
- **Tiered Pinning**: Different pinning strategies for different data types
- **Resource Management**: Optimize pinning resources across the network

#### PubSub Manager

- **Event Publishing**: Automatically publish model events
- **Real-time Updates**: Enable real-time application features
- **Event Filtering**: Intelligent event routing and filtering
- **Performance**: Efficient event handling with batching

```typescript
// Example: Automatic features in action
@Model({
  pinning: { strategy: 'popularity', factor: 2 },
  pubsub: { publishEvents: ['create', 'update'] },
})
class ImportantData extends BaseModel {
  // Data is automatically pinned based on popularity
  // Events are published on create/update
}
```

### 6. Migration System

Handles schema evolution and data transformation:

#### Migration Manager

- **Version Management**: Track schema versions across databases
- **Safe Migrations**: Rollback capabilities for failed migrations
- **Data Transformation**: Transform existing data during migrations
- **Conflict Resolution**: Handle migration conflicts in distributed systems

#### Migration Builder

- **Fluent API**: Easy-to-use migration definition
- **Validation**: Pre and post migration validation
- **Batch Processing**: Handle large datasets efficiently
- **Progress Tracking**: Monitor migration progress

```typescript
// Example: Schema migration
const migration = createMigration('add_user_profiles', '1.1.0')
  .addField('User', 'profilePicture', {
    type: 'string',
    required: false,
  })
  .transformData('User', (user) => ({
    ...user,
    displayName: user.username || 'Anonymous',
  }))
  .addValidator('check_profile_data', async (context) => {
    // Validate migration
    return { valid: true, errors: [], warnings: [] };
  })
  .build();

await migrationManager.runMigration(migration.id);
```

## Data Flow

### 1. Model Operation Flow

```
User Code → Model Method → Database Manager → Shard Manager → OrbitDB → IPFS
    ↑                                                                    ↓
    └─── Query Cache ← Query Optimizer ← Query Executor ←──────────────────
```

### 2. Query Execution Flow

1. **Query Building**: User builds query using Query Builder
2. **Optimization**: Query Optimizer analyzes and optimizes the query
3. **Routing**: Query Executor determines which databases/shards to query
4. **Execution**: Parallel execution across relevant databases
5. **Aggregation**: Results are combined and returned
6. **Caching**: Results are cached for future queries

### 3. Relationship Loading Flow

1. **Detection**: Framework detects relationship access
2. **Strategy**: Determines lazy vs eager loading strategy
3. **Batching**: Batches multiple relationship loads
4. **Caching**: Caches loaded relationships
5. **Resolution**: Returns resolved relationship data

## Scalability Features

### Horizontal Scaling

- **Automatic Sharding**: Data is automatically distributed across shards
- **Dynamic Scaling**: Add new shards without downtime
- **Load Balancing**: Distribute queries across available resources
- **Peer Distribution**: Leverage IPFS network for data distribution

### Performance Optimization

- **Query Optimization**: Automatic query optimization and caching
- **Lazy Loading**: Load data only when needed
- **Batch Operations**: Combine multiple operations for efficiency
- **Memory Management**: Efficient memory usage with automatic cleanup

### Data Consistency

- **Eventual Consistency**: Handle distributed system consistency challenges
- **Conflict Resolution**: Automatic conflict resolution strategies
- **Version Management**: Track data versions across the network
- **Validation**: Ensure data integrity with comprehensive validation

## Security Considerations

### Access Control

- **User-Scoped Data**: Automatic isolation of user data
- **Permission System**: Built-in permission checking
- **Validation**: Input validation and sanitization
- **Audit Logging**: Track all data operations

### Data Protection

- **Encryption**: Support for data encryption at rest and in transit
- **Privacy**: User-scoped databases ensure data privacy
- **Network Security**: Leverage IPFS and OrbitDB security features
- **Key Management**: Secure key storage and rotation

## Framework Lifecycle

### Initialization

1. **Service Setup**: Initialize IPFS and OrbitDB services
2. **Framework Init**: Initialize DebrosFramework with services
3. **Model Registration**: Register application models
4. **Database Creation**: Create necessary databases on-demand

### Operation

1. **Request Processing**: Handle user requests through models
2. **Query Execution**: Execute optimized queries across shards
3. **Data Management**: Manage data lifecycle and cleanup
4. **Event Publishing**: Publish relevant events through PubSub

### Shutdown

1. **Graceful Shutdown**: Complete ongoing operations
2. **Data Persistence**: Ensure all data is persisted
3. **Resource Cleanup**: Clean up resources and connections
4. **Service Shutdown**: Stop underlying services

## Best Practices

### Model Design

- Use appropriate scoping (user vs global) based on data access patterns
- Design efficient sharding strategies for your data distribution
- Implement proper validation to ensure data integrity
- Use relationships judiciously to avoid performance issues

### Query Optimization

- Use indexes for frequently queried fields
- Implement proper caching strategies
- Use eager loading for predictable relationship access
- Monitor query performance and optimize accordingly

### Data Management

- Implement proper migration strategies for schema evolution
- Use appropriate pinning strategies for data availability
- Monitor and manage resource usage
- Implement proper error handling and recovery

This architecture enables DebrosFramework to provide a powerful, scalable, and easy-to-use abstraction over the complexities of distributed systems while maintaining the benefits of decentralization.
