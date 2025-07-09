---
sidebar_position: 1
---

# @Model Decorator

The `@Model` decorator is used to define model configuration and metadata in DebrosFramework.

## Overview

The `@Model` decorator configures how a model class should be handled by the framework, including database scope, store type, sharding configuration, and other model-specific options.

## Syntax

```typescript
@Model(config: ModelConfig)
class ModelName extends BaseModel {
  // Model definition
}
```

## Configuration Options

### ModelConfig Interface

```typescript
interface ModelConfig {
  scope: 'user' | 'global';
  type: StoreType;
  sharding?: ShardingConfig;
  pinning?: PinningConfig;
  pubsub?: PubSubConfig;
  validation?: ValidationConfig;
}
```

### Basic Usage

```typescript
@Model({
  scope: 'global',
  type: 'docstore'
})
class User extends BaseModel {
  // Model properties
}
```

## Configuration Properties

### scope

Determines the database scope for the model.

- `'user'` - Each user gets their own database instance
- `'global'` - Single shared database for all users

### type

Specifies the OrbitDB store type.

- `'docstore'` - Document-based storage
- `'eventlog'` - Append-only event log
- `'keyvalue'` - Key-value storage
- `'counter'` - Counter storage
- `'feed'` - Feed storage

### sharding (optional)

Configures data sharding for the model.

```typescript
@Model({
  scope: 'global',
  type: 'docstore',
  sharding: {
    strategy: 'hash',
    count: 4,
    key: 'id'
  }
})
class ShardedModel extends BaseModel {
  // Model definition
}
```

## Examples

### User-Scoped Model

```typescript
@Model({
  scope: 'user',
  type: 'docstore'
})
class UserPost extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;
}
```

### Global Model with Sharding

```typescript
@Model({
  scope: 'global',
  type: 'docstore',
  sharding: {
    strategy: 'hash',
    count: 8,
    key: 'userId'
  }
})
class GlobalPost extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  userId: string;
}
```

## Related Decorators

- [`@Field`](./field) - Field configuration
- [`@BelongsTo`](./relationships#belongsto) - Relationship decorators
- [`@HasMany`](./relationships#hasmany) - Relationship decorators
