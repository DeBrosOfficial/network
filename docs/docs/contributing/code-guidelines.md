---
sidebar_position: 3
---

# Code Guidelines

This document outlines the coding standards, best practices, and architectural principles for contributing to DebrosFramework.

## 🎯 Core Principles

### 1. Developer Experience First

Every API decision should prioritize developer experience:

- **Intuitive naming** - Use clear, descriptive names
- **Consistent patterns** - Follow established conventions
- **Helpful errors** - Provide actionable error messages
- **Complete TypeScript** - Full type safety and IntelliSense

### 2. Performance by Default

Optimize for common use cases:

- **Lazy loading** - Load data only when needed
- **Automatic caching** - Cache frequently accessed data
- **Efficient queries** - Optimize database operations
- **Memory management** - Clean up resources properly

### 3. Scalability Built-in

Design for applications with millions of users:

- **Automatic sharding** - Distribute data effectively
- **Parallel processing** - Execute operations concurrently
- **Resource optimization** - Use resources efficiently
- **Graceful degradation** - Handle failures elegantly

## 📝 TypeScript Standards

### Type Safety Requirements

All code must be fully typed with strict TypeScript:

```typescript
// ✅ Good - Explicit types
interface UserCreateData {
  username: string;
  email: string;
  bio?: string;
}

async function createUser(data: UserCreateData): Promise<User> {
  // Implementation
}

// ❌ Bad - Using any
async function createUser(data: any): Promise<any> {
  // Implementation
}
```

### Interface Design

Use interfaces for all public APIs:

```typescript
// ✅ Good - Clear interface definition
interface QueryOptions {
  limit?: number;
  offset?: number;
  orderBy?: OrderByClause[];
  with?: string[];
  cache?: boolean | number;
}

// ✅ Good - Generic interfaces
interface PaginatedResult<T> {
  data: T[];
  total: number;
  page: number;
  perPage: number;
  hasMore: boolean;
}
```

### Error Handling

Use typed error classes with helpful messages:

```typescript
// ✅ Good - Specific error types
export class ValidationError extends DebrosFrameworkError {
  constructor(
    public field: string,
    public value: any,
    public constraint: string,
    message?: string,
  ) {
    super(message || `Validation failed for field '${field}': ${constraint}`, 'VALIDATION_ERROR');
  }
}

// ✅ Good - Error usage
throw new ValidationError('email', 'invalid-email', 'must be valid email format');
```

### Generic Programming

Use generics for reusable components:

```typescript
// ✅ Good - Generic model operations
abstract class BaseModel {
  static async create<T extends BaseModel>(
    this: ModelConstructor<T>,
    data: Partial<T>,
  ): Promise<T> {
    // Implementation
  }

  static query<T extends BaseModel>(this: ModelConstructor<T>): QueryBuilder<T> {
    // Implementation
  }
}
```

## 🏗️ Architecture Patterns

### Decorator Pattern

Use decorators consistently for metadata:

```typescript
// ✅ Good - Consistent decorator usage
@Model({
  scope: 'global',
  type: 'docstore',
  sharding: { strategy: 'hash', count: 4, key: 'id' },
})
export class User extends BaseModel {
  @Field({
    type: 'string',
    required: true,
    unique: true,
    validate: (value: string) => value.length >= 3,
  })
  username: string;

  @HasMany(() => Post, 'userId')
  posts: Post[];
}
```

### Service Pattern

Use dependency injection for services:

```typescript
// ✅ Good - Service injection
export class DatabaseManager {
  constructor(
    private orbitDBService: FrameworkOrbitDBService,
    private shardManager: ShardManager,
    private configManager: ConfigManager,
  ) {}

  async getDatabaseForModel<T extends BaseModel>(
    modelClass: ModelConstructor<T>,
    userId?: string,
  ): Promise<Database> {
    // Implementation
  }
}
```

### Builder Pattern

Use builders for complex configuration:

```typescript
// ✅ Good - Fluent builder API
const migration = createMigration('add_user_profiles', '1.1.0')
  .addField('User', 'profilePicture', { type: 'string', required: false })
  .addField('User', 'bio', { type: 'string', required: false })
  .transformData('User', (user) => ({
    ...user,
    displayName: user.username || 'Anonymous',
  }))
  .addValidator('check_profile_data', async (context) => {
    // Validation logic
  })
  .build();
```

