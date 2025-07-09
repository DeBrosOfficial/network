---
sidebar_position: 4
---

# Hook Decorators

DebrosFramework provides hook decorators for defining lifecycle methods that are called at specific points during model operations.

## Overview

Hook decorators allow you to define methods that are automatically called before or after specific model operations like create, update, delete, and save.

## Hook Types

### @BeforeCreate {#beforecreate}

Called before a model instance is created.

```typescript
@BeforeCreate
async beforeCreate(): Promise<void> {
  // Pre-creation logic
}
```

**Example:**
```typescript
class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @BeforeCreate
  async beforeCreate(): Promise<void> {
    this.username = this.username.toLowerCase();
  }
}
```

### @AfterCreate {#aftercreate}

Called after a model instance is created.

```typescript
@AfterCreate
async afterCreate(): Promise<void> {
  // Post-creation logic
}
```

**Example:**
```typescript
class User extends BaseModel {
  @AfterCreate
  async afterCreate(): Promise<void> {
    console.log(`User ${this.username} created successfully`);
  }
}
```

### @BeforeUpdate {#beforeupdate}

Called before a model instance is updated.

```typescript
@BeforeUpdate
async beforeUpdate(): Promise<void> {
  // Pre-update logic
}
```

**Example:**
```typescript
class Post extends BaseModel {
  @Field({ type: 'number' })
  updatedAt: number;

  @BeforeUpdate
  async beforeUpdate(): Promise<void> {
    this.updatedAt = Date.now();
  }
}
```

### @AfterUpdate {#afterupdate}

Called after a model instance is updated.

```typescript
@AfterUpdate
async afterUpdate(): Promise<void> {
  // Post-update logic
}
```

### @BeforeDelete {#beforedelete}

Called before a model instance is deleted.

```typescript
@BeforeDelete
async beforeDelete(): Promise<void> {
  // Pre-deletion logic
}
```

**Example:**
```typescript
class User extends BaseModel {
  @BeforeDelete
  async beforeDelete(): Promise<void> {
    // Clean up related data
    await Post.query().where('userId', this.id).delete();
  }
}
```

### @AfterDelete {#afterdelete}

Called after a model instance is deleted.

```typescript
@AfterDelete
async afterDelete(): Promise<void> {
  // Post-deletion logic
}
```

### @BeforeSave {#beforesave}

Called before a model instance is saved (create or update).

```typescript
@BeforeSave
async beforeSave(): Promise<void> {
  // Pre-save logic
}
```

**Example:**
```typescript
class Post extends BaseModel {
  @Field({ type: 'string' })
  slug: string;

  @Field({ type: 'string' })
  title: string;

  @BeforeSave
  async beforeSave(): Promise<void> {
    if (!this.slug) {
      this.slug = this.title.toLowerCase().replace(/\s+/g, '-');
    }
  }
}
```

### @AfterSave {#aftersave}

Called after a model instance is saved (create or update).

```typescript
@AfterSave
async afterSave(): Promise<void> {
  // Post-save logic
}
```

## Usage Examples

### Complete User Model with Hooks

```typescript
class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'string', required: true })
  email: string;

  @Field({ type: 'number' })
  createdAt: number;

  @Field({ type: 'number' })
  updatedAt: number;

  @BeforeCreate
  async beforeCreate(): Promise<void> {
    this.username = this.username.toLowerCase().trim();
    this.email = this.email.toLowerCase().trim();
    this.createdAt = Date.now();
  }

  @BeforeUpdate
  async beforeUpdate(): Promise<void> {
    this.updatedAt = Date.now();
  }

  @BeforeSave
  async beforeSave(): Promise<void> {
    // Validate email format
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(this.email)) {
      throw new Error('Invalid email format');
    }
  }

  @AfterSave
  async afterSave(): Promise<void> {
    console.log(`User ${this.username} saved at ${new Date()}`);
  }

  @BeforeDelete
  async beforeDelete(): Promise<void> {
    // Clean up related data
    await Post.query().where('userId', this.id).delete();
    await Comment.query().where('userId', this.id).delete();
  }

  @AfterDelete
  async afterDelete(): Promise<void> {
    console.log(`User ${this.username} and related data deleted`);
  }
}
```

## Related Classes

- [`BaseModel`](../base-model) - Base model class
- [`@Model`](./model) - Model configuration
- [`@Field`](./field) - Field configuration
