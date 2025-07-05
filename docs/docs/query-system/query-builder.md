---
sidebar_position: 1
---

# Query Builder

The DebrosFramework Query Builder provides a powerful, type-safe, and intuitive API for querying your decentralized data. It supports complex filtering, ordering, pagination, and relationship loading across distributed databases.

## Basic Query Syntax

### Simple Queries

```typescript
import { User, Post } from './models';

// Find all users
const allUsers = await User.query().find();

// Find a single user
const user = await User.query().findOne();

// Find user by ID
const userById = await User.findById('user_123');

// Count users
const userCount = await User.query().count();

// Check if any users exist
const hasUsers = await User.query().exists();
```

### Where Clauses

The query builder supports various where clause operators:

```typescript
// Basic equality
const activeUsers = await User.query().where('isActive', true).find();

// Comparison operators
const recentPosts = await Post.query()
  .where('createdAt', '>', Date.now() - 24 * 60 * 60 * 1000)
  .find();

const popularPosts = await Post.query().where('likeCount', '>=', 100).find();

// Multiple conditions (AND)
const recentPopularPosts = await Post.query()
  .where('createdAt', '>', Date.now() - 24 * 60 * 60 * 1000)
  .where('likeCount', '>=', 50)
  .where('isPublished', true)
  .find();

// IN operator
const specificUsers = await User.query().where('id', 'in', ['user_1', 'user_2', 'user_3']).find();

// NOT IN operator
const excludedUsers = await User.query().where('username', 'not in', ['admin', 'test']).find();

// LIKE operator (for strings)
const usersStartingWithA = await User.query().where('username', 'like', 'a%').find();

// Regular expressions
const emailUsers = await User.query()
  .where('email', 'regex', /@gmail\.com$/)
  .find();

// Null checks
const usersWithoutBio = await User.query().where('bio', 'is null').find();

const usersWithBio = await User.query().where('bio', 'is not null').find();

// Array operations
const techPosts = await Post.query().where('tags', 'includes', 'technology').find();

const multipleTags = await Post.query()
  .where('tags', 'includes any', ['tech', 'programming', 'code'])
  .find();

const allTags = await Post.query().where('tags', 'includes all', ['react', 'typescript']).find();
```

#### Supported Where Operators

| Operator       | Description               | Example                                      |
| -------------- | ------------------------- | -------------------------------------------- |
| `=` or `eq`    | Equal to                  | `.where('status', 'active')`                 |
| `!=` or `ne`   | Not equal to              | `.where('status', '!=', 'deleted')`          |
| `>` or `gt`    | Greater than              | `.where('score', '>', 100)`                  |
| `>=` or `gte`  | Greater than or equal     | `.where('score', '>=', 100)`                 |
| `<` or `lt`    | Less than                 | `.where('age', '<', 18)`                     |
| `<=` or `lte`  | Less than or equal        | `.where('age', '<=', 65)`                    |
| `in`           | In array                  | `.where('id', 'in', ['1', '2'])`             |
| `not in`       | Not in array              | `.where('status', 'not in', ['deleted'])`    |
| `like`         | Pattern matching          | `.where('name', 'like', 'John%')`            |
| `regex`        | Regular expression        | `.where('email', 'regex', /@gmail/)`         |
| `is null`      | Is null                   | `.where('deletedAt', 'is null')`             |
| `is not null`  | Is not null               | `.where('email', 'is not null')`             |
| `includes`     | Array includes value      | `.where('tags', 'includes', 'tech')`         |
| `includes any` | Array includes any value  | `.where('tags', 'includes any', ['a', 'b'])` |
| `includes all` | Array includes all values | `.where('tags', 'includes all', ['a', 'b'])` |

### OR Conditions

Use `orWhere` to create OR conditions:

```typescript
// Users who are either active OR have logged in recently
const relevantUsers = await User.query()
  .where('isActive', true)
  .orWhere('lastLoginAt', '>', Date.now() - 7 * 24 * 60 * 60 * 1000)
  .find();

// Complex OR conditions with grouping
const complexQuery = await Post.query()
  .where('isPublished', true)
  .where((query) => {
    query.where('category', 'tech').orWhere('category', 'programming');
  })
  .orWhere((query) => {
    query.where('likeCount', '>', 100).where('commentCount', '>', 10);
  })
  .find();
```

### Query Grouping

Group conditions using nested query builders:

```typescript
// (status = 'active' OR status = 'pending') AND (role = 'admin' OR role = 'moderator')
const privilegedUsers = await User.query()
  .where((query) => {
    query.where('status', 'active').orWhere('status', 'pending');
  })
  .where((query) => {
    query.where('role', 'admin').orWhere('role', 'moderator');
  })
  .find();

// Complex nested conditions
const complexPosts = await Post.query()
  .where('isPublished', true)
  .where((query) => {
    query
      .where((subQuery) => {
        subQuery.where('category', 'tech').where('difficulty', 'beginner');
      })
      .orWhere((subQuery) => {
        subQuery.where('category', 'tutorial').where('likeCount', '>', 50);
      });
  })
  .find();
```

