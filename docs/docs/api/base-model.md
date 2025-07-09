---
sidebar_position: 3
---

# BaseModel Class

The `BaseModel` class is the abstract base class for all data models in Debros Network. It provides ORM-like functionality with automatic database management, validation, relationships, and lifecycle hooks.

## Class Definition

```typescript
abstract class BaseModel {
  // Instance properties
  id: string;
  createdAt?: number;
  updatedAt?: number;

  // Static methods
  static async create<T extends BaseModel>(
    this: ModelConstructor<T>,
    data: Partial<T>,
    options?: CreateOptions,
  ): Promise<T>;

  static async findById<T extends BaseModel>(
    this: ModelConstructor<T>,
    id: string,
    options?: FindOptions,
  ): Promise<T | null>;

  static async findOne<T extends BaseModel>(
    this: ModelConstructor<T>,
    criteria: Partial<T>,
    options?: FindOptions,
  ): Promise<T | null>;

  static query<T extends BaseModel>(this: ModelConstructor<T>): QueryBuilder<T>;

  // Instance methods
  save(options?: SaveOptions): Promise<this>;
  delete(options?: DeleteOptions): Promise<boolean>;
  reload(options?: ReloadOptions): Promise<this>;
  validate(): Promise<ValidationResult>;
  toJSON(): Record<string, any>;
  clone(): this;
}
```

## Static Methods

### create(data, options?)

Creates a new model instance and saves it to the database.

**Parameters:**

- `data`: Partial model data
- `options` (optional): Creation options

**Returns:** `Promise<T>` - The created model instance

**Throws:**

- `ValidationError` - If validation fails
- `DatabaseError` - If database operation fails

**Example:**

```typescript
import { BaseModel, Model, Field } from '@debros/network';

@Model({
  scope: 'global',
  type: 'docstore',
})
class User extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  username: string;

  @Field({ type: 'string', required: true, unique: true })
  email: string;

  @Field({ type: 'number', required: false, default: 0 })
  score: number;
}

// Create a new user
const user = await User.create({
  username: 'alice',
  email: 'alice@example.com',
  score: 100,
});

console.log(user.id); // Generated ID
console.log(user.username); // 'alice'
console.log(user.createdAt); // Timestamp
```

**With validation:**

```typescript
try {
  const user = await User.create({
    username: 'ab', // Too short
    email: 'invalid-email', // Invalid format
  });
} catch (error) {
  if (error instanceof ValidationError) {
    console.log('Validation failed:', error.field, error.constraint);
  }
}
```

**With options:**

```typescript
const user = await User.create(
  {
    username: 'bob',
    email: 'bob@example.com',
  },
  {
    validate: true,
    skipHooks: false,
    userId: 'user123', // For user-scoped models
  },
);
```

### findById(id, options?)

Finds a model instance by its ID.

**Parameters:**

- `id`: The model ID to search for
- `options` (optional): Find options

**Returns:** `Promise<T | null>` - The found model or null

**Example:**

```typescript
// Basic find
const user = await User.findById('user123');
if (user) {
  console.log('Found user:', user.username);
} else {
  console.log('User not found');
}

// With relationships
const user = await User.findById('user123', {
  with: ['posts', 'profile'],
});

// With specific user context
const user = await User.findById('user123', {
  userId: 'current-user-id',
});
```

### findOne(criteria, options?)

Finds the first model instance matching the criteria.

**Parameters:**

- `criteria`: Partial model data to match
- `options` (optional): Find options

**Returns:** `Promise<T | null>` - The found model or null

**Example:**

```typescript
// Find by field
const user = await User.findOne({
  username: 'alice',
});

// Find with multiple criteria
const user = await User.findOne({
  email: 'alice@example.com',
  isActive: true,
});

// With options
const user = await User.findOne(
  {
    username: 'alice',
  },
  {
    with: ['posts'],
    cache: 300, // Cache for 5 minutes
  },
);
```

### query()

Returns a query builder for complex queries.

**Returns:** `QueryBuilder<T>` - Query builder instance

**Example:**

