---
sidebar_position: 7
---

# MigrationManager

The `MigrationManager` handles database schema migrations and data transformations in DebrosFramework.

## Overview

The MigrationManager provides tools for evolving database schemas over time, handling version control, and performing data transformations during migrations.

## Class Definition

```typescript
class MigrationManager {
  constructor(
    private databaseManager: DatabaseManager,
    private configManager: ConfigManager
  );
}
```

## Core Methods

### Migration Management

#### `runMigration(migration)`

Executes a migration.

```typescript
async runMigration(migration: Migration): Promise<void>
```

**Parameters:**
- `migration` - The migration to execute

**Example:**
```typescript
await migrationManager.runMigration(addUserProfileMigration);
```

### Migration History

#### `getPendingMigrations()`

Gets all pending migrations.

```typescript
async getPendingMigrations(): Promise<Migration[]>
```

**Returns:** Promise resolving to array of pending migrations

## Related Classes

- [`MigrationBuilder`](./migration-builder) - Migration construction
- [`DatabaseManager`](./database-manager) - Database management
