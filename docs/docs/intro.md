---
sidebar_position: 1
---

# Welcome to DebrosFramework

**DebrosFramework** is a powerful Node.js framework that provides an ORM-like abstraction over OrbitDB and IPFS, making it easy to build scalable decentralized applications.

## What is DebrosFramework?

DebrosFramework simplifies the development of decentralized applications by providing:

- **Model-based Abstraction**: Define your data models using decorators and TypeScript classes
- **Automatic Database Management**: Handle user-scoped and global databases automatically
- **Smart Sharding**: Distribute data across multiple databases for scalability
- **Advanced Query System**: Rich query capabilities with relationship loading and caching
- **Automatic Features**: Built-in pinning strategies and PubSub event publishing
- **Migration System**: Schema evolution and data transformation capabilities
- **Type Safety**: Full TypeScript support with strong typing throughout

## Key Features

### 🏗️ Model-Driven Development

Define your data models using familiar decorator patterns:

```typescript
@Model({
  scope: 'user',
  type: 'docstore',
  sharding: { strategy: 'hash', count: 4, key: 'userId' },
})
class User extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  username: string;

  @Field({ type: 'string', required: true, unique: true })
  email: string;

  @HasMany(() => Post, 'userId')
  posts: Post[];
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

## Architecture Overview

DebrosFramework is built around several core components:

1. **Models & Decorators**: Define your data structure and behavior
2. **Database Manager**: Handles database creation and management
3. **Shard Manager**: Distributes data across multiple databases
4. **Query System**: Processes queries with optimization and caching
5. **Relationship Manager**: Handles complex relationships between models
6. **Migration System**: Manages schema evolution over time
7. **Automatic Features**: Pinning, PubSub, and performance optimization

## Who Should Use DebrosFramework?

DebrosFramework is perfect for developers who want to:

- Build decentralized applications without dealing with low-level OrbitDB complexities
- Create scalable applications that can handle millions of users
- Use familiar ORM patterns in a decentralized environment
- Implement complex data relationships in distributed systems
- Focus on business logic rather than infrastructure concerns

## Getting Started

Ready to build your first decentralized application? Check out our [Getting Started Guide](./getting-started) to set up your development environment and create your first models.

## Community and Support

- 📖 [Documentation](./getting-started) - Comprehensive guides and examples
- 💻 [GitHub Repository](https://github.com/debros/network) - Source code and issue tracking
- 💬 [Discord Community](#) - Chat with other developers
- 📧 [Support Email](#) - Get help from the core team

---

_DebrosFramework is designed to make decentralized application development as simple as traditional web development, while providing the benefits of distributed systems._
