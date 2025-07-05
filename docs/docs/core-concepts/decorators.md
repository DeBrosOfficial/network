---
sidebar_position: 3
---

# Decorators Reference

DebrosFramework uses TypeScript decorators to provide a clean, declarative way to define models, fields, relationships, and hooks. This guide covers all available decorators and their usage patterns.

## Model Decorators

### @Model

The `@Model` decorator is used to mark a class as a DebrosFramework model and configure its behavior.

```typescript
import { BaseModel, Model } from 'debros-framework';

@Model({
  scope: 'global',
  type: 'docstore',
  sharding: {
    strategy: 'hash',
    count: 4,
    key: 'id',
  },
  pinning: {
    strategy: 'popularity',
    factor: 2,
  },
  pubsub: {
    publishEvents: ['create', 'update'],
  },
})
export class User extends BaseModel {
  // Model definition
}
```

#### Configuration Options

| Option       | Type                 | Description              | Default      |
| ------------ | -------------------- | ------------------------ | ------------ |
| `scope`      | `'user' \| 'global'` | Database scope           | `'global'`   |
| `type`       | `StoreType`          | OrbitDB store type       | `'docstore'` |
| `sharding`   | `ShardingConfig`     | Sharding configuration   | `undefined`  |
| `pinning`    | `PinningConfig`      | Pinning configuration    | `undefined`  |
| `pubsub`     | `PubSubConfig`       | PubSub configuration     | `undefined`  |
| `validation` | `ValidationConfig`   | Validation configuration | `undefined`  |

#### Store Types

```typescript
type StoreType = 'docstore' | 'eventlog' | 'keyvalue' | 'counter' | 'feed';

// Examples of different store types
@Model({ type: 'docstore' }) // Document storage (most common)
class Document extends BaseModel {}

@Model({ type: 'eventlog' }) // Event log storage
class Event extends BaseModel {}

@Model({ type: 'keyvalue' }) // Key-value storage
class Setting extends BaseModel {}

@Model({ type: 'counter' }) // Counter storage
class Counter extends BaseModel {}

@Model({ type: 'feed' }) // Feed storage
class FeedItem extends BaseModel {}
```

#### Sharding Configuration

```typescript
interface ShardingConfig {
  strategy: 'hash' | 'range' | 'user';
  count: number;
  key: string;
  ranges?: Array<{ min: any; max: any; shard: number }>;
}

// Hash-based sharding
@Model({
  sharding: {
    strategy: 'hash',
    count: 8,
    key: 'userId', // Shard based on userId hash
  },
})
class UserPost extends BaseModel {}

// Range-based sharding
@Model({
  sharding: {
    strategy: 'range',
    count: 4,
    key: 'createdAt',
    ranges: [
      { min: 0, max: Date.now() - 365 * 24 * 60 * 60 * 1000, shard: 0 }, // Old data
      {
        min: Date.now() - 365 * 24 * 60 * 60 * 1000,
        max: Date.now() - 30 * 24 * 60 * 60 * 1000,
        shard: 1,
      }, // Medium data
      { min: Date.now() - 30 * 24 * 60 * 60 * 1000, max: Date.now(), shard: 2 }, // Recent data
      { min: Date.now(), max: Infinity, shard: 3 }, // Future data
    ],
  },
})
class TimeBasedData extends BaseModel {}

// User-based sharding
@Model({
  sharding: {
    strategy: 'user',
    count: 1, // One shard per user
    key: 'userId',
  },
})
class PrivateUserData extends BaseModel {}
```

## Field Decorators

### @Field

The `@Field` decorator defines model fields with validation, transformation, and serialization options.

```typescript
import { Field } from 'debros-framework';

export class User extends BaseModel {
  @Field({
    type: 'string',
    required: true,
    unique: true,
    minLength: 3,
    maxLength: 20,
    pattern: /^[a-zA-Z0-9_]+$/,
    validate: (value: string) => value.toLowerCase() !== 'admin',
    transform: (value: string) => value.toLowerCase(),
    index: true,
  })
  username: string;
}
```

