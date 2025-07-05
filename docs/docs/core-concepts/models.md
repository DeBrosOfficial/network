---
sidebar_position: 2
---

# Models and Fields

Models are the foundation of DebrosFramework applications. They define your data structure, validation rules, and behavior using TypeScript classes and decorators. This guide covers everything you need to know about creating and working with models.

## Basic Model Structure

### Creating a Model

Every model in DebrosFramework extends the `BaseModel` class and uses the `@Model` decorator:

```typescript
import { BaseModel, Model, Field } from 'debros-framework';

@Model({
  scope: 'global',
  type: 'docstore',
})
export class User extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  username: string;

  @Field({ type: 'string', required: true })
  email: string;
}
```

### Model Configuration Options

The `@Model` decorator accepts several configuration options:

```typescript
@Model({
  // Database scope: 'user' or 'global'
  scope: 'user',

  // OrbitDB store type
  type: 'docstore', // 'docstore' | 'eventlog' | 'keyvalue' | 'counter' | 'feed'

  // Sharding configuration
  sharding: {
    strategy: 'hash', // 'hash' | 'range' | 'user'
    count: 4, // Number of shards
    key: 'userId', // Field to use for sharding
  },

  // Pinning configuration
  pinning: {
    strategy: 'popularity', // 'fixed' | 'popularity' | 'tiered'
    factor: 2, // Pinning factor
  },

  // PubSub configuration
  pubsub: {
    publishEvents: ['create', 'update', 'delete'],
  },

  // Validation configuration
  validation: {
    strict: true, // Strict validation mode
    allowExtraFields: false,
  },
})
export class Post extends BaseModel {
  // Model fields go here
}
```

## Field Types and Validation

### Basic Field Types

DebrosFramework supports several field types with built-in validation:

```typescript
export class ExampleModel extends BaseModel {
  @Field({ type: 'string', required: true })
  name: string;

  @Field({ type: 'number', required: true, min: 0, max: 100 })
  score: number;

  @Field({ type: 'boolean', required: false, default: false })
  isActive: boolean;

  @Field({ type: 'array', required: false, default: [] })
  tags: string[];

  @Field({ type: 'object', required: false })
  metadata: Record<string, any>;

  @Field({ type: 'date', required: false, default: () => new Date() })
  createdAt: Date;
}
```

### Field Configuration Options

Each field can be configured with various options:

```typescript
@Field({
  // Basic type information
  type: 'string',
  required: true,
  unique: false,

  // Default values
  default: 'default-value',
  default: () => Date.now(), // Function for dynamic defaults

  // Validation constraints
  min: 0,                    // Minimum value (numbers)
  max: 100,                  // Maximum value (numbers)
  minLength: 3,              // Minimum length (strings/arrays)
  maxLength: 50,             // Maximum length (strings/arrays)
  pattern: /^[a-zA-Z0-9]+$/, // Regex pattern (strings)

  // Custom validation
  validate: (value: any) => {
    return value.length >= 3 && value.length <= 20;
  },

  // Field transformation
  transform: (value: any) => value.toLowerCase(),

  // Serialization options
  serialize: true,           // Include in serialization

  // Indexing (for query optimization)
  index: true
})
fieldName: string;
```

### Custom Validation

You can implement complex validation logic using custom validators:

```typescript
export class User extends BaseModel {
  @Field({
    type: 'string',
    required: true,
    validate: (value: string) => {
      // Username validation
      if (value.length < 3 || value.length > 20) {
        throw new Error('Username must be between 3 and 20 characters');
      }

      if (!/^[a-zA-Z0-9_]+$/.test(value)) {
        throw new Error('Username can only contain letters, numbers, and underscores');
      }

      return true;
    },
  })
  username: string;

  @Field({
    type: 'string',
    required: true,
    validate: (value: string) => {
      // Email validation
      const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
      if (!emailRegex.test(value)) {
        throw new Error('Invalid email format');
      }
      return true;
    },
  })
  email: string;

  @Field({
    type: 'number',
    required: false,
    validate: (value: number) => {
      // Age validation
      if (value < 13 || value > 120) {
        throw new Error('Age must be between 13 and 120');
      }
      return true;
    },
  })
  age?: number;
}
```