## Ordering and Pagination

### Ordering Results

```typescript
// Order by single field
const usersByName = await User.query().orderBy('username').find();

// Order by multiple fields
const postsByPopularity = await Post.query()
  .orderBy('likeCount', 'desc')
  .orderBy('createdAt', 'desc')
  .find();

// Order by computed fields
const usersByActivity = await User.query()
  .orderBy('lastLoginAt', 'desc')
  .orderBy('username', 'asc')
  .find();

// Random ordering
const randomPosts = await Post.query().orderBy('random').limit(10).find();
```

### Pagination

```typescript
// Limit results
const latestPosts = await Post.query().orderBy('createdAt', 'desc').limit(10).find();

// Offset pagination
const secondPage = await Post.query().orderBy('createdAt', 'desc').limit(10).offset(10).find();

// Cursor-based pagination (more efficient for large datasets)
const cursorPosts = await Post.query()
  .orderBy('createdAt', 'desc')
  .after('cursor_value_here')
  .limit(10)
  .find();

// Get pagination info
const paginatedResult = await Post.query().orderBy('createdAt', 'desc').paginate(1, 20); // page 1, 20 items per page

console.log(paginatedResult.data); // Results
console.log(paginatedResult.total); // Total count
console.log(paginatedResult.page); // Current page
console.log(paginatedResult.perPage); // Items per page
console.log(paginatedResult.totalPages); // Total pages
```

## Relationship Loading

### Eager Loading

Load relationships along with the main query:

```typescript
// Load user with their posts
const usersWithPosts = await User.query().with(['posts']).find();

// Load nested relationships
const usersWithPostsAndComments = await User.query().with(['posts.comments']).find();

// Load multiple relationships
const fullUserData = await User.query().with(['posts', 'comments', 'followers']).find();

// Load relationships with conditions
const usersWithRecentPosts = await User.query()
  .with(['posts'], (query) => {
    query
      .where('createdAt', '>', Date.now() - 30 * 24 * 60 * 60 * 1000)
      .orderBy('createdAt', 'desc')
      .limit(5);
  })
  .find();

// Complex relationship loading
const complexUserData = await User.query()
  .with([
    'posts.comments.user', // Deep nested relationships
    'followers.posts', // Multiple levels
    'settings', // Simple relationship
  ])
  .find();
```

### Lazy Loading

Relationships are loaded automatically when accessed:

```typescript
const user = await User.findById('user_123');

// Posts are loaded when first accessed
console.log(user.posts.length); // Triggers lazy load

// Subsequent access uses cached data
user.posts.forEach((post) => console.log(post.title));
```

### Relationship Constraints

Apply constraints to relationship loading:

```typescript
// Load users with their published posts only
const usersWithPublishedPosts = await User.query()
  .with(['posts'], (query) => {
    query.where('isPublished', true).orderBy('publishedAt', 'desc');
  })
  .find();

// Load posts with recent comments only
const postsWithRecentComments = await Post.query()
  .with(['comments'], (query) => {
    query
      .where('createdAt', '>', Date.now() - 24 * 60 * 60 * 1000)
      .with(['user']) // Load comment users too
      .orderBy('createdAt', 'desc');
  })
  .find();

// Load users with follower count > 100
const popularUsers = await User.query()
  .with(['followers'], (query) => {
    query.limit(100); // Limit followers loaded
  })
  .having('followers.length', '>', 100)
  .find();
```

## Advanced Query Features

### Aggregation

```typescript
// Count records
const userCount = await User.query().where('isActive', true).count();

// Count distinct values
const uniqueCategories = await Post.query().countDistinct('category');

// Sum values
const totalLikes = await Post.query().sum('likeCount');

// Average values
const averageScore = await User.query().average('score');

// Min/Max values
const oldestPost = await Post.query().min('createdAt');

const newestPost = await Post.query().max('createdAt');

// Group by aggregations
const postsByCategory = await Post.query()
  .groupBy('category')
  .select(['category', 'COUNT(*) as count', 'AVG(likeCount) as avgLikes'])
  .find();
```

### Having Clauses

Use `having` for filtering aggregated results:

```typescript
// Categories with more than 10 posts
const popularCategories = await Post.query()
  .groupBy('category')
  .having('COUNT(*)', '>', 10)
  .select(['category', 'COUNT(*) as postCount'])
  .find();

// Users with high average post likes
const influentialUsers = await User.query()
  .join('posts', 'users.id', 'posts.userId')
  .groupBy('users.id')
  .having('AVG(posts.likeCount)', '>', 50)
  .select(['users.*', 'AVG(posts.likeCount) as avgLikes'])
  .find();
```

### Subqueries