## 🔄 Async/Await Patterns

### Promise Handling

Always use async/await instead of Promise chains:

```typescript
// ✅ Good - async/await
async function getUserWithPosts(userId: string): Promise<User> {
  const user = await User.findById(userId);
  if (!user) {
    throw new NotFoundError('User', userId);
  }

  const posts = await Post.query().where('userId', userId).orderBy('createdAt', 'desc').find();

  user.posts = posts;
  return user;
}

// ❌ Bad - Promise chains
function getUserWithPosts(userId: string): Promise<User> {
  return User.findById(userId).then((user) => {
    if (!user) {
      throw new NotFoundError('User', userId);
    }
    return Post.query()
      .where('userId', userId)
      .orderBy('createdAt', 'desc')
      .find()
      .then((posts) => {
        user.posts = posts;
        return user;
      });
  });
}
```

### Error Propagation

Let errors bubble up with proper context:

```typescript
// ✅ Good - Error context
async function createUserWithProfile(userData: UserCreateData): Promise<User> {
  try {
    const user = await User.create(userData);

    try {
      await UserProfile.create({
        userId: user.id,
        displayName: userData.username,
      });
    } catch (error) {
      // Clean up user if profile creation fails
      await user.delete();
      throw new OperationError('Failed to create user profile', 'USER_PROFILE_CREATION_FAILED', {
        userId: user.id,
        originalError: error,
      });
    }

    return user;
  } catch (error) {
    if (error instanceof ValidationError) {
      throw error; // Re-throw validation errors as-is
    }
    throw new OperationError('Failed to create user', 'USER_CREATION_FAILED', {
      userData,
      originalError: error,
    });
  }
}
```

## 🧪 Testing Standards

### Unit Test Structure

Use consistent test structure:

```typescript
// ✅ Good - Clear test structure
describe('User Model', () => {
  beforeEach(async () => {
    await setupTestDatabase();
  });

  afterEach(async () => {
    await cleanupTestDatabase();
  });

  describe('create', () => {
    it('should create user with valid data', async () => {
      // Arrange
      const userData = {
        username: 'testuser',
        email: 'test@example.com',
      };

      // Act
      const user = await User.create(userData);

      // Assert
      expect(user).toBeDefined();
      expect(user.username).toBe(userData.username);
      expect(user.email).toBe(userData.email);
      expect(user.id).toBeDefined();
    });

    it('should throw ValidationError for invalid email', async () => {
      // Arrange
      const userData = {
        username: 'testuser',
        email: 'invalid-email',
      };

      // Act & Assert
      await expect(User.create(userData)).rejects.toThrow(ValidationError);
    });
  });

  describe('relationships', () => {
    it('should load posts relationship', async () => {
      // Test relationship loading
    });
  });
});
```

### Integration Test Patterns

Test real-world scenarios:

```typescript
// ✅ Good - Integration test
describe('Blog Scenario Integration', () => {
  let framework: DebrosFramework;

  beforeAll(async () => {
    framework = await setupFrameworkForTesting();
  });

  afterAll(async () => {
    await framework.stop();
  });

  it('should handle complete blog workflow', async () => {
    // Create user
    const user = await User.create({
      username: 'blogger',
      email: 'blogger@example.com',
    });

    // Create post
    const post = await Post.create({
      title: 'Test Post',
      content: 'Test content',
      userId: user.id,
    });

    // Add comment
    const comment = await Comment.create({
      content: 'Great post!',
      postId: post.id,
      authorId: user.id,
    });

    // Verify relationships
    const postWithComments = await Post.query()
      .where('id', post.id)
      .with(['comments.author'])
      .findOne();

    expect(postWithComments).toBeDefined();
    expect(postWithComments!.comments).toHaveLength(1);
    expect(postWithComments!.comments[0].author.username).toBe('blogger');
  });
});
```

## 📊 Performance Guidelines

### Query Optimization

Write efficient queries:

```typescript
// ✅ Good - Optimized query
const recentPosts = await Post.query()
  .where('publishedAt', '>', Date.now() - 7 * 24 * 60 * 60 * 1000)
  .where('isPublished', true)
  .with(['author']) // Eager load to avoid N+1
  .orderBy('publishedAt', 'desc')
  .limit(20)
  .cache(300) // Cache for 5 minutes
  .find();

// ❌ Bad - Inefficient query
const allPosts = await Post.query().find(); // Loads everything
const recentPosts = allPosts
  .filter((p) => p.publishedAt > Date.now() - 7 * 24 * 60 * 60 * 1000)
  .filter((p) => p.isPublished)
  .slice(0, 20);
```

### Memory Management

Clean up resources properly:

```typescript
// ✅ Good - Resource cleanup
export class QueryCache {
  private cache = new Map<string, CacheEntry>();
  private cleanupInterval: NodeJS.Timeout;

  constructor(private ttl: number = 300000) {
    this.cleanupInterval = setInterval(() => {
      this.cleanup();
    }, this.ttl / 2);
  }

  async stop(): Promise<void> {
    if (this.cleanupInterval) {
      clearInterval(this.cleanupInterval);
    }
    this.cache.clear();
  }

  private cleanup(): void {
    const now = Date.now();
    for (const [key, entry] of this.cache.entries()) {
      if (entry.expiresAt < now) {
        this.cache.delete(key);
      }
    }
  }
}
```

### Async Performance

Use Promise.all for parallel operations:

```typescript
// ✅ Good - Parallel execution
async function getUserDashboardData(userId: string): Promise<DashboardData> {
  const [user, recentPosts, stats, notifications] = await Promise.all([
    User.findById(userId),
    Post.query().where('userId', userId).limit(5).find(),
    getUserStats(userId),
    getRecentNotifications(userId),
  ]);

  return {
    user,
    recentPosts,
    stats,
    notifications,
  };
}

// ❌ Bad - Sequential execution
async function getUserDashboardData(userId: string): Promise<DashboardData> {
  const user = await User.findById(userId);
  const recentPosts = await Post.query().where('userId', userId).limit(5).find();
  const stats = await getUserStats(userId);
  const notifications = await getRecentNotifications(userId);

  return {
    user,
    recentPosts,
    stats,
    notifications,
  };
}
```

## 🔧 Code Organization

### File Structure

Organize code logically:

```
src/framework/
├── core/                    # Core framework components
│   ├── ConfigManager.ts
│   ├── DatabaseManager.ts
│   └── ModelRegistry.ts
├── models/                  # Model system
│   ├── BaseModel.ts
│   └── decorators/
│       ├── Field.ts
│       ├── Model.ts
│       ├── relationships.ts
│       └── hooks.ts
├── query/                   # Query system
│   ├── QueryBuilder.ts
│   ├── QueryExecutor.ts
│   └── QueryOptimizer.ts
└── types/                   # Type definitions
    ├── framework.ts
    ├── models.ts
    └── queries.ts
```

### Import Organization

Use consistent import patterns:

```typescript
// ✅ Good - Organized imports
// Node.js built-ins
import { EventEmitter } from 'events';
import { promisify } from 'util';

// External packages
import { Database } from '@orbitdb/core';
import { CID } from 'multiformats';

// Framework internals
import { BaseModel } from '../models/BaseModel';
import { ConfigManager } from '../core/ConfigManager';
import { DatabaseManager } from '../core/DatabaseManager';

// Types
import type { ModelConfig, FieldConfig } from '../types/models';
import type { QueryOptions, QueryResult } from '../types/queries';
```

### Export Patterns

Use consistent export patterns:

```typescript
// ✅ Good - Clear exports
// Main class export
export class QueryBuilder<T extends BaseModel> {
  // Implementation
}

// Type exports
export type { QueryOptions, QueryResult, WhereClause, OrderByClause };

// Utility exports
export { buildWhereClause, optimizeQuery, validateQueryOptions };

// Default export for main functionality
export default QueryBuilder;
```

## 📚 Documentation Standards

### JSDoc Comments

Document all public APIs:

````typescript
/**
 * Creates a new model instance with the provided data.
 *
 * @template T - The model type extending BaseModel
 * @param data - The data to create the model with
 * @param options - Optional creation options
 * @returns Promise resolving to the created model instance
 *
 * @throws {ValidationError} When data validation fails
 * @throws {DatabaseError} When database operation fails
 *
 * @example
 * ```typescript
 * const user = await User.create({
 *   username: 'john',
 *   email: 'john@example.com'
 * });
 * ```
 */