```typescript
// Basic query
const users = await User.query()
  .where('isActive', true)
  .orderBy('createdAt', 'desc')
  .limit(10)
  .find();

// Complex query with relationships
const activeUsers = await User.query()
  .where('isActive', true)
  .where('score', '>', 100)
  .where('registeredAt', '>', Date.now() - 30 * 24 * 60 * 60 * 1000)
  .with(['posts.comments', 'profile'])
  .orderBy('score', 'desc')
  .paginate(1, 20);

// Query with caching
const cachedUsers = await User.query()
  .where('isActive', true)
  .cache(600) // Cache for 10 minutes
  .find();
```

## Instance Methods

### save(options?)

Saves the current model instance to the database.

**Parameters:**

- `options` (optional): Save options

**Returns:** `Promise<this>` - The saved model instance

**Example:**

```typescript
const user = await User.findById('user123');
user.email = 'newemail@example.com';
user.score += 10;

await user.save();
console.log('User updated');

// With options
await user.save({
  validate: true,
  skipHooks: false,
});
```

### delete(options?)

Deletes the model instance from the database.

**Parameters:**

- `options` (optional): Delete options

**Returns:** `Promise<boolean>` - True if deletion was successful

**Example:**

```typescript
const user = await User.findById('user123');
if (user) {
  const deleted = await user.delete();
  console.log('User deleted:', deleted);
}

// With options
await user.delete({
  skipHooks: false,
  cascade: true, // Delete related records
});
```

### reload(options?)

Reloads the model instance from the database.

**Parameters:**

- `options` (optional): Reload options

**Returns:** `Promise<this>` - The reloaded model instance

**Example:**

```typescript
const user = await User.findById('user123');

// Model might be updated elsewhere
await user.reload();

// With relationships
await user.reload({
  with: ['posts', 'profile'],
});
```

### validate()

Validates the current model instance.

**Returns:** `Promise<ValidationResult>` - Validation result

**Example:**

```typescript
const user = new User();
user.username = 'alice';
user.email = 'invalid-email';

const result = await user.validate();
if (!result.valid) {
  console.log('Validation errors:', result.errors);
  // [{ field: 'email', constraint: 'must be valid email', value: 'invalid-email' }]
}
```

### toJSON()

Converts the model instance to a plain JavaScript object.

**Returns:** `Record<string, any>` - Plain object representation

**Example:**

```typescript
const user = await User.findById('user123');
const userObj = user.toJSON();

console.log(userObj);
// {
//   id: 'user123',
//   username: 'alice',
//   email: 'alice@example.com',
//   score: 100,
//   createdAt: 1234567890,
//   updatedAt: 1234567890
// }

// Useful for JSON serialization
const jsonString = JSON.stringify(user); // Calls toJSON() automatically
```

### clone()

Creates a deep copy of the model instance (without ID).

**Returns:** `this` - Cloned model instance

**Example:**

```typescript
const user = await User.findById('user123');
const userCopy = user.clone();

userCopy.username = 'alice_copy';
await userCopy.save(); // Creates new record

console.log(user.id !== userCopy.id); // true
```

## Lifecycle Hooks

### Hook Decorators

Models can define lifecycle hooks using decorators:

```typescript
@Model({ scope: 'global', type: 'docstore' })
class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'string', required: true })
  email: string;

  @Field({ type: 'number', required: false })
  loginCount: number = 0;

  @BeforeCreate()
  async beforeCreateHook() {
    this.createdAt = Date.now();
    this.updatedAt = Date.now();

    // Validate unique username
    const existing = await User.findOne({ username: this.username });
    if (existing) {
      throw new ValidationError('username', this.username, 'must be unique');
    }
  }

  @AfterCreate()
  async afterCreateHook() {
    console.log(`New user created: ${this.username}`);

    // Send welcome email
    await this.sendWelcomeEmail();

    // Create default settings
    await this.createDefaultSettings();
  }

  @BeforeUpdate()
  async beforeUpdateHook() {
    this.updatedAt = Date.now();
  }

  @AfterUpdate()
  async afterUpdateHook() {
    console.log(`User updated: ${this.username}`);
  }

  @BeforeDelete()
  async beforeDeleteHook() {
    // Clean up related data
    await this.deleteRelatedPosts();
  }

  @AfterDelete()
  async afterDeleteHook() {
    console.log(`User deleted: ${this.username}`);
  }

  // Custom methods
  private async sendWelcomeEmail() {
    // Implementation
  }

  private async createDefaultSettings() {
    // Implementation
  }

  private async deleteRelatedPosts() {
    // Implementation
  }
}
```

