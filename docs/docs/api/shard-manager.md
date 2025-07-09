---
sidebar_position: 4
---

# ShardManager

The `ShardManager` is responsible for managing data sharding and distribution across multiple database instances in DebrosFramework.

## Overview

The ShardManager handles data partitioning and routing to appropriate shards based on configuration and sharding keys. It  optimizes query performance and maintains data consistency across shards.

## Class Definition

```typescript
class ShardManager {
  constructor(
    private config: ShardConfig
  );
}
```

## Core Methods

### Shard Configuration

#### `configureSharding(strategy, count, key)`

Configures sharding strategy.

```typescript
configureSharding(
  strategy: ShardingStrategy,
  count: number,
  key: string
): void
```

**Parameters:**
- `strategy` - One of ('hash', 'range', 'user')
- `count` - Number of shards
- `key` - Field used for sharding

**Example:**
```typescript
shardManager.configureSharding('hash', 4, 'id');
```

### Shard Management

#### `getShardForKey(key)`

Determines the shard for a given key.

```typescript
getShardForKey(key: string): number
```

