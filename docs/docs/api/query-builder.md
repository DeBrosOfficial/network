---
sidebar_position: 4
---

# QueryBuilder Class

The `QueryBuilder` class provides a fluent API for constructing complex database queries. It supports filtering, sorting, relationships, pagination, and caching with type safety throughout.

## Class Definition

```typescript
class QueryBuilder<T extends BaseModel> {
  // Filtering methods
  where(field: string, value: any): QueryBuilder<T>;
  where(field: string, operator: QueryOperator, value: any): QueryBuilder<T>;
  whereIn(field: string, values: any[]): QueryBuilder<T>;
  whereNotIn(field: string, values: any[]): QueryBuilder<T>;
  whereNull(field: string): QueryBuilder<T>;
  whereNotNull(field: string): QueryBuilder<T>;
  whereBetween(field: string, min: any, max: any): QueryBuilder<T>;
  whereRaw(condition: string, parameters?: any[]): QueryBuilder<T>;

  // Logical operators
  and(): QueryBuilder<T>;
  or(): QueryBuilder<T>;
  not(): QueryBuilder<T>;

  // Sorting
  orderBy(field: string, direction?: 'asc' | 'desc'): QueryBuilder<T>;
  orderByRaw(orderClause: string): QueryBuilder<T>;

  // Limiting and pagination
  limit(count: number): QueryBuilder<T>;
  offset(count: number): QueryBuilder<T>;
  paginate(page: number, perPage: number): Promise<PaginatedResult<T>>;

  // Relationships
  with(relationships: string[]): QueryBuilder<T>;
  withCount(relationships: string[]): QueryBuilder<T>;
  whereHas(relationship: string, callback?: (query: QueryBuilder<any>) => void): QueryBuilder<T>;
  whereDoesntHave(relationship: string): QueryBuilder<T>;

  // Aggregation
  count(): Promise<number>;
  sum(field: string): Promise<number>;
  avg(field: string): Promise<number>;
  min(field: string): Promise<any>;
  max(field: string): Promise<any>;

  // Caching
  cache(ttl?: number): QueryBuilder<T>;
  fresh(): QueryBuilder<T>;

  // Execution
  find(): Promise<T[]>;
  findOne(): Promise<T | null>;
  first(): Promise<T | null>;
  get(): Promise<T[]>;

  // Advanced features
  distinct(field?: string): QueryBuilder<T>;
  groupBy(field: string): QueryBuilder<T>;
  having(field: string, operator: QueryOperator, value: any): QueryBuilder<T>;

  // Query info
  toSQL(): string;
  getParameters(): any[];
  explain(): Promise<QueryPlan>;
}
```

## Filtering Methods

### where(field, value) / where(field, operator, value)

Adds a WHERE condition to the query.

**Parameters:**

- `field`: Field name to filter on
- `value`: Value to compare (when using equals operator)
- `operator`: Comparison operator
- `value`: Value to compare against

**Returns:** `QueryBuilder<T>` - Builder instance for chaining

**Example:**

```typescript
// Basic equality
const activeUsers = await User.query().where('isActive', true).find();

// With operators
const highScoreUsers = await User.query()
  .where('score', '>', 1000)
  .where('registeredAt', '>=', Date.now() - 30 * 24 * 60 * 60 * 1000)
  .find();

// String operations
const usersWithAlice = await User.query().where('username', 'like', '%alice%').find();

// Array operations
const adminUsers = await User.query().where('roles', 'includes', 'admin').find();
```

### whereIn(field, values)

Filters records where field value is in the provided array.

**Parameters:**

- `field`: Field name
- `values`: Array of values to match

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Find users with specific IDs
const users = await User.query().whereIn('id', ['user1', 'user2', 'user3']).find();

// Find posts with specific tags
const posts = await Post.query().whereIn('category', ['tech', 'programming', 'tutorial']).find();
```

### whereNotIn(field, values)

Filters records where field value is NOT in the provided array.

**Parameters:**

- `field`: Field name
- `values`: Array of values to exclude

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Exclude specific users
const users = await User.query().whereNotIn('status', ['banned', 'suspended']).find();
```

### whereNull(field) / whereNotNull(field)

Filters records based on null values.

**Parameters:**