#### Field Types

```typescript
// String fields
@Field({ type: 'string', required: true })
name: string;

@Field({
  type: 'string',
  required: false,
  default: 'default-value',
  minLength: 3,
  maxLength: 100,
  pattern: /^[a-zA-Z\s]+$/
})
description?: string;

// Number fields
@Field({
  type: 'number',
  required: true,
  min: 0,
  max: 100
})
score: number;

@Field({
  type: 'number',
  required: false,
  default: () => Date.now()
})
timestamp?: number;

// Boolean fields
@Field({
  type: 'boolean',
  required: false,
  default: false
})
isActive: boolean;

// Array fields
@Field({
  type: 'array',
  required: false,
  default: [],
  maxLength: 10
})
tags: string[];

// Object fields
@Field({
  type: 'object',
  required: false,
  default: {}
})
metadata: Record<string, any>;

// Date fields
@Field({
  type: 'date',
  required: false,
  default: () => new Date()
})
createdAt: Date;
```

#### Field Configuration Options

| Option      | Type              | Description                   | Applies To      |
| ----------- | ----------------- | ----------------------------- | --------------- |
| `type`      | `FieldType`       | Field data type               | All             |
| `required`  | `boolean`         | Whether field is required     | All             |
| `unique`    | `boolean`         | Whether field must be unique  | All             |
| `default`   | `any \| Function` | Default value or function     | All             |
| `min`       | `number`          | Minimum value                 | Numbers         |
| `max`       | `number`          | Maximum value                 | Numbers         |
| `minLength` | `number`          | Minimum length                | Strings, Arrays |
| `maxLength` | `number`          | Maximum length                | Strings, Arrays |
| `pattern`   | `RegExp`          | Validation pattern            | Strings         |
| `validate`  | `Function`        | Custom validation function    | All             |
| `transform` | `Function`        | Value transformation function | All             |
| `serialize` | `boolean`         | Include in serialization      | All             |
| `index`     | `boolean`         | Create index for queries      | All             |
| `virtual`   | `boolean`         | Virtual field (computed)      | All             |

#### Advanced Field Examples

```typescript
export class User extends BaseModel {
  // Email with validation
  @Field({
    type: 'string',
    required: true,
    unique: true,
    validate: (email: string) => {
      const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
      if (!emailRegex.test(email)) {
        throw new Error('Invalid email format');
      }
      return true;
    },
    transform: (email: string) => email.toLowerCase(),
    index: true,
  })
  email: string;

  // Password with hashing
  @Field({
    type: 'string',
    required: true,
    serialize: false, // Don't include in JSON
    validate: (password: string) => {
      if (password.length < 8) {
        throw new Error('Password must be at least 8 characters');
      }
      return true;
    },
  })
  passwordHash: string;

  // Tags with normalization
  @Field({
    type: 'array',
    required: false,
    default: [],
    maxLength: 10,
    transform: (tags: string[]) => {
      // Normalize and deduplicate tags
      return [...new Set(tags.map((tag) => tag.toLowerCase().trim()))].filter(
        (tag) => tag.length > 0,
      );
    },
  })
  tags: string[];

  // Virtual computed field
  @Field({
    type: 'string',
    virtual: true,
  })
  get emailDomain(): string {
    return this.email.split('@')[1];
  }

  // Score with validation
  @Field({
    type: 'number',
    required: false,
    default: 0,
    min: 0,
    max: 1000,
    validate: (score: number) => {
      if (score % 1 !== 0) {
        throw new Error('Score must be a whole number');
      }
      return true;
    },
  })
  score: number;
}
```

## Relationship Decorators

### @BelongsTo

Defines a many-to-one relationship where this model belongs to another model.

```typescript
import { BelongsTo } from 'debros-framework';

@Model({ scope: 'user' })
export class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  userId: string;

  // Post belongs to a User
  @BelongsTo(() => User, 'userId')
  user: User;

  // Post belongs to a Category (with options)
  @BelongsTo(() => Category, 'categoryId', {
    cache: true,
    eager: false,
  })
  category: Category;
}
```

