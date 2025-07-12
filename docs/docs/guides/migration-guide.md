# Migration Guide

This guide helps you migrate between versions of DebrosFramework and understand breaking changes, new features, and upgrade procedures.

## Version History

### Current Version: 0.5.1-beta

**Status**: Active Development
**Release Date**: Current
**Stability**: Beta - API may change

## Migration Strategies

### Understanding DebrosFramework Versions

DebrosFramework follows semantic versioning:
- **Major versions** (1.0.0) - Breaking changes, major new features
- **Minor versions** (0.1.0) - New features, backwards compatible
- **Patch versions** (0.0.1) - Bug fixes, no API changes

### Current Development Status

Since DebrosFramework is currently in beta (0.5.x), some features are:

✅ **Stable and Production-Ready:**
- Core model system with decorators
- Basic CRUD operations
- Field validation and transformation
- Lifecycle hooks
- Database management
- Sharding system

🚧 **In Active Development:**
- Advanced query builder features
- Complex relationship loading
- Query optimization
- Full migration system

❌ **Planned for Future Releases:**
- Real-time synchronization
- Advanced caching strategies
- Performance monitoring tools
- Distributed consensus features

## Upgrade Procedures

### From 0.4.x to 0.5.x

#### Breaking Changes

1. **Model Field Definition Changes**
   ```typescript
   // OLD (0.4.x)
   @Field({ type: String, required: true })
   username: string;

   // NEW (0.5.x)
   @Field({ type: 'string', required: true })
   username: string;
   ```

2. **Framework Configuration Structure**
   ```typescript
   // OLD (0.4.x)
   const framework = new DebrosFramework({
     cacheEnabled: true,
     logLevel: 'debug'
   });

   // NEW (0.5.x)
   const framework = new DebrosFramework({
     features: {
       queryCache: true,
     },
     monitoring: {
       logLevel: 'debug'
     }
   });
   ```

3. **Query Builder API Changes**
   ```typescript
   // OLD (0.4.x) - Static methods
   const users = await User.where('isActive', true).find();

   // NEW (0.5.x) - Query builder pattern
   const users = await User.query().where('isActive', true).find();
   ```

#### Migration Steps

1. **Update Package**
   ```bash
   npm install @debros/network@^0.5.0
   ```

2. **Update Field Definitions**
   ```typescript
   // Update all field type definitions from constructors to strings
   @Field({ type: 'string' }) // instead of String
   @Field({ type: 'number' }) // instead of Number
   @Field({ type: 'boolean' }) // instead of Boolean
   @Field({ type: 'array' }) // instead of Array
   @Field({ type: 'object' }) // instead of Object
   ```

3. **Update Framework Configuration**
   ```typescript
   // Migrate old config structure to new nested structure
   const framework = new DebrosFramework({
     features: {
       queryCache: true,
       automaticPinning: true,
       pubsub: true,
       relationshipCache: true,
     },
     performance: {
       queryTimeout: 30000,
       batchSize: 100,
     },
     monitoring: {
       enableMetrics: true,
       logLevel: 'info',
     },
   });
   ```

4. **Update Query Patterns**
   ```typescript
   // Replace static query methods with query builder
   
   // OLD
   const users = await User.where('isActive', true).find();
   const posts = await Post.orderBy('createdAt', 'desc').limit(10).find();
   
   // NEW
   const users = await User.query().where('isActive', true).find();
   const posts = await Post.query().orderBy('createdAt', 'desc').limit(10).find();
   ```

5. **Update Error Handling**
   ```typescript
   // NEW error types
   try {
     const user = await User.create(data);
   } catch (error) {
     if (error instanceof ValidationError) {
       console.log('Field validation failed:', error.field);
     } else if (error instanceof DatabaseError) {
       console.log('Database operation failed:', error.message);
     }
   }
   ```

#### Automated Migration Script