static async create<T extends BaseModel>(
  this: ModelConstructor<T>,
  data: Partial<T>,
  options?: CreateOptions
): Promise<T> {
  // Implementation
}
````

### Code Comments

Add comments for complex logic:

```typescript
// ✅ Good - Explaining complex logic
private calculateShardIndex(key: string, shardCount: number): number {
  // Use consistent hashing to distribute data evenly across shards
  // This ensures that the same key always maps to the same shard
  const hash = this.hashFunction(key);

  // Use modulo to map hash to shard index
  // Add 1 to avoid negative numbers with certain hash functions
  return Math.abs(hash) % shardCount;
}

// ✅ Good - Explaining business logic
async ensureUserDatabaseExists(userId: string): Promise<Database> {
  // Check if user database already exists in cache
  const existingDb = this.userDatabases.get(userId);
  if (existingDb) {
    return existingDb;
  }

  // Create new user database with user-specific configuration
  // This provides data isolation and improved performance
  const database = await this.createUserDatabase(userId);

  // Cache the database for future use
  this.userDatabases.set(userId, database);

  return database;
}
```

## ⚡ Performance Monitoring

### Metrics Collection

Add metrics to important operations:

```typescript
// ✅ Good - Performance monitoring
export class QueryExecutor {
  private metrics = new Map<string, PerformanceMetric>();

  async executeQuery<T>(query: QueryBuilder<T>): Promise<QueryResult<T>> {
    const startTime = Date.now();
    const queryKey = query.toString();

    try {
      const result = await this.internalExecuteQuery(query);

      // Record successful execution
      this.recordMetric(queryKey, Date.now() - startTime, true);

      return result;
    } catch (error) {
      // Record failed execution
      this.recordMetric(queryKey, Date.now() - startTime, false);
      throw error;
    }
  }

  private recordMetric(queryKey: string, duration: number, success: boolean): void {
    const existing = this.metrics.get(queryKey) || {
      count: 0,
      totalDuration: 0,
      successCount: 0,
      averageDuration: 0,
    };

    existing.count++;
    existing.totalDuration += duration;
    if (success) existing.successCount++;
    existing.averageDuration = existing.totalDuration / existing.count;

    this.metrics.set(queryKey, existing);
  }
}
```

## 🔒 Security Considerations

### Input Validation

Validate all inputs thoroughly:

```typescript
// ✅ Good - Input validation
export class UserService {
  async createUser(userData: UserCreateData): Promise<User> {
    // Validate required fields
    if (!userData.username || typeof userData.username !== 'string') {
      throw new ValidationError('username', userData.username, 'required string');
    }

    // Sanitize username
    const sanitizedUsername = userData.username.trim().toLowerCase();

    // Check length constraints
    if (sanitizedUsername.length < 3 || sanitizedUsername.length > 20) {
      throw new ValidationError('username', sanitizedUsername, 'length between 3-20');
    }

    // Check for valid characters
    if (!/^[a-zA-Z0-9_]+$/.test(sanitizedUsername)) {
      throw new ValidationError('username', sanitizedUsername, 'alphanumeric and underscore only');
    }

    // Check uniqueness
    const existingUser = await User.findOne({ username: sanitizedUsername });
    if (existingUser) {
      throw new ConflictError('Username already exists');
    }

    return User.create({
      ...userData,
      username: sanitizedUsername,
    });
  }
}
```

### Error Information

Don't leak sensitive information in errors:

```typescript
// ✅ Good - Safe error messages
catch (error) {
  if (error instanceof DatabaseConnectionError) {
    // Don't expose internal connection details
    throw new OperationError(
      'Database operation failed',
      'DATABASE_ERROR',
      { operation: 'create_user' } // Safe context only
    );
  }
  throw error;
}

// ❌ Bad - Leaking sensitive info
catch (error) {
  throw new Error(`Database connection failed: ${error.message} at ${error.stack}`);
}
```

---

These guidelines help ensure that DebrosFramework maintains high code quality, performance, and developer experience. When in doubt, prioritize clarity, type safety, and developer experience.

**Next:** Check out our [Testing Guide](./testing-guide) to learn about writing comprehensive tests.