### Available Hook Types

| Hook              | Trigger                         | Usage                                   |
| ----------------- | ------------------------------- | --------------------------------------- |
| `@BeforeCreate()` | Before creating new record      | Validation, default values, preparation |
| `@AfterCreate()`  | After creating new record       | Notifications, related record creation  |
| `@BeforeUpdate()` | Before updating existing record | Validation, timestamp updates, logging  |
| `@AfterUpdate()`  | After updating existing record  | Cache invalidation, notifications       |
| `@BeforeDelete()` | Before deleting record          | Cleanup, validation, logging            |
| `@AfterDelete()`  | After deleting record           | Cleanup, notifications, cascade deletes |
| `@BeforeSave()`   | Before save (create or update)  | Common validation, timestamps           |
| `@AfterSave()`    | After save (create or update)   | Common post-processing                  |

## Validation

### Field Validation

```typescript
@Model({ scope: 'global', type: 'docstore' })
class User extends BaseModel {
  @Field({
    type: 'string',
    required: true,
    unique: true,
    minLength: 3,
    maxLength: 20,
    validate: (username: string) => /^[a-zA-Z0-9_]+$/.test(username),
    transform: (username: string) => username.toLowerCase(),
  })
  username: string;

  @Field({
    type: 'string',
    required: true,
    unique: true,
    validate: (email: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email),
    transform: (email: string) => email.toLowerCase(),
  })
  email: string;

  @Field({
    type: 'number',
    required: false,
    default: 0,
    validate: (score: number) => score >= 0 && score <= 1000,
  })
  score: number;

  @Field({
    type: 'array',
    required: false,
    default: [],
    validate: (tags: string[]) => tags.length <= 10,
  })
  tags: string[];
}
```

### Custom Validation Methods

```typescript
@Model({ scope: 'global', type: 'docstore' })
class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'string', required: true })
  email: string;

  @Field({ type: 'string', required: true })
  password: string;

  // Custom validation method
  async customValidation(): Promise<ValidationResult> {
    const errors: ValidationError[] = [];

    // Check username availability
    if (this.username) {
      const existing = await User.findOne({ username: this.username });
      if (existing && existing.id !== this.id) {
        errors.push(new ValidationError('username', this.username, 'already taken'));
      }
    }

    // Check email format and availability
    if (this.email) {
      const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
      if (!emailRegex.test(this.email)) {
        errors.push(new ValidationError('email', this.email, 'invalid format'));
      }

      const existing = await User.findOne({ email: this.email });
      if (existing && existing.id !== this.id) {
        errors.push(new ValidationError('email', this.email, 'already registered'));
      }
    }

    // Check password strength
    if (this.password && this.password.length < 8) {
      errors.push(new ValidationError('password', this.password, 'must be at least 8 characters'));
    }

    return {
      valid: errors.length === 0,
      errors,
    };
  }

  @BeforeCreate()
  @BeforeUpdate()
  async validateBeforeSave() {
    const result = await this.customValidation();
    if (!result.valid) {
      throw new ValidationError('model', this, 'custom validation failed', result.errors);
    }
  }
}
```

## Configuration Interfaces

### ModelConstructor Type

```typescript
type ModelConstructor<T extends BaseModel> = new () => T;
```

### Options Interfaces

```typescript
interface CreateOptions {
  validate?: boolean;
  skipHooks?: boolean;
  userId?: string; // For user-scoped models
}

interface FindOptions {
  with?: string[]; // Relationship names to eager load
  cache?: boolean | number; // Cache result
  userId?: string; // For user-scoped models
}

interface SaveOptions {
  validate?: boolean;
  skipHooks?: boolean;
  upsert?: boolean;
}

interface DeleteOptions {
  skipHooks?: boolean;
  cascade?: boolean; // Delete related records
}

interface ReloadOptions {
  with?: string[]; // Relationship names to eager load
}
```

### Validation Types

