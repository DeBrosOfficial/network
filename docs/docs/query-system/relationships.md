---
sidebar_position: 2
---

# Relationships

DebrosFramework provides powerful relationship management capabilities for connecting models.

## Overview

Relationships define how models are connected to each other and provide mechanisms for loading related data efficiently.

## Relationship Types

### BelongsTo

A many-to-one relationship where the current model belongs to another model.

```typescript
class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  userId: string;

  @BelongsTo(() => User, 'userId')
  user: User;
}
```

### HasMany

A one-to-many relationship where the current model has many related models.

```typescript
class User extends BaseModel {
  @HasMany(() => Post, 'userId')
  posts: Post[];
}
```

### HasOne

A one-to-one relationship where the current model has one related model.

```typescript
class User extends BaseModel {
  @HasOne(() => UserProfile, 'userId')
  profile: UserProfile;
}
```

### ManyToMany

A many-to-many relationship using a pivot table.

```typescript
class User extends BaseModel {
  @ManyToMany(() => Role, 'user_roles')
  roles: Role[];
}
```

## Loading Relationships

### Eager Loading

Load relationships when querying:

```typescript
const usersWithPosts = await User.query()
  .with(['posts'])
  .find();
```

### Lazy Loading

Load relationships on-demand:

```typescript
const user = await User.findById('user123');
const posts = await user.loadRelationship('posts');
```

## Nested Relationships

Load nested relationships:

```typescript
const usersWithPostsAndComments = await User.query()
  .with(['posts.comments', 'posts.comments.user'])
  .find();
```

## Related Classes

- [`RelationshipManager`](../api/relationship-manager) - Manages relationships
- [`BaseModel`](../api/base-model) - Base model class
