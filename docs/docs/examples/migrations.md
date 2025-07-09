---
sidebar_position: 3
---

# Migration Examples
This example demonstrates various migration scenarios using DebrosFramework.

## Overview

Explore different migration techniques to evolve your database schema over time.

## Migration Scenarios

### Adding Fields

Add new fields to an existing model:

```typescript
const migration = createMigration('add_user_address', '1.4.0')
  .addField('User', 'address', { type: 'string', required: false })
  .addField('User', 'phone', { type: 'string', required: false });
```

### Removing Fields

Remove obsolete fields from a model:

```typescript
const migration = createMigration('remove_user_legacy', '1.5.0')
  .removeField('User', 'legacyField');
```

### Modifying Fields

Modify field properties without data loss:

```typescript
const migration = createMigration('modify_email_format', '1.6.0')
  .modifyField('User', 'email', {
    type: 'string',
    required: true,
    unique: true,
    validate: (email: string) =\u003e /^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$/.test(email),
  });
```

### Data Transformation

Use data transformers to update field values during migration:

```typescript
const migration = createMigration('transform_display_names', '1.7.0')
  .transformData('User', (user) =\u003e ({
    ...user,
    displayName: `${user.firstName} ${user.lastName}`.trim()
  }));
```

## Running Migrations

### Manual Execution

Execute migrations manually using the MigrationManager:

```typescript
const migrationManager = new MigrationManager(databaseManager, configManager);
await migrationManager.runMigration(migration);
```

### Automatic Execution

Enable automatic migration execution:

```typescript
const framework = new DebrosFramework({
  migrations: {
    autoRun: true,
    directory: './migrations'
  }
});
```

## Best Practices

1. **Test Thoroughly** - Validate migrations in a test environment before applying to production.
2. **Backup Data** - Ensure data backups before running critical migrations.
3. **Use Transactions** - Where possible, use transactions for atomic operations.
4. **Have Rollback Plans** - Prepare rollback scripts for critical changes.

## Related Topics

- [MigrationManager](../api/migration-manager) - Migration execution
- [MigrationBuilder](../api/migration-builder) - Migration creation
