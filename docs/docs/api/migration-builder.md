---
sidebar_position: 8
---

# MigrationBuilder

The `MigrationBuilder` provides a fluent API for creating database migrations in DebrosFramework.

## Overview

The MigrationBuilder allows you to define schema changes, data transformations, and migration operations using a fluent, chainable API.

## Class Definition

```typescript
class MigrationBuilder {
  constructor(
    private id: string,
    private version: string,
    private name: string
  );
}
```

## Core Methods

### Field Operations

#### `addField(modelName, fieldName, config)`

Adds a new field to a model.

```typescript
addField(
  modelName: string,
  fieldName: string,
  config: FieldConfig
): MigrationBuilder
```

**Parameters:**
- `modelName` - Name of the model
- `fieldName` - Name of the field to add
- `config` - Field configuration

**Example:**
```typescript
const migration = createMigration('add_user_bio', '1.1.0')
  .addField('User', 'bio', { type: 'string', required: false });
```

### Data Transformations

#### `transformData(modelName, transformer)`

Transforms existing data during migration.

```typescript
transformData(
  modelName: string,
  transformer: (data: any) => any
): MigrationBuilder
```

## Related Classes

- [`MigrationManager`](./migration-manager) - Migration execution
- [`DatabaseManager`](./database-manager) - Database management
