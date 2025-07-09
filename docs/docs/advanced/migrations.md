---
sidebar_position: 2
---

# Migrations

Migrations provide a way to evolve your database schema over time while preserving data integrity.

## Overview

DebrosFramework includes a migration system that allows you to:

- Add, remove, or modify fields
- Transform existing data
- Handle schema evolution
- Maintain data integrity

## Creating Migrations

### Basic Migration

```typescript
const migration = createMigration('add_user_bio', '1.1.0')
  .addField('User', 'bio', { type: 'string', required: false })
  .addField('User', 'profilePicture', { type: 'string', required: false });
```

### Data Transformation

```typescript
const migration = createMigration('update_user_display_name', '1.2.0')
  .transformData('User', (user) => ({
    ...user,
    displayName: user.displayName || user.username
  }));
```

### Field Modifications

```typescript
const migration = createMigration('modify_user_email', '1.3.0')
  .modifyField('User', 'email', { type: 'string', required: true, unique: true })
  .addValidator('email_format', async (context) => {
    // Custom validation logic
  });
```

## Running Migrations

### Manual Execution

```typescript
const migrationManager = new MigrationManager(databaseManager, configManager);
await migrationManager.runMigration(migration);
```

### Automatic Execution

```typescript
const framework = new DebrosFramework({
  migrations: {
    autoRun: true,
    directory: './migrations'
  }
});
```

## Migration Types

### Schema Changes

- Add fields
- Remove fields
- Modify field types
- Add indexes

### Data Transformations

- Update existing records
- Migrate data formats
- Clean up invalid data

## Best Practices

1. **Test migrations** thoroughly before production
2. **Backup data** before running migrations
3. **Use transactions** where possible
4. **Plan rollback strategies**

## Related Classes

- [`MigrationManager`](../api/migration-manager) - Migration execution
- [`MigrationBuilder`](../api/migration-builder) - Migration creation