### @HasMany

Defines a one-to-many relationship where this model has many of another model.

```typescript
import { HasMany } from 'debros-framework';

@Model({ scope: 'global' })
export class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  // User has many Posts
  @HasMany(() => Post, 'userId')
  posts: Post[];

  // User has many Comments (with options)
  @HasMany(() => Comment, 'userId', {
    cache: true,
    eager: false,
    orderBy: 'createdAt',
    limit: 100,
  })
  comments: Comment[];
}
```

### @HasOne

Defines a one-to-one relationship where this model has one of another model.

```typescript
import { HasOne } from 'debros-framework';

@Model({ scope: 'global' })
export class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  // User has one Profile
  @HasOne(() => UserProfile, 'userId')
  profile: UserProfile;

  // User has one Setting (with options)
  @HasOne(() => UserSetting, 'userId', {
    cache: true,
    eager: true,
  })
  settings: UserSetting;
}
```

### @ManyToMany

Defines a many-to-many relationship through a join table or field.

```typescript
import { ManyToMany } from 'debros-framework';

@Model({ scope: 'global' })
export class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  // Many-to-many through join table
  @ManyToMany(() => Role, 'user_roles', 'userId', 'roleId')
  roles: Role[];

  // Many-to-many through array field
  @ManyToMany(() => Tag, 'userTags', {
    through: 'tagIds', // Array field in this model
    cache: true,
  })
  tags: Tag[];

  @Field({ type: 'array', required: false, default: [] })
  tagIds: string[]; // Array of tag IDs
}
```

#### Relationship Configuration Options

| Option    | Type      | Description                     | Default     |
| --------- | --------- | ------------------------------- | ----------- |
| `cache`   | `boolean` | Cache relationship data         | `false`     |
| `eager`   | `boolean` | Load relationship eagerly       | `false`     |
| `orderBy` | `string`  | Order related records by field  | `undefined` |
| `limit`   | `number`  | Limit number of related records | `undefined` |
| `where`   | `object`  | Additional where conditions     | `undefined` |

#### Advanced Relationship Examples

```typescript
@Model({ scope: 'global' })
export class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  // Posts with caching and ordering
  @HasMany(() => Post, 'userId', {
    cache: true,
    orderBy: 'createdAt',
    limit: 50,
    where: { isPublished: true },
  })
  publishedPosts: Post[];

  // Recent posts
  @HasMany(() => Post, 'userId', {
    orderBy: 'createdAt',
    limit: 10,
    where: {
      createdAt: { $gt: Date.now() - 30 * 24 * 60 * 60 * 1000 },
    },
  })
  recentPosts: Post[];

  // Followers (many-to-many)
  @ManyToMany(() => User, 'user_follows', 'followingId', 'followerId')
  followers: User[];

  // Following (many-to-many)
  @ManyToMany(() => User, 'user_follows', 'followerId', 'followingId')
  following: User[];
}
```

## Hook Decorators

Hook decorators allow you to execute code at specific points in the model lifecycle.

### Lifecycle Hooks

