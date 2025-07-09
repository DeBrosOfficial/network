---
sidebar_position: 3
---

# Relationship Decorators

DebrosFramework provides decorators for defining relationships between models.

## Overview

Relationship decorators define how models are connected to each other. They support various relationship types including one-to-one, one-to-many, and many-to-many relationships.

## Relationship Types

### @BelongsTo {#belongsto}

Defines a many-to-one relationship.

```typescript
@BelongsTo(relatedModel: () => ModelClass, foreignKey: string)
```

**Example:**
```typescript
class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  userId: string;

  @BelongsTo(() => User, 'userId')
  user: User;
}
```

### @HasMany {#hasmany}

Defines a one-to-many relationship.

```typescript
@HasMany(relatedModel: () => ModelClass, foreignKey: string)
```

**Example:**
```typescript
class User extends BaseModel {
  @HasMany(() => Post, 'userId')
  posts: Post[];
}
```

### @HasOne {#hasone}

Defines a one-to-one relationship.

```typescript
@HasOne(relatedModel: () => ModelClass, foreignKey: string)
```

**Example:**
```typescript
class User extends BaseModel {
  @HasOne(() => UserProfile, 'userId')
  profile: UserProfile;
}
```

### @ManyToMany {#manytomany}

Defines a many-to-many relationship.

```typescript
@ManyToMany(relatedModel: () => ModelClass, through: string)
```

**Example:**
```typescript
class User extends BaseModel {
  @ManyToMany(() => Role, 'user_roles')
  roles: Role[];
}
```

## Usage Examples

### Blog System

```typescript
class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @HasMany(() => Post, 'userId')
  posts: Post[];

  @HasMany(() => Comment, 'userId')
  comments: Comment[];
}

class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  userId: string;

  @BelongsTo(() => User, 'userId')
  user: User;

  @HasMany(() => Comment, 'postId')
  comments: Comment[];
}

class Comment extends BaseModel {
  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  userId: string;

  @Field({ type: 'string', required: true })
  postId: string;

  @BelongsTo(() => User, 'userId')
  user: User;

  @BelongsTo(() => Post, 'postId')
  post: Post;
}
```

## Related Classes

- [`BaseModel`](../base-model) - Base model class
- [`RelationshipManager`](../relationship-manager) - Relationship management