### Field Transformation

Transform field values before storage or after retrieval:

```typescript
export class User extends BaseModel {
  @Field({
    type: 'string',
    required: true,
    transform: (value: string) => value.toLowerCase().trim(),
  })
  username: string;

  @Field({
    type: 'string',
    required: true,
    transform: (value: string) => value.toLowerCase(),
  })
  email: string;

  @Field({
    type: 'array',
    required: false,
    default: [],
    transform: (tags: string[]) => {
      // Normalize and deduplicate tags
      return [...new Set(tags.map((tag) => tag.toLowerCase().trim()))];
    },
  })
  tags: string[];
}
```

## Model Scoping

### User-Scoped Models

User-scoped models create separate databases for each user, providing data isolation:

```typescript
@Model({
  scope: 'user', // Each user gets their own database
  type: 'docstore',
  sharding: {
    strategy: 'user',
    count: 2,
    key: 'userId',
  },
})
export class UserPost extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  userId: string; // Required for user-scoped models
}
```

### Global Models

Global models are shared across all users:

```typescript
@Model({
  scope: 'global', // Shared across all users
  type: 'docstore',
  sharding: {
    strategy: 'hash',
    count: 8,
    key: 'id',
  },
})
export class GlobalNews extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  category: string;

  @Field({ type: 'boolean', required: false, default: true })
  isPublished: boolean;
}
```

## Model Hooks

Use hooks to execute code at specific points in the model lifecycle:

```typescript
import {
  BeforeCreate,
  AfterCreate,
  BeforeUpdate,
  AfterUpdate,
  BeforeDelete,
} from 'debros-framework';

export class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'string', required: true })
  passwordHash: string;

  @Field({ type: 'number', required: false })
  createdAt: number;

  @Field({ type: 'number', required: false })
  updatedAt: number;

  @BeforeCreate()
  async beforeCreate() {
    this.createdAt = Date.now();
    this.updatedAt = Date.now();

    // Hash password before saving
    if (this.passwordHash && !this.passwordHash.startsWith('$2b$')) {
      this.passwordHash = await this.hashPassword(this.passwordHash);
    }
  }

  @BeforeUpdate()
  async beforeUpdate() {
    this.updatedAt = Date.now();

    // Hash password if it was changed
    if (this.isFieldModified('passwordHash') && !this.passwordHash.startsWith('$2b$')) {
      this.passwordHash = await this.hashPassword(this.passwordHash);
    }
  }

  @AfterCreate()
  async afterCreate() {
    // Send welcome email
    await this.sendWelcomeEmail();
  }

  @BeforeDelete()
  async beforeDelete() {
    // Clean up user's data
    await this.cleanupUserData();
  }

  private async hashPassword(password: string): Promise<string> {
    // Implementation of password hashing
    const bcrypt = require('bcrypt');
    return await bcrypt.hash(password, 10);
  }

  private async sendWelcomeEmail(): Promise<void> {
    // Implementation of welcome email
    console.log(`Welcome email sent to ${this.username}`);
  }

  private async cleanupUserData(): Promise<void> {
    // Implementation of data cleanup
    console.log(`Cleaning up data for user ${this.username}`);
  }
}
```

## Advanced Model Features

### Computed Properties

Create computed properties that are automatically calculated:

```typescript
export class User extends BaseModel {
  @Field({ type: 'string', required: true })
  firstName: string;

  @Field({ type: 'string', required: true })
  lastName: string;

  @Field({ type: 'string', required: true })
  email: string;

  // Computed property
  get fullName(): string {
    return `${this.firstName} ${this.lastName}`;
  }

  get emailDomain(): string {
    return this.email.split('@')[1];
  }

  // Virtual field (not stored but serialized)
  @Field({ type: 'string', virtual: true })
  get displayName(): string {
    return this.fullName || this.email.split('@')[0];
  }
}
```