- `field`: Field name to check

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Find users without profile pictures
const usersWithoutAvatars = await User.query().whereNull('avatarUrl').find();

// Find users with profile pictures
const usersWithAvatars = await User.query().whereNotNull('avatarUrl').find();
```

### whereBetween(field, min, max)

Filters records where field value is between min and max.

**Parameters:**

- `field`: Field name
- `min`: Minimum value (inclusive)
- `max`: Maximum value (inclusive)

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Find users with scores between 100 and 500
const moderateUsers = await User.query().whereBetween('score', 100, 500).find();

// Find posts from last week
const lastWeek = Date.now() - 7 * 24 * 60 * 60 * 1000;
const recentPosts = await Post.query().whereBetween('createdAt', lastWeek, Date.now()).find();
```

### whereRaw(condition, parameters?)

Adds a raw WHERE condition.

**Parameters:**

- `condition`: Raw SQL-like condition string
- `parameters`: Optional parameters for the condition

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Complex condition
const users = await User.query()
  .whereRaw('score > ? AND (registeredAt > ? OR isPremium = ?)', [100, lastWeek, true])
  .find();
```

## Logical Operators

### and() / or() / not()

Combines conditions with logical operators.

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// AND (default behavior)
const premiumActiveUsers = await User.query()
  .where('isActive', true)
  .and()
  .where('isPremium', true)
  .find();

// OR condition
const eligibleUsers = await User.query()
  .where('isPremium', true)
  .or()
  .where('score', '>', 1000)
  .find();

// NOT condition
const nonAdminUsers = await User.query().not().where('role', 'admin').find();

// Complex combinations
const complexQuery = await User.query()
  .where('isActive', true)
  .and()
  .group((query) => query.where('isPremium', true).or().where('score', '>', 500))
  .find();
```

## Sorting Methods

### orderBy(field, direction?)

Orders results by specified field.

**Parameters:**

- `field`: Field name to sort by
- `direction`: Sort direction ('asc' or 'desc', defaults to 'asc')

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Sort by score descending
const topUsers = await User.query().orderBy('score', 'desc').limit(10).find();

// Multiple sort criteria
const users = await User.query().orderBy('score', 'desc').orderBy('username', 'asc').find();

// Sort by date
const recentPosts = await Post.query().orderBy('createdAt', 'desc').find();
```

### orderByRaw(orderClause)

Orders results using a raw ORDER BY clause.

**Parameters:**

- `orderClause`: Raw order clause

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
const users = await User.query().orderByRaw('score DESC, RANDOM()').find();
```

## Limiting and Pagination

### limit(count)

Limits the number of results.

**Parameters:**

- `count`: Maximum number of records to return

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Get top 10 users
const topUsers = await User.query().orderBy('score', 'desc').limit(10).find();
```

### offset(count)

Skips a number of records.

**Parameters:**

- `count`: Number of records to skip

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Get users 11-20 (skip first 10)
const nextBatch = await User.query().orderBy('score', 'desc').offset(10).limit(10).find();
```

### paginate(page, perPage)

Paginates results and returns pagination info.

**Parameters:**

- `page`: Page number (1-based)
- `perPage`: Number of records per page

**Returns:** `Promise<PaginatedResult<T>>`

**Example:**

```typescript
// Get page 2 with 20 users per page
const result = await User.query().where('isActive', true).orderBy('score', 'desc').paginate(2, 20);

console.log('Users:', result.data);
console.log('Total:', result.total);
console.log('Page:', result.page);
console.log('Per page:', result.perPage);
console.log('Total pages:', result.totalPages);
console.log('Has next:', result.hasNext);
console.log('Has previous:', result.hasPrev);
```

## Relationship Methods

### with(relationships)

Eager loads relationships.

**Parameters:**

- `relationships`: Array of relationship names or dot-notation for nested relationships

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Load single relationship
const usersWithPosts = await User.query().with(['posts']).find();

// Load multiple relationships
const usersWithData = await User.query().with(['posts', 'profile', 'followers']).find();

// Load nested relationships
const usersWithCommentsAuthors = await User.query().with(['posts.comments.author']).find();

// Mixed relationships
const complexLoad = await User.query()
  .with(['posts.comments.author', 'profile', 'followers.profile'])
  .find();