```typescript
// migration-script.ts
import * as fs from 'fs';
import * as path from 'path';

interface MigrationRule {
  pattern: RegExp;
  replacement: string;
  description: string;
}

const migrationRules: MigrationRule[] = [
  {
    pattern: /@Field\(\s*{\s*type:\s*(String|Number|Boolean|Array|Object)/g,
    replacement: '@Field({ type: \'$1\'.toLowerCase()',
    description: 'Convert field types from constructors to strings'
  },
  {
    pattern: /(\w+)\.where\(/g,
    replacement: '$1.query().where(',
    description: 'Convert static where calls to query builder'
  },
  {
    pattern: /(\w+)\.orderBy\(/g,
    replacement: '$1.query().orderBy(',
    description: 'Convert static orderBy calls to query builder'
  },
  {
    pattern: /(\w+)\.limit\(/g,
    replacement: '$1.query().limit(',
    description: 'Convert static limit calls to query builder'
  },
];

function migrateFile(filePath: string): void {
  let content = fs.readFileSync(filePath, 'utf8');
  let hasChanges = false;

  migrationRules.forEach(rule => {
    if (rule.pattern.test(content)) {
      content = content.replace(rule.pattern, rule.replacement);
      hasChanges = true;
      console.log(`✅ Applied: ${rule.description} in ${filePath}`);
    }
  });

  if (hasChanges) {
    fs.writeFileSync(filePath, content);
    console.log(`📝 Updated: ${filePath}`);
  }
}

function migrateDirectory(dirPath: string): void {
  const files = fs.readdirSync(dirPath);
  
  files.forEach(file => {
    const fullPath = path.join(dirPath, file);
    const stat = fs.statSync(fullPath);
    
    if (stat.isDirectory()) {
      migrateDirectory(fullPath);
    } else if (file.endsWith('.ts') || file.endsWith('.js')) {
      migrateFile(fullPath);
    }
  });
}

// Run migration
console.log('🚀 Starting DebrosFramework 0.5.x migration...');
migrateDirectory('./src');
console.log('✅ Migration completed!');
```

## Feature Development Roadmap

### Upcoming Features (0.6.x)

1. **Enhanced Query Builder**
   - Full WHERE clause support
   - JOIN operations
   - Subqueries
   - Query optimization

2. **Advanced Relationships**
   - Polymorphic relationships
   - Through relationships
   - Eager loading optimization

3. **Performance Improvements**
   - Query result caching
   - Connection pooling
   - Batch operations

### Future Features (0.7.x+)

1. **Real-time Features**
   - Live queries
   - Real-time synchronization
   - Conflict resolution

2. **Advanced Migration System**
   - Schema versioning
   - Data transformation
   - Rollback capabilities

3. **Monitoring and Analytics**
   - Performance metrics
   - Query analysis
   - Health monitoring

## Best Practices for Migration

### 1. Test in Development First

```typescript
// Create a test migration environment
const testFramework = new DebrosFramework({
  environment: 'test',
  features: {
    queryCache: false, // Disable caching for testing
  },
  monitoring: {
    logLevel: 'debug', // Verbose logging
  },
});

// Test all your models and operations
async function testMigration() {
  try {
    await testFramework.initialize(orbitDBService, ipfsService);
    
    // Test each model
    await testUserOperations();
    await testPostOperations();
    await testQueryOperations();
    
    console.log('✅ Migration test passed');
  } catch (error) {
    console.error('❌ Migration test failed:', error);
  }
}
```

### 2. Gradual Migration Strategy

```typescript
// Step 1: Update dependencies
// Step 2: Migrate models one at a time
// Step 3: Update query patterns
// Step 4: Test thoroughly
// Step 5: Deploy to staging
// Step 6: Deploy to production

class GradualMigration {
  private migratedModels = new Set<string>();
  
  async migrateModel(modelName: string, modelClass: any) {
    try {
      // Validate model configuration
      await this.validateModelConfig(modelClass);
      
      // Test basic operations
      await this.testModelOperations(modelClass);
      
      this.migratedModels.add(modelName);
      console.log(`✅ Migrated model: ${modelName}`);
    } catch (error) {
      console.error(`❌ Failed to migrate model ${modelName}:`, error);
      throw error;
    }
  }
  
  private async validateModelConfig(modelClass: any) {
    // Validate field definitions
    // Check relationship configurations
    // Verify decorator usage
  }
  
  private async testModelOperations(modelClass: any) {
    // Test create, read, update, delete
    // Test query operations
    // Test relationships
  }
}
```

