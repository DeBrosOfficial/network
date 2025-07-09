---
sidebar_position: 5
---

# QueryExecutor

The `QueryExecutor` is responsible for executing queries across databases and shards in DebrosFramework.

## Overview

The QueryExecutor handles query execution, optimization, and result aggregation. It provides caching and performance monitoring capabilities.

## Class Definition

```typescript
class QueryExecutor {
  constructor(
    private databaseManager: DatabaseManager,
    private shardManager: ShardManager,
    private queryCache: QueryCache
  );
}
```

## Core Methods

### Query Execution

#### `executeQuery<T>(query)`

Executes a query and returns results.

```typescript
async executeQuery<T>(query: QueryBuilder<T>): Promise<QueryResult<T>>
```

**Parameters:**
- `query` - The query to execute

**Returns:** Promise resolving to query results

**Example:**
```typescript
const results = await queryExecutor.executeQuery(
  User.query().where('isActive', true)
);
```

## Related Classes

- [`QueryBuilder`](./query-builder) - Query construction
- [`DatabaseManager`](./database-manager) - Database management
- [`ShardManager`](./shard-manager) - Data sharding