```typescript
// Users who have posts
const usersWithPosts = await User.query()
  .whereExists(User.query().select('1').from('posts').whereColumn('posts.userId', 'users.id'))
  .find();

// Users who don't have posts
const usersWithoutPosts = await User.query()
  .whereNotExists(User.query().select('1').from('posts').whereColumn('posts.userId', 'users.id'))
  .find();

// Posts with above-average like count
const popularPosts = await Post.query()
  .where('likeCount', '>', (query) => {
    query.select('AVG(likeCount)').from('posts');
  })
  .find();
```

### Raw Queries

For complex queries that can't be expressed with the query builder:

```typescript
// Raw where clause
const complexUsers = await User.query()
  .whereRaw('score > ? AND (status = ? OR created_at > ?)', [
    100,
    'active',
    Date.now() - 30 * 24 * 60 * 60 * 1000,
  ])
  .find();

// Raw select
const userStats = await User.query()
  .selectRaw('username, COUNT(posts.id) as post_count, AVG(posts.like_count) as avg_likes')
  .join('posts', 'users.id', 'posts.user_id')
  .groupBy('users.id', 'username')
  .find();

// Completely raw query
const customResults = await User.raw(
  `
  SELECT u.username, COUNT(p.id) as posts
  FROM users u
  LEFT JOIN posts p ON u.id = p.user_id
  WHERE u.created_at > ?
  GROUP BY u.id, u.username
  HAVING COUNT(p.id) > ?
`,
  [Date.now() - 30 * 24 * 60 * 60 * 1000, 5],
);
```

## Query Caching

### Basic Caching

```typescript
// Cache query results for 5 minutes
const cachedUsers = await User.query()
  .where('isActive', true)
  .cache(300) // 300 seconds
  .find();

// Cache with custom key
const cachedPosts = await Post.query()
  .where('category', 'tech')
  .cache(600, 'tech-posts') // Custom cache key
  .find();

// Disable caching for sensitive data
const sensitiveData = await User.query().where('role', 'admin').noCache().find();
```

### Cache Tags

Use cache tags for intelligent cache invalidation:

```typescript
// Tag cache entries
const taggedQuery = await Post.query()
  .where('category', 'tech')
  .cacheTag(['posts', 'tech-posts'])
  .cache(600)
  .find();

// Invalidate tagged cache entries
await Post.invalidateCacheTag('tech-posts');
```

## Query Optimization

### Indexes

Ensure your frequently queried fields are indexed:

```typescript
@Model({
  indexes: [
    { fields: ['username'], unique: true },
    { fields: ['email'], unique: true },
    { fields: ['createdAt', 'isActive'] },
    { fields: ['category', 'publishedAt'] },
  ],
})
export class User extends BaseModel {
  // Model definition
}
```

### Query Hints

Provide hints to the query optimizer:

```typescript
// Hint about expected result size
const users = await User.query()
  .where('isActive', true)
  .hint('small_result_set') // Expects < 100 results
  .find();

// Hint about query pattern
const recentPosts = await Post.query()
  .where('createdAt', '>', Date.now() - 24 * 60 * 60 * 1000)
  .hint('time_range_query')
  .orderBy('createdAt', 'desc')
  .find();
```

### Batch Operations

Optimize multiple queries with batching:

```typescript
// Batch multiple queries
const [users, posts, comments] = await Promise.all([
  User.query().where('isActive', true).find(),
  Post.query().where('isPublished', true).find(),
  Comment.query().where('isApproved', true).find(),
]);

// Batch with shared cache
const batchResults = await Query.batch()
  .add('users', User.query().where('isActive', true))
  .add('posts', Post.query().where('isPublished', true))
  .add('comments', Comment.query().where('isApproved', true))
  .cache(300)
  .execute();
```

## Error Handling

```typescript
try {
  const users = await User.query()
    .where('invalidField', 'value') // This might cause an error
    .find();
} catch (error) {
  if (error.code === 'INVALID_FIELD') {
    console.error('Invalid field in query:', error.field);
  } else if (error.code === 'NETWORK_ERROR') {
    console.error('Network error during query:', error.message);
  } else {
    console.error('Unexpected error:', error);
  }
}

// Query with error recovery
const safeUsers = await User.query()
  .where('isActive', true)
  .fallback(() => {
    // Fallback to cache if query fails
    return User.fromCache('active-users') || [];
  })
  .find();
```

## Performance Best Practices

1. **Use indexes** for frequently queried fields
2. **Limit results** with `.limit()` to avoid loading large datasets
3. **Use caching** for expensive or repeated queries
4. **Eager load relationships** when you know you'll need them
5. **Use pagination** for large result sets
6. **Monitor query performance** and optimize slow queries
7. **Batch related queries** when possible
8. **Use appropriate data types** and avoid unnecessary type conversions

The DebrosFramework Query Builder provides a powerful and flexible way to query your decentralized data while maintaining type safety and performance across distributed systems.