```

### withCount(relationships)

Loads relationship counts without loading the actual relationships.

**Parameters:**

- `relationships`: Array of relationship names

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
const usersWithCounts = await User.query().withCount(['posts', 'followers', 'following']).find();

// Access counts
usersWithCounts.forEach((user) => {
  console.log(`${user.username}: ${user.postsCount} posts, ${user.followersCount} followers`);
});
```

### whereHas(relationship, callback?)

Filters records that have related records matching criteria.

**Parameters:**

- `relationship`: Relationship name
- `callback`: Optional callback to add conditions to the relationship query

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Users who have posts
const usersWithPosts = await User.query().whereHas('posts').find();

// Users who have published posts
const usersWithPublishedPosts = await User.query()
  .whereHas('posts', (query) => {
    query.where('isPublished', true);
  })
  .find();

// Users who have recent posts
const usersWithRecentPosts = await User.query()
  .whereHas('posts', (query) => {
    query.where('createdAt', '>', Date.now() - 7 * 24 * 60 * 60 * 1000);
  })
  .find();
```

### whereDoesntHave(relationship)

Filters records that don't have related records.

**Parameters:**

- `relationship`: Relationship name

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Users without posts
const usersWithoutPosts = await User.query().whereDoesntHave('posts').find();
```

## Aggregation Methods

### count()

Returns the count of matching records.

**Returns:** `Promise<number>`

**Example:**

```typescript
// Count all active users
const activeUserCount = await User.query().where('isActive', true).count();

console.log(`Active users: ${activeUserCount}`);
```

### sum(field) / avg(field) / min(field) / max(field)

Performs aggregation operations on a field.

**Parameters:**

- `field`: Field name to aggregate

**Returns:** `Promise<number>` or `Promise<any>` for min/max

**Example:**

```typescript
// Calculate statistics
const totalScore = await User.query().sum('score');
const averageScore = await User.query().avg('score');
const highestScore = await User.query().max('score');
const lowestScore = await User.query().min('score');

console.log(`Total: ${totalScore}, Average: ${averageScore}`);
console.log(`Range: ${lowestScore} - ${highestScore}`);

// With conditions
const premiumAverage = await User.query().where('isPremium', true).avg('score');
```

## Caching Methods

### cache(ttl?)

Caches query results.

**Parameters:**

- `ttl`: Time to live in seconds (optional, uses default if not provided)

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Cache for default duration
const cachedUsers = await User.query().where('isActive', true).cache().find();

// Cache for 10 minutes
const cachedPosts = await Post.query().where('isPublished', true).cache(600).find();
```

### fresh()

Forces a fresh query, bypassing cache.

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Always fetch fresh data
const freshUsers = await User.query().where('isActive', true).fresh().find();
```

## Execution Methods

### find()

Executes the query and returns all matching records.

**Returns:** `Promise<T[]>`

**Example:**

```typescript
const users = await User.query().where('isActive', true).orderBy('score', 'desc').find();
```

### findOne() / first()

Executes the query and returns the first matching record.

**Returns:** `Promise<T | null>`

**Example:**

```typescript
// Find the highest scoring user
const topUser = await User.query().orderBy('score', 'desc').findOne();

// Find specific user
const user = await User.query().where('username', 'alice').first();
```

### get()

Alias for `find()`.

**Returns:** `Promise<T[]>`

## Advanced Methods

### distinct(field?)

Returns distinct values.

**Parameters:**

- `field`: Optional field to get distinct values for

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Get users with distinct emails
const uniqueUsers = await User.query().distinct('email').find();

// Get all distinct tags from posts
const uniqueTags = await Post.query().distinct('tags').find();
```

### groupBy(field)

Groups results by field.

**Parameters:**

- `field`: Field to group by

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Group users by registration date
const usersByDate = await User.query().groupBy('registeredAt').withCount(['posts']).find();
```

### having(field, operator, value)

Adds HAVING condition (used with groupBy).

**Parameters:**

- `field`: Field name
- `operator`: Comparison operator
- `value`: Value to compare

**Returns:** `QueryBuilder<T>`

**Example:**

```typescript
// Find dates with more than 10 user registrations
const busyDays = await User.query().groupBy('registeredAt').having('COUNT(*)', '>', 10).find();
```

## Query Inspection

### toSQL()

