# DebrosFramework Project Overview

## Project Identity
**DebrosFramework** is a powerful Node.js framework that provides an ORM-like abstraction over OrbitDB and IPFS, making it easy to build scalable decentralized applications.

- **Package Name**: `@debros/network`
- **Version**: 0.5.1-beta (Active Development)
- **License**: GNU GPL v3.0
- **Language**: TypeScript
- **Framework Type**: Decentralized Application Framework

## What This Project Does

DebrosFramework simplifies the development of decentralized applications by providing:

### Core Capabilities
- **Model-based Abstraction**: Define data models using decorators and TypeScript classes
- **Automatic Database Management**: Handle user-scoped and global databases automatically
- **Smart Sharding**: Distribute data across multiple databases for scalability
- **Advanced Query System**: Rich query capabilities with relationship loading and caching
- **Automatic Features**: Built-in pinning strategies and PubSub event publishing
- **Migration System**: Schema evolution and data transformation capabilities
- **Type Safety**: Full TypeScript support with strong typing throughout

### Key Features
1. **🏗️ Model-Driven Development**: Familiar decorator patterns for data models
2. **🔍 Powerful Query System**: Complex queries with relationship loading
3. **🚀 Automatic Scaling**: Handle millions of users with automatic sharding
4. **🔄 Schema Evolution**: Safe data structure migrations
5. **🔗 Rich Relationships**: Complex relationships between models
6. **⚡ Performance Features**: Query caching, eager loading, optimized pagination
7. **🎯 Model Hooks**: Lifecycle hooks for business logic

## Architecture Overview

The framework is built around several core components:

1. **Models & Decorators**: Define data structure and behavior
2. **Database Manager**: Handles database creation and management
3. **Shard Manager**: Distributes data across multiple databases
4. **Query System**: Processes queries with optimization and caching
5. **Relationship Manager**: Handles complex relationships between models
6. **Migration System**: Manages schema evolution over time
7. **Automatic Features**: Pinning, PubSub, and performance optimization

## Technology Stack

### Core Dependencies
- **OrbitDB**: Distributed peer-to-peer database on IPFS
- **IPFS/Helia**: Distributed file system for data storage
- **libp2p**: Peer-to-peer networking stack
- **TypeScript**: Strong typing and modern JavaScript features

### Key Libraries
- `@orbitdb/core`: Core OrbitDB functionality
- `@helia/unixfs`: IPFS file system operations
- `@libp2p/*`: Various libp2p modules for networking
- `winston`: Logging
- `node-cache`: In-memory caching
- `express`: HTTP server for integration tests

## Project Structure

```
src/framework/
├── core/           # Core framework components
│   ├── ConfigManager.ts
│   ├── DatabaseManager.ts
│   └── ModelRegistry.ts
├── models/         # Model system and decorators
│   ├── BaseModel.ts
│   └── decorators/
├── query/          # Query builder and execution
│   ├── QueryBuilder.ts
│   ├── QueryExecutor.ts
│   ├── QueryOptimizer.ts
│   └── QueryCache.ts
├── relationships/  # Relationship management
│   ├── RelationshipManager.ts
│   ├── LazyLoader.ts
│   └── RelationshipCache.ts
├── sharding/       # Data sharding logic
│   └── ShardManager.ts
├── migrations/     # Schema migration system
│   ├── MigrationManager.ts
│   └── MigrationBuilder.ts
├── pinning/        # Automatic pinning features
│   └── PinningManager.ts
├── pubsub/         # Event publishing system
│   └── PubSubManager.ts
├── services/       # External service integrations
│   ├── OrbitDBService.ts
│   ├── IPFSService.ts
│   └── RealOrbitDBService.ts
└── types/          # TypeScript type definitions
    ├── framework.ts
    ├── models.ts
    └── queries.ts
```

## Development Status

**Current State**: Beta (v0.5.1-beta) - Active Development

### What's Stable
- ✅ Core model system with decorators
- ✅ Basic CRUD operations
- ✅ Query builder and execution
- ✅ Relationship management
- ✅ Database management and sharding
- ✅ Migration system foundation

### What's In Development
- 🚧 Advanced query optimization
- 🚧 Performance optimization features
- 🚧 Production-ready configurations
- 🚧 Extended relationship types
- 🚧 Enhanced error handling

### Testing
- **Unit Tests**: Comprehensive test suite using Jest
- **Integration Tests**: Docker-based real-world scenarios
- **Blog Scenario**: Complete blogging platform test case

## Key Concepts

### Model Scoping
- **Global Models**: Shared across all users (e.g., User profiles)
- **User Models**: Each user has their own database instance (e.g., Posts, Comments)

### Sharding Strategies
- **Hash Sharding**: Distribute data based on key hashing
- **Range Sharding**: Distribute data based on value ranges
- **User Sharding**: Dedicated shards per user or user group

### Query System
- **Chainable API**: Fluent interface for building queries
- **Type Safety**: Full TypeScript support with auto-completion
- **Relationship Loading**: Eager and lazy loading strategies
- **Caching**: Intelligent query result caching

### Relationships
- **BelongsTo**: Many-to-one relationships
- **HasMany**: One-to-many relationships
- **HasOne**: One-to-one relationships
- **ManyToMany**: Many-to-many relationships (with through tables)

## Common Use Cases

### 1. Social Platforms
- User profiles and authentication
- Posts, comments, and reactions
- Friend networks and messaging
- Activity feeds and notifications

### 2. Content Management
- Blogs and publishing platforms
- Document management systems
- Media galleries and collections
- Version control for content

### 3. Collaborative Applications
- Real-time document editing
- Project management tools
- Team collaboration platforms
- Knowledge bases and wikis

### 4. Marketplace Applications
- Product catalogs and inventory
- User reviews and ratings
- Order management systems
- Payment and transaction records

## Development Workflow

### Prerequisites
- Node.js 18.0 or higher
- TypeScript knowledge
- Basic understanding of IPFS and OrbitDB concepts
- Docker (for integration tests)

### Build Process
```bash
npm run build      # Compile TypeScript
npm run dev        # Development with watch mode
npm run lint       # ESLint code checking
npm run format     # Prettier code formatting
```

### Testing
```bash
npm run test:unit                # Fast unit tests with mocks
npm run test:real               # Full integration tests with Docker
npm run test:blog-integration   # Blog scenario integration tests
```

### Key Development Principles
1. **Type Safety First**: Everything is strongly typed
2. **Decorator-Based**: Use decorators for configuration
3. **Async/Await**: All operations are promise-based
4. **Error Handling**: Comprehensive error management
5. **Performance**: Built-in optimization and caching
6. **Scalability**: Designed for distributed systems

## API Patterns

### Model Definition
```typescript
@Model({
  scope: 'user',
  type: 'docstore',
  sharding: { strategy: 'hash', count: 4, key: 'userId' }
})
class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;
  
  @BelongsTo(() => User, 'userId')
  user: User;
}
```

### Query Operations
```typescript
const posts = await Post.query()
  .where('isPublished', true)
  .where('createdAt', '>', Date.now() - 7 * 24 * 60 * 60 * 1000)
  .with(['user', 'comments'])
  .orderBy('likeCount', 'desc')
  .limit(20)
  .find();
```

### Lifecycle Hooks
```typescript
class User extends BaseModel {
  @BeforeCreate()
  setupNewUser() {
    this.registeredAt = Date.now();
  }
  
  @AfterCreate()
  async sendWelcomeEmail() {
    // Business logic after creation
  }
}
```

This framework makes building decentralized applications feel like traditional web development while providing the benefits of distributed, peer-to-peer systems.
