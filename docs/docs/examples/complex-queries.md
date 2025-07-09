---
sidebar_position: 2
---

# Complex Queries

This example demonstrates advanced query patterns and techniques in DebrosFramework.

## Overview

Learn how to build complex queries using DebrosFramework's powerful query system.

## Query Examples

### Basic Filtering

```typescript
// Simple where clause
const activeUsers = await User.query()
  .where('isActive', true)
  .find();

// Multiple conditions
const recentPosts = await Post.query()
  .where('createdAt', '>', Date.now() - 7 * 24 * 60 * 60 * 1000)
  .where('isPublished', true)
  .find();
```

### Advanced Filtering

```typescript
// OR conditions
const popularPosts = await Post.query()
  .where('viewCount', '>', 1000)
  .orWhere('likeCount', '>', 100)
  .find();

// IN operator
const categorizedPosts = await Post.query()
  .where('category', 'in', ['tech', 'science', 'programming'])
  .find();

// LIKE operator for text search
const searchResults = await Post.query()
  .where('title', 'like', '%javascript%')
  .find();
```

### Sorting and Pagination

```typescript
// Sorting
const sortedPosts = await Post.query()
  .orderBy('createdAt', 'desc')
  .orderBy('title', 'asc')
  .find();

// Pagination
const paginatedPosts = await Post.query()
  .orderBy('createdAt', 'desc')
  .limit(10)
  .offset(20)
  .find();
```

### Relationship Loading

```typescript
// Eager loading
const usersWithPosts = await User.query()
  .with(['posts'])
  .find();

// Nested relationships
const usersWithPostsAndComments = await User.query()
  .with(['posts.comments', 'posts.comments.user'])
  .find();

// Conditional relationship loading
const activeUsersWithRecentPosts = await User.query()
  .where('isActive', true)
  .with(['posts'], (query) => 
    query.where('createdAt', '>', Date.now() - 30 * 24 * 60 * 60 * 1000)
  )
  .find();
```

### Aggregation

```typescript
// Count
const userCount = await User.query()
  .where('isActive', true)
  .count();

// Group by
const postsByCategory = await Post.query()
  .select('category', 'COUNT(*) as count')
  .groupBy('category')
  .find();
```

### Complex Joins

```typescript
// Manual join
const usersWithPostCount = await User.query()
  .leftJoin('posts', 'users.id', 'posts.userId')
  .select('users.username', 'COUNT(posts.id) as postCount')
  .groupBy('users.id', 'users.username')
  .find();
```

### Caching

```typescript
// Cache for 5 minutes
const cachedPosts = await Post.query()
  .where('isPublished', true)
  .cache(300)
  .find();

// Disable caching
const freshPosts = await Post.query()
  .where('isPublished', true)
  .cache(false)
  .find();
```

## Performance Optimization

### Query Optimization

```typescript
// Use indexes
const optimizedQuery = await Post.query()
  .where('userId', userId) // Indexed field
  .where('createdAt', '>', startDate)
  .orderBy('createdAt', 'desc')
  .limit(50)
  .find();

// Batch operations
const userIds = ['user1', 'user2', 'user3'];
const users = await User.query()
  .where('id', 'in', userIds)
  .find();
```

### Parallel Queries

```typescript
// Execute queries in parallel
const [users, posts, comments] = await Promise.all([
  User.query().where('isActive', true).find(),
  Post.query().where('isPublished', true).find(),
  Comment.query().where('isModerated', true).find()
]);
```

## Related Topics

- [Query Builder](../query-system/query-builder) - Query construction
- [Relationships](../query-system/relationships) - Model relationships
- [Performance](../advanced/performance) - Performance optimization