### 3. Backup and Recovery

```typescript
// Create backup before migration
async function createBackup() {
  const framework = getCurrentFramework();
  const databaseManager = framework.getDatabaseManager();
  
  // Export all data
  const backup = {
    timestamp: Date.now(),
    version: '0.4.x',
    databases: {},
  };
  
  // Backup each database
  const databases = await databaseManager.getAllDatabases();
  for (const [name, db] of databases) {
    backup.databases[name] = await exportDatabase(db);
  }
  
  // Save backup
  await saveBackup(backup);
  console.log('✅ Backup created successfully');
}

async function restoreFromBackup(backupPath: string) {
  const backup = await loadBackup(backupPath);
  
  // Restore each database
  for (const [name, data] of Object.entries(backup.databases)) {
    await restoreDatabase(name, data);
  }
  
  console.log('✅ Restored from backup');
}
```

## Troubleshooting Migration Issues

### Common Issues and Solutions

#### 1. Field Type Errors

**Problem**: `TypeError: Field type must be a string`

**Solution**:
```typescript
// Wrong
@Field({ type: String })

// Correct
@Field({ type: 'string' })
```

#### 2. Query Builder Not Found

**Problem**: `TypeError: User.where is not a function`

**Solution**:
```typescript
// Wrong
const users = await User.where('isActive', true).find();

// Correct
const users = await User.query().where('isActive', true).find();
```

#### 3. Configuration Structure Errors

**Problem**: `Unknown configuration option: cacheEnabled`

**Solution**:
```typescript
// Wrong
const framework = new DebrosFramework({
  cacheEnabled: true
});

// Correct
const framework = new DebrosFramework({
  features: {
    queryCache: true
  }
});
```

#### 4. Relationship Loading Issues

**Problem**: `Cannot read property 'posts' of undefined`

**Solution**:
```typescript
// Ensure relationships are loaded
const user = await User.findById(userId, {
  with: ['posts']
});

// Or use the relationship manager
const relationshipManager = framework.getRelationshipManager();
await relationshipManager.loadRelationship(user, 'posts');
```

### Migration Validation

```typescript
// Validation script to run after migration
async function validateMigration() {
  const checks = [
    validateModels,
    validateQueries,
    validateRelationships,
    validatePerformance,
  ];
  
  for (const check of checks) {
    try {
      await check();
      console.log(`✅ ${check.name} passed`);
    } catch (error) {
      console.error(`❌ ${check.name} failed:`, error);
      throw error;
    }
  }
  
  console.log('🎉 Migration validation completed successfully');
}

async function validateModels() {
  // Test model creation, updates, deletion
  const user = await User.create({
    username: 'test_migration',
    email: 'test@migration.com'
  });
  
  await user.delete();
}

async function validateQueries() {
  // Test basic queries work
  const users = await User.query().find();
  if (!Array.isArray(users)) {
    throw new Error('Query did not return array');
  }
}

async function validateRelationships() {
  // Test relationship loading
  // Implementation depends on your models
}

async function validatePerformance() {
  // Basic performance checks
  const start = Date.now();
  await User.query().find();
  const duration = Date.now() - start;
  
  if (duration > 5000) {
    console.warn('⚠️ Query performance degraded');
  }
}
```

## Getting Help

### Migration Support

- **GitHub Issues**: Report migration problems
- **Discord Community**: Get real-time help
- **Migration Assistance**: Contact the development team for complex migrations

### Useful Commands

```bash
# Check current version
npm list @debros/network

# Update to latest beta
npm install @debros/network@beta

# Check for breaking changes
npm audit

# Run migration tests
npm run test:migration
```

This migration guide will be updated as DebrosFramework evolves. Always check the latest documentation before starting a migration.