```typescript
interface ValidationResult {
  valid: boolean;
  errors: ValidationError[];
}

class ValidationError extends Error {
  constructor(
    public field: string,
    public value: any,
    public constraint: string,
    public details?: any,
  ) {
    super(`Validation failed for field '${field}': ${constraint}`);
  }
}
```

## Error Handling

### Common Errors

```typescript
// Validation errors
try {
  const user = await User.create({
    username: 'a', // Too short
    email: 'invalid',
  });
} catch (error) {
  if (error instanceof ValidationError) {
    console.log(`Field ${error.field} failed: ${error.constraint}`);
  }
}

// Not found errors
const user = await User.findById('non-existent-id');
if (!user) {
  throw new NotFoundError('User', 'non-existent-id');
}

// Database errors
try {
  await user.save();
} catch (error) {
  if (error instanceof DatabaseError) {
    console.log('Database operation failed:', error.message);
  }
}
```

## Complete Example

### Blog Post Model

```typescript
import {
  BaseModel,
  Model,
  Field,
  BelongsTo,
  HasMany,
  BeforeCreate,
  AfterCreate,
} from '@debros/network';
import { User } from './User';
import { Comment } from './Comment';

@Model({
  scope: 'user',
  type: 'docstore',
  sharding: {
    strategy: 'user',
    count: 2,
    key: 'authorId',
  },
})
export class Post extends BaseModel {
  @Field({
    type: 'string',
    required: true,
    minLength: 1,
    maxLength: 200,
  })
  title: string;

  @Field({
    type: 'string',
    required: true,
    minLength: 1,
    maxLength: 10000,
  })
  content: string;

  @Field({ type: 'string', required: true })
  authorId: string;

  @Field({
    type: 'array',
    required: false,
    default: [],
    transform: (tags: string[]) => tags.map((tag) => tag.toLowerCase()),
  })
  tags: string[];

  @Field({ type: 'boolean', required: false, default: false })
  isPublished: boolean;

  @Field({ type: 'number', required: false, default: 0 })
  viewCount: number;

  @Field({ type: 'number', required: false })
  publishedAt?: number;

  // Relationships
  @BelongsTo(() => User, 'authorId')
  author: User;

  @HasMany(() => Comment, 'postId')
  comments: Comment[];

  @BeforeCreate()
  setupNewPost() {
    this.createdAt = Date.now();
    this.updatedAt = Date.now();
    this.viewCount = 0;
  }

  @AfterCreate()
  async afterPostCreated() {
    console.log(`New post created: ${this.title}`);

    // Update author's post count
    const author = await User.findById(this.authorId);
    if (author) {
      author.postCount = (author.postCount || 0) + 1;
      await author.save();
    }
  }

  // Custom methods
  async publish(): Promise<void> {
    this.isPublished = true;
    this.publishedAt = Date.now();
    await this.save();
  }

  async incrementViews(): Promise<void> {
    this.viewCount += 1;
    await this.save({ skipHooks: true }); // Skip hooks for performance
  }

  async getCommentCount(): Promise<number> {
    return await Comment.query().where('postId', this.id).count();
  }

  async getTopComments(limit: number = 5): Promise<Comment[]> {
    return await Comment.query()
      .where('postId', this.id)
      .orderBy('likeCount', 'desc')
      .limit(limit)
      .with(['author'])
      .find();
  }
}

// Usage examples
async function blogExamples() {
  // Create a post
  const post = await Post.create({
    title: 'My First Post',
    content: 'This is the content of my first post...',
    authorId: 'user123',
    tags: ['JavaScript', 'Web Development'],
  });

  // Find posts by author
  const userPosts = await Post.query()
    .where('authorId', 'user123')
    .where('isPublished', true)
    .orderBy('publishedAt', 'desc')
    .with(['author', 'comments.author'])
    .find();

  // Publish a post
  await post.publish();

  // Get post with comments
  const postWithComments = await Post.findById(post.id, {
    with: ['author', 'comments.author'],
  });

  console.log('Post:', postWithComments?.title);
  console.log('Author:', postWithComments?.author.username);
  console.log('Comments:', postWithComments?.comments.length);
}
```

This comprehensive BaseModel documentation covers all the essential functionality for working with models in Debros Network, including CRUD operations, validation, relationships, hooks, and real-world usage examples.
