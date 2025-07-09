# @debros/network

**DebrosFramework** - A powerful Node.js framework that provides an ORM-like abstraction over OrbitDB and IPFS, making it easy to build scalable decentralized applications.

## What is DebrosFramework?

DebrosFramework simplifies the development of decentralized applications by providing:

- **Model-based Abstraction**: Define your data models using decorators and TypeScript classes
- **Automatic Database Management**: Handle user-scoped and global databases automatically
- **Smart Sharding**: Distribute data across multiple databases for scalability
- **Advanced Query System**: Rich query capabilities with relationship loading and caching
- **Automatic Features**: Built-in pinning strategies and PubSub event publishing
- **Migration System**: Schema evolution and data transformation capabilities
- **Type Safety**: Full TypeScript support with strong typing throughout

## Installation

```bash
npm install @debros/network
```

## Quick Start

### 1. Define Your Models

```typescript
import { BaseModel, Model, Field, HasMany } from '@debros/network';

@Model({
  scope: 'global',
  type: 'docstore',
  sharding: { strategy: 'hash', count: 4, key: 'id' },
})
export class User extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  username: string;

  @Field({ type: 'string', required: true, unique: true })
  email: string;

  @HasMany(() => Post, 'userId')
  posts: Post[];
}

@Model({
  scope: 'user',
  type: 'docstore',
  sharding: { strategy: 'user', count: 2, key: 'userId' },
})
export class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  userId: string;
}
```

### 2. Initialize the Framework

```typescript
import { DebrosFramework } from '@debros/network';
import { setupOrbitDB, setupIPFS } from './services';

async function startApp() {
  // Initialize services
  const orbitDBService = await setupOrbitDB();
  const ipfsService = await setupIPFS();

  // Initialize framework
  const framework = new DebrosFramework({
    features: {
      queryCache: true,
      automaticPinning: true,
      pubsub: true,
    },
  });

  await framework.initialize(orbitDBService, ipfsService);
  console.log('✅ DebrosFramework initialized successfully!');

  // Create a user
  const user = await User.create({
    username: 'alice',
    email: 'alice@example.com',
  });

  // Create a post
  const post = await Post.create({
    title: 'My First Post',
    content: 'Hello DebrosFramework!',
    userId: user.id,
  });

  // Query with relationships
  const usersWithPosts = await User.query().with(['posts']).where('username', 'alice').find();

  console.log('User:', usersWithPosts[0]);
  console.log('Posts:', usersWithPosts[0].posts);
}
```

## Key Features

### 🏗️ Model-Driven Development

Define your data models using familiar decorator patterns:

```typescript
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

### 🔍 Powerful Query System

Build complex queries with relationship loading:

```typescript
const users = await User.query()
  .where('isActive', true)
  .where('registeredAt', '>', Date.now() - 30 * 24 * 60 * 60 * 1000)
  .with(['posts', 'followers'])
  .orderBy('username')
  .limit(20)
  .find();
```

### 🚀 Automatic Scaling

Handle millions of users with automatic sharding and pinning:

```typescript
// Framework automatically:
// - Creates user-scoped databases
// - Distributes data across shards
// - Manages pinning strategies
// - Optimizes query routing
```

### 🔄 Schema Evolution

Migrate your data structures safely:

```typescript
const migration = createMigration('add_user_profiles', '1.1.0')
  .addField('User', 'profilePicture', { type: 'string', required: false })
  .addField('User', 'bio', { type: 'string', required: false })
  .transformData('User', (user) => ({
    ...user,
    displayName: user.username || 'Anonymous',
  }))
  .build();
```

### 🔗 Rich Relationships

Handle complex relationships between models:

```typescript
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

### ⚡ Performance Features

```typescript
// Query caching
const cachedUsers = await User.query()
  .where('isActive', true)
  .cache(300) // Cache for 5 minutes
  .find();

// Eager loading
const usersWithPosts = await User.query().with(['posts.comments']).find();

// Optimized pagination
const page = await User.query().orderBy('createdAt', 'desc').paginate(1, 20);
```

### 🎯 Model Hooks

```typescript
export class User extends BaseModel {
  @BeforeCreate()
  async beforeCreate() {
    this.createdAt = Date.now();
    // Hash password, validate data, etc.
  }

  @AfterCreate()
  async afterCreate() {
    // Send welcome email, create defaults, etc.
  }
}
```

## API Reference

### Framework Management

- `new DebrosFramework(config?)` - Create framework instance
- `framework.initialize(orbitDBService, ipfsService, config?)` - Initialize framework
- `framework.start()` - Start the framework
- `framework.stop()` - Stop the framework
- `framework.getStatus()` - Get framework status

### Model Operations