### Model Methods

Add custom methods to your models:

```typescript
export class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'array', required: false, default: [] })
  tags: string[];

  @Field({ type: 'number', required: false, default: 0 })
  viewCount: number;

  // Instance methods
  async incrementViews(): Promise<void> {
    this.viewCount += 1;
    await this.save();
  }

  addTag(tag: string): void {
    if (!this.tags.includes(tag)) {
      this.tags.push(tag);
    }
  }

  removeTag(tag: string): void {
    this.tags = this.tags.filter((t) => t !== tag);
  }

  getWordCount(): number {
    return this.content.split(/\s+/).length;
  }

  // Static methods
  static async findByTag(tag: string): Promise<Post[]> {
    return await this.query().where('tags', 'includes', tag).find();
  }

  static async findPopular(limit: number = 10): Promise<Post[]> {
    return await this.query().orderBy('viewCount', 'desc').limit(limit).find();
  }
}
```

### Model Serialization

Control how models are serialized:

```typescript
export class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'string', required: true })
  email: string;

  @Field({
    type: 'string',
    required: true,
    serialize: false, // Don't include in serialization
  })
  passwordHash: string;

  @Field({ type: 'string', required: false })
  profilePicture?: string;

  // Custom serialization
  toJSON(): any {
    const json = super.toJSON();

    // Add computed fields
    json.initials = this.getInitials();

    // Remove sensitive data
    delete json.passwordHash;

    return json;
  }

  // Safe serialization for public APIs
  toPublic(): any {
    return {
      id: this.id,
      username: this.username,
      profilePicture: this.profilePicture,
      initials: this.getInitials(),
    };
  }

  private getInitials(): string {
    return this.username.substring(0, 2).toUpperCase();
  }
}
```

## Model Inheritance

Create base models for common functionality:

```typescript
// Base model with common fields
abstract class TimestampedModel extends BaseModel {
  @Field({ type: 'number', required: false, default: () => Date.now() })
  createdAt: number;

  @Field({ type: 'number', required: false, default: () => Date.now() })
  updatedAt: number;

  @BeforeUpdate()
  updateTimestamp() {
    this.updatedAt = Date.now();
  }
}

// User model extending base
@Model({ scope: 'global', type: 'docstore' })
export class User extends TimestampedModel {
  @Field({ type: 'string', required: true, unique: true })
  username: string;

  @Field({ type: 'string', required: true, unique: true })
  email: string;
}

// Post model extending base
@Model({ scope: 'user', type: 'docstore' })
export class Post extends TimestampedModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  userId: string;
}
```

## Best Practices

### Model Design

1. **Use appropriate scoping**: Choose 'user' or 'global' scope based on your data access patterns
2. **Design for sharding**: Consider how your data will be distributed when choosing sharding keys
3. **Validate early**: Use field validation to catch errors early in the development process
4. **Use TypeScript**: Take advantage of TypeScript's type safety throughout your models

### Performance Optimization

1. **Index frequently queried fields**: Add indexes to fields you query often
2. **Use computed properties sparingly**: Heavy computations can impact performance
3. **Optimize serialization**: Only serialize the data you need
4. **Consider caching**: Use caching for expensive operations

### Security Considerations

1. **Validate all input**: Never trust user input without validation
2. **Sanitize data**: Clean data before storage
3. **Control serialization**: Be careful about what data you expose in APIs
4. **Use appropriate scoping**: User-scoped models provide better data isolation

### Code Organization

1. **Keep models focused**: Each model should have a single responsibility
2. **Use inheritance wisely**: Create base models for common functionality
3. **Document your models**: Use clear names and add comments for complex logic
4. **Test thoroughly**: Write comprehensive tests for your model logic

This comprehensive model system provides the foundation for building scalable, maintainable decentralized applications with DebrosFramework.
