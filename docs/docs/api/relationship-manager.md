---
sidebar_position: 6
---

# RelationshipManager

The `RelationshipManager` handles model relationships and data loading in DebrosFramework.

## Overview

The RelationshipManager manages relationships between models, handles lazy and eager loading, and provides relationship caching for performance optimization.

## Class Definition

```typescript
class RelationshipManager {
  constructor(
    private databaseManager: DatabaseManager,
    private queryExecutor: QueryExecutor
  );
}
```

## Core Methods

### Relationship Loading

#### `loadRelationship<T>(model, relationship)`

Loads a relationship for a model instance.

```typescript
async loadRelationship<T>(
  model: BaseModel,
  relationship: string
): Promise<T | T[]>
```

**Parameters:**
- `model` - The model instance
- `relationship` - The relationship name to load

**Returns:** Promise resolving to related model(s)

**Example:**
```typescript
const user = await User.findById('user123');
const posts = await relationshipManager.loadRelationship(user, 'posts');
```

## Related Classes

- [`BaseModel`](./base-model) - Base model class
- [`QueryExecutor`](./query-executor) - Query execution