- `Model.create(data)` - Create a new model instance
- `Model.findById(id, options?)` - Find model by ID
- `Model.findOne(criteria, options?)` - Find single model
- `Model.query()` - Start a query builder
- `model.save()` - Save model changes
- `model.delete()` - Delete model instance

### Query System

- `Model.query().where(field, operator, value)` - Add where condition
- `Model.query().with(relationships)` - Eager load relationships
- `Model.query().orderBy(field, direction)` - Add ordering
- `Model.query().limit(count)` - Limit results
- `Model.query().offset(count)` - Add offset
- `Model.query().paginate(page, perPage)` - Paginate results
- `Model.query().cache(ttl)` - Cache query results
- `Model.query().find()` - Execute query
- `Model.query().count()` - Count results

### Decorators

- `@Model(config)` - Define model configuration
- `@Field(config)` - Define field properties
- `@BelongsTo(target, foreignKey)` - Many-to-one relationship
- `@HasMany(target, foreignKey)` - One-to-many relationship
- `@HasOne(target, foreignKey)` - One-to-one relationship
- `@ManyToMany(target, through)` - Many-to-many relationship
- `@BeforeCreate()`, `@AfterCreate()` - Lifecycle hooks
- `@BeforeUpdate()`, `@AfterUpdate()` - Update hooks
- `@BeforeDelete()`, `@AfterDelete()` - Delete hooks

### Migration System

- `createMigration(name, version)` - Create new migration
- `migration.addField(model, field, config)` - Add field to model
- `migration.removeField(model, field)` - Remove field from model
- `migration.transformData(model, transformer)` - Transform existing data
- `migrationManager.runPendingMigrations()` - Run pending migrations

## Configuration

### Framework Configuration

```typescript
import { DebrosFramework, PRODUCTION_CONFIG, DEVELOPMENT_CONFIG } from '@debros/network';

// Development configuration
const framework = new DebrosFramework({
  ...DEVELOPMENT_CONFIG,
  features: {
    queryCache: true,
    automaticPinning: false,
    pubsub: true,
    relationshipCache: true,
    autoMigration: true,
  },
  performance: {
    queryTimeout: 30000,
    batchSize: 50,
  },
  monitoring: {
    enableMetrics: true,
    logLevel: 'debug',
  },
});

// Production configuration
const prodFramework = new DebrosFramework({
  ...PRODUCTION_CONFIG,
  performance: {
    queryTimeout: 10000,
    batchSize: 200,
    maxConcurrentOperations: 500,
  },
});
```

### Model Configuration

```typescript
@Model({
  scope: 'global', // 'user' or 'global'
  type: 'docstore', // OrbitDB store type
  sharding: {
    strategy: 'hash', // 'hash', 'range', or 'user'
    count: 4, // Number of shards
    key: 'id', // Sharding key
  },
  pinning: {
    strategy: 'popularity', // Pinning strategy
    factor: 2,
  },
})
export class MyModel extends BaseModel {
  // Model definition
}
```

## Architecture Overview

DebrosFramework is built around several core components:

1. **Models & Decorators**: Define your data structure and behavior
2. **Database Manager**: Handles database creation and management
3. **Shard Manager**: Distributes data across multiple databases
4. **Query System**: Processes queries with optimization and caching
5. **Relationship Manager**: Handles complex relationships between models
6. **Migration System**: Manages schema evolution over time
7. **Automatic Features**: Pinning, PubSub, and performance optimization

### Key Benefits

- **Scalability**: Automatic sharding and distributed data management
- **Performance**: Built-in caching, query optimization, and lazy loading
- **Developer Experience**: Familiar ORM patterns with TypeScript support
- **Flexibility**: Support for various data patterns and relationships
- **Reliability**: Comprehensive error handling and recovery mechanisms

## Getting Started

Ready to build your first decentralized application? Check out our comprehensive documentation:

- **[📖 Complete Documentation](./docs)** - Comprehensive guides and examples
- **[🚀 Getting Started Guide](./docs/docs/getting-started.md)** - Set up your development environment
- **[🏗️ Architecture Overview](./docs/docs/core-concepts/architecture.md)** - Understand how the framework works
- **[📝 API Reference](./docs/docs/api/overview.md)** - Complete API documentation
- **[💡 Examples](./docs/docs/examples/basic-usage.md)** - Practical usage examples

## Testing

```bash
# Run unit tests
npm run test:unit

# Run integration tests
npm run test:real

# Run specific blog scenario integration test
npm run test:blog-integration
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](./CONTRIBUTING.md) for details.

## License

This project is licensed under the GNU GPL v3.0 License - see the [LICENSE](./LICENSE) file for details.

---

**DebrosFramework** - Making decentralized application development as simple as traditional web development, while providing the benefits of distributed systems.