```typescript
import {
  BeforeCreate,
  AfterCreate,
  BeforeUpdate,
  AfterUpdate,
  BeforeDelete,
  AfterDelete,
  BeforeSave,
  AfterSave,
} from 'debros-framework';

export class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'string', required: true })
  email: string;

  @Field({ type: 'number', required: false })
  createdAt: number;

  @Field({ type: 'number', required: false })
  updatedAt: number;

  // Before creating a new record
  @BeforeCreate()
  async beforeCreate() {
    this.createdAt = Date.now();
    this.updatedAt = Date.now();

    // Validate uniqueness
    const existing = await User.findOne({ email: this.email });
    if (existing) {
      throw new Error('Email already exists');
    }
  }

  // After creating a new record
  @AfterCreate()
  async afterCreate() {
    // Send welcome email
    await this.sendWelcomeEmail();

    // Create default settings
    await this.createDefaultSettings();
  }

  // Before updating a record
  @BeforeUpdate()
  async beforeUpdate() {
    this.updatedAt = Date.now();

    // Log the change
    console.log(`Updating user ${this.username}`);
  }

  // After updating a record
  @AfterUpdate()
  async afterUpdate() {
    // Invalidate cache
    await this.invalidateCache();
  }

  // Before deleting a record
  @BeforeDelete()
  async beforeDelete() {
    // Clean up related data
    await this.cleanupRelatedData();
  }

  // After deleting a record
  @AfterDelete()
  async afterDelete() {
    // Log deletion
    console.log(`User ${this.username} deleted`);
  }

  // Before any save operation (create or update)
  @BeforeSave()
  async beforeSave() {
    // Validate data
    await this.validateData();
  }

  // After any save operation (create or update)
  @AfterSave()
  async afterSave() {
    // Update search index
    await this.updateSearchIndex();
  }

  private async sendWelcomeEmail(): Promise<void> {
    // Implementation
  }

  private async createDefaultSettings(): Promise<void> {
    // Implementation
  }

  private async invalidateCache(): Promise<void> {
    // Implementation
  }

  private async cleanupRelatedData(): Promise<void> {
    // Implementation
  }

  private async validateData(): Promise<void> {
    // Implementation
  }

  private async updateSearchIndex(): Promise<void> {
    // Implementation
  }
}
```

### Hook Parameters

Some hooks can receive parameters with information about the operation:

```typescript
export class AuditedModel extends BaseModel {
  @BeforeUpdate()
  async beforeUpdate(changes: Record<string, any>) {
    // Log what fields are changing
    console.log('Fields changing:', Object.keys(changes));

    // Audit specific changes
    if (changes.status) {
      await this.auditStatusChange(changes.status);
    }
  }

  @AfterCreate()
  async afterCreate(model: this) {
    // The model instance is passed to after hooks
    console.log(`Created ${model.constructor.name} with ID ${model.id}`);
  }

  private async auditStatusChange(newStatus: string): Promise<void> {
    // Implementation
  }
}
```

## Decorator Best Practices

### TypeScript Configuration

Ensure your `tsconfig.json` has the required decorator settings:

```json
{
  "compilerOptions": {
    "experimentalDecorators": true,
    "emitDecoratorMetadata": true,
    "target": "ES2020",
    "module": "commonjs"
  }
}
```

### Performance Considerations

1. **Use caching wisely**: Only cache relationships that are accessed frequently
2. **Limit eager loading**: Eager loading can impact performance with large datasets
3. **Optimize hooks**: Keep hook operations lightweight and fast
4. **Index frequently queried fields**: Add indexes to improve query performance

### Code Organization

1. **Group related decorators**: Keep related decorators together
2. **Document complex logic**: Add comments for complex validation or transformation logic
3. **Use consistent naming**: Follow consistent naming conventions for fields and relationships
4. **Separate concerns**: Keep business logic in separate methods, not in decorators

### Error Handling

```typescript
export class User extends BaseModel {
  @Field({
    type: 'string',
    required: true,
    validate: (value: string) => {
      if (!value || value.trim().length === 0) {
        throw new Error('Username cannot be empty');
      }

      if (value.length < 3) {
        throw new Error('Username must be at least 3 characters long');
      }

      if (!/^[a-zA-Z0-9_]+$/.test(value)) {
        throw new Error('Username can only contain letters, numbers, and underscores');
      }

      return true;
    },
  })
  username: string;

  @BeforeCreate()
  async beforeCreate() {
    try {
      // Expensive validation
      await this.validateUniqueEmail();
    } catch (error) {
      throw new Error(`Validation failed: ${error.message}`);
    }
  }

  private async validateUniqueEmail(): Promise<void> {
    const existing = await User.findOne({ email: this.email });
    if (existing) {
      throw new Error('Email address is already in use');
    }
  }
}
```

This comprehensive decorator system provides a powerful, type-safe way to define your data models and their behavior in DebrosFramework applications.