Returns the SQL representation of the query.

**Returns:** `string`

**Example:**

```typescript
const query = User.query().where('isActive', true).orderBy('score', 'desc').limit(10);

console.log('SQL:', query.toSQL());
```

### getParameters()

Returns the parameters for the query.

**Returns:** `any[]`

**Example:**

```typescript
const query = User.query().where('score', '>', 100);

console.log('Parameters:', query.getParameters());
```

### explain()

Returns the query execution plan.

**Returns:** `Promise<QueryPlan>`

**Example:**

```typescript
const plan = await User.query().where('isActive', true).explain();

console.log('Query plan:', plan);
```

## Type Definitions

### QueryOperator

```typescript
type QueryOperator =
  | 'eq'
  | '='
  | 'ne'
  | '!='
  | '<>'
  | 'gt'
  | '>'
  | 'gte'
  | '>='
  | 'lt'
  | '<'
  | 'lte'
  | '<='
  | 'in'
  | 'not in'
  | 'like'
  | 'not like'
  | 'regex'
  | 'is null'
  | 'is not null'
  | 'includes'
  | 'includes any'
  | 'includes all';
```

### PaginatedResult

```typescript
interface PaginatedResult<T> {
  data: T[];
  total: number;
  page: number;
  perPage: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
  firstPage: number;
  lastPage: number;
}
```

### QueryPlan

```typescript
interface QueryPlan {
  estimatedCost: number;
  estimatedRows: number;
  indexesUsed: string[];
  operations: QueryOperation[];
  warnings: string[];
}

interface QueryOperation {
  type: string;
  description: string;
  cost: number;
}
```

## Complex Query Examples

### Advanced Filtering

```typescript
// Complex user search
const searchResults = await User.query()
  .where('isActive', true)
  .and()
  .group((query) =>
    query
      .where('username', 'like', `%${searchTerm}%`)
      .or()
      .where('email', 'like', `%${searchTerm}%`)
      .or()
      .where('bio', 'like', `%${searchTerm}%`),
  )
  .and()
  .group((query) => query.where('isPremium', true).or().where('score', '>', 500))
  .orderBy('score', 'desc')
  .with(['profile', 'posts.comments'])
  .paginate(page, 20);
```

### Analytics Query

```typescript
// User activity analytics
const analytics = await User.query()
  .where('registeredAt', '>', startDate)
  .where('registeredAt', '<', endDate)
  .whereHas('posts', (query) => {
    query.where('isPublished', true);
  })
  .withCount(['posts', 'comments', 'followers'])
  .orderBy('score', 'desc')
  .cache(300) // Cache for 5 minutes
  .find();

// Process analytics data
const stats = {
  totalUsers: analytics.length,
  averageScore: analytics.reduce((sum, user) => sum + user.score, 0) / analytics.length,
  totalPosts: analytics.reduce((sum, user) => sum + user.postsCount, 0),
  totalComments: analytics.reduce((sum, user) => sum + user.commentsCount, 0),
};
```

### Dashboard Query

```typescript
async function getDashboardData(userId: string) {
  // Multiple optimized queries
  const [userStats, recentPosts, topComments, followingActivity] = await Promise.all([
    // User statistics
    User.query()
      .where('id', userId)
      .withCount(['posts', 'comments', 'followers', 'following'])
      .cache(60) // Cache for 1 minute
      .first(),

    // Recent posts
    Post.query()
      .where('authorId', userId)
      .orderBy('createdAt', 'desc')
      .limit(5)
      .with(['comments.author'])
      .find(),

    // Top comments by user
    Comment.query()
      .where('authorId', userId)
      .orderBy('likeCount', 'desc')
      .limit(10)
      .with(['post.author'])
      .find(),

    // Activity from people user follows
    Post.query()
      .whereHas('author', (query) => {
        query.whereHas('followers', (followQuery) => {
          followQuery.where('followerId', userId);
        });
      })
      .orderBy('createdAt', 'desc')
      .limit(20)
      .with(['author', 'comments'])
      .find(),
  ]);

  return {
    userStats,
    recentPosts,
    topComments,
    followingActivity,
  };
}
```

The QueryBuilder provides a powerful, type-safe way to construct complex database queries with support for relationships, caching, pagination, and advanced filtering options.
