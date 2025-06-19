/**
 * Comprehensive Migration Examples for DebrosFramework
 * 
 * This file demonstrates the migration system capabilities:
 * - Schema evolution with field additions and modifications
 * - Data transformation and migration
 * - Rollback scenarios and recovery
 * - Cross-model relationship changes
 * - Performance optimization migrations
 * - Version management and dependency handling
 */

import { MigrationManager, Migration } from '../src/framework/migrations/MigrationManager';
import { MigrationBuilder, createMigration } from '../src/framework/migrations/MigrationBuilder';
import { SocialPlatformFramework } from './framework-integration';

export class MigrationExamples {
  private migrationManager: MigrationManager;
  private framework: SocialPlatformFramework;

  constructor(framework: SocialPlatformFramework) {
    this.framework = framework;
    this.migrationManager = new MigrationManager(
      (framework as any).databaseManager,
      (framework as any).shardManager
    );
  }

  async runAllExamples(): Promise<void> {
    console.log('🔄 Running comprehensive migration examples...\n');

    await this.createExampleMigrations();
    await this.basicMigrationExamples();
    await this.complexDataTransformationExamples();
    await this.rollbackAndRecoveryExamples();
    await this.performanceOptimizationExamples();
    await this.crossModelMigrationExamples();
    await this.versionManagementExamples();

    console.log('✅ All migration examples completed!\n');
  }

  async createExampleMigrations(): Promise<void> {
    console.log('📝 Creating Example Migrations');
    console.log('==============================\n');

    // Migration 1: Add timestamps to User model
    const addTimestampsMigration = createMigration(
      'add_user_timestamps',
      '1.0.1',
      'Add timestamps to User model'
    )
      .description('Add createdAt and updatedAt timestamps to User model for better tracking')
      .author('Framework Team')
      .tags('schema', 'timestamps', 'user')
      .addTimestamps('User')
      .addValidator(
        'validate_timestamp_format',
        'Ensure timestamp fields are valid numbers',
        async (context) => {
          const errors: string[] = [];
          const warnings: string[] = [];
          
          // Validate that all timestamps are valid
          return { valid: errors.length === 0, errors, warnings };
        }
      )
      .build();

    // Migration 2: Add user profile enhancements
    const userProfileEnhancement = createMigration(
      'enhance_user_profile',
      '1.1.0',
      'Enhance User profile with additional fields'
    )
      .description('Add profile picture, location, and social links to User model')
      .dependencies('add_user_timestamps')
      .addField('User', 'profilePicture', {
        type: 'string',
        required: false,
        validate: (value) => !value || value.startsWith('http')
      })
      .addField('User', 'location', {
        type: 'string',
        required: false
      })
      .addField('User', 'socialLinks', {
        type: 'array',
        required: false,
        default: []
      })
      .addField('User', 'isVerified', {
        type: 'boolean',
        required: false,
        default: false
      })
      .build();

    // Migration 3: Restructure Post content
    const postContentRestructure = createMigration(
      'restructure_post_content',
      '1.2.0',
      'Restructure Post content with rich metadata'
    )
      .description('Transform Post content from plain text to rich content structure')
      .addField('Post', 'contentType', {
        type: 'string',
        required: false,
        default: 'text'
      })
      .addField('Post', 'metadata', {
        type: 'object',
        required: false,
        default: {}
      })
      .transformData('Post', (post) => {
        // Transform existing content to new structure
        const wordCount = post.content ? post.content.split(' ').length : 0;
        const hasLinks = post.content ? /https?:\/\//.test(post.content) : false;
        
        return {
          ...post,
          contentType: hasLinks ? 'rich' : 'text',
          metadata: {
            wordCount,
            hasLinks,
            transformedAt: Date.now()
          }
        };
      })
      .build();

    // Migration 4: Add Comment threading
    const commentThreading = createMigration(
      'add_comment_threading',
      '1.3.0',
      'Add threading support to Comments'
    )
      .description('Add parent-child relationships to comments for threading')
      .addField('Comment', 'parentId', {
        type: 'string',
        required: false,
        default: null
      })
      .addField('Comment', 'threadDepth', {
        type: 'number',
        required: false,
        default: 0
      })
      .addField('Comment', 'childCount', {
        type: 'number',
        required: false,
        default: 0
      })
      .transformData('Comment', (comment) => {
        // All existing comments become root-level comments
        return {
          ...comment,
          parentId: null,
          threadDepth: 0,
          childCount: 0
        };
      })
      .build();

    // Migration 5: Performance optimization
    const performanceOptimization = createMigration(
      'optimize_post_indexing',
      '1.4.0',
      'Optimize Post model for better query performance'
    )
      .description('Add computed fields and indexes for better query performance')
      .addField('Post', 'searchText', {
        type: 'string',
        required: false,
        default: ''
      })
      .addField('Post', 'popularityScore', {
        type: 'number',
        required: false,
        default: 0
      })
      .transformData('Post', (post) => {
        // Create searchable text and calculate popularity
        const searchText = `${post.title || ''} ${post.content || ''}`.toLowerCase();
        const popularityScore = (post.likeCount || 0) * 2 + (post.commentCount || 0);
        
        return {
          ...post,
          searchText,
          popularityScore
        };
      })
      .createIndex('Post', ['searchText'])
      .createIndex('Post', ['popularityScore'], { name: 'popularity_index' })
      .build();

    // Register all migrations
    const migrations = [
      addTimestampsMigration,
      userProfileEnhancement,
      postContentRestructure,
      commentThreading,
      performanceOptimization
    ];

    for (const migration of migrations) {
      this.migrationManager.registerMigration(migration);
      console.log(`✅ Registered migration: ${migration.name} (v${migration.version})`);
    }

    console.log(`\nRegistered ${migrations.length} example migrations\n`);
  }

  async basicMigrationExamples(): Promise<void> {
    console.log('🔄 Basic Migration Examples');
    console.log('===========================\n');

    // Get pending migrations
    const pendingMigrations = this.migrationManager.getPendingMigrations();
    console.log(`Found ${pendingMigrations.length} pending migrations:`);
    
    pendingMigrations.forEach(migration => {
      console.log(`- ${migration.name} (v${migration.version})`);
    });

    // Run a single migration with dry run first
    if (pendingMigrations.length > 0) {
      const firstMigration = pendingMigrations[0];
      
      console.log(`\nRunning dry run for: ${firstMigration.name}`);
      const dryRunResult = await this.migrationManager.runMigration(firstMigration.id, {
        dryRun: true
      });
      
      console.log('Dry run results:');
      console.log(`- Success: ${dryRunResult.success}`);
      console.log(`- Estimated records: ${dryRunResult.recordsProcessed}`);
      console.log(`- Duration: ${dryRunResult.duration}ms`);
      console.log(`- Warnings: ${dryRunResult.warnings.length}`);

      // Run the actual migration
      console.log(`\nRunning actual migration: ${firstMigration.name}`);
      try {
        const result = await this.migrationManager.runMigration(firstMigration.id, {
          batchSize: 50
        });
        
        console.log('Migration results:');
        console.log(`- Success: ${result.success}`);
        console.log(`- Records processed: ${result.recordsProcessed}`);
        console.log(`- Records modified: ${result.recordsModified}`);
        console.log(`- Duration: ${result.duration}ms`);
        console.log(`- Rollback available: ${result.rollbackAvailable}`);
        
        if (result.warnings.length > 0) {
          console.log('- Warnings:', result.warnings);
        }
        
      } catch (error) {
        console.error(`Migration failed: ${error}`);
      }
    }

    console.log('');
  }

  async complexDataTransformationExamples(): Promise<void> {
    console.log('🔄 Complex Data Transformation Examples');
    console.log('=======================================\n');

    // Create a complex migration that transforms user data
    const userDataNormalization = createMigration(
      'normalize_user_data',
      '2.0.0',
      'Normalize and clean user data'
    )
      .description('Clean up user data, normalize email formats, and merge duplicate accounts')
      .transformData('User', (user) => {
        // Normalize email to lowercase
        if (user.email) {
          user.email = user.email.toLowerCase().trim();
        }

        // Clean up username
        if (user.username) {
          user.username = user.username.trim().replace(/[^a-zA-Z0-9_]/g, '');
        }

        // Add normalized search fields
        user.searchName = (user.username || '').toLowerCase();
        user.displayName = user.username || user.email?.split('@')[0] || 'Anonymous';

        return user;
      })
      .addValidator(
        'validate_email_uniqueness',
        'Ensure email addresses are unique after normalization',
        async (context) => {
          // Simulation of validation logic
          return {
            valid: true,
            errors: [],
            warnings: ['Some duplicate emails may have been found']
          };
        }
      )
      .build();

    this.migrationManager.registerMigration(userDataNormalization);

    // Create a migration that handles relationship data
    const postRelationshipMigration = createMigration(
      'update_post_relationships',
      '2.1.0',
      'Update Post relationship structure'
    )
      .description('Restructure how posts relate to users and add engagement metrics')
      .addField('Post', 'engagementScore', {
        type: 'number',
        required: false,
        default: 0
      })
      .addField('Post', 'lastActivityAt', {
        type: 'number',
        required: false,
        default: Date.now()
      })
      .customOperation('Post', async (context) => {
        context.logger.info('Calculating engagement scores for all posts');
        
        // Simulate complex calculation across related models
        const posts = await context.databaseManager.getAllRecords('Post');
        
        for (const post of posts) {
          // Get related comments and likes
          const comments = await context.databaseManager.getRelatedRecords('Comment', 'postId', post.id);
          const likes = post.likeCount || 0;
          
          // Calculate engagement score
          const engagementScore = (comments.length * 2) + likes;
          const lastActivityAt = comments.length > 0 
            ? Math.max(...comments.map((c: any) => c.createdAt || 0))
            : post.createdAt || Date.now();

          post.engagementScore = engagementScore;
          post.lastActivityAt = lastActivityAt;

          await context.databaseManager.updateRecord('Post', post);
        }
      })
      .build();

    this.migrationManager.registerMigration(postRelationshipMigration);

    console.log('Created complex data transformation migrations');
    console.log('- User data normalization');
    console.log('- Post relationship updates with engagement scoring');

    console.log('');
  }

  async rollbackAndRecoveryExamples(): Promise<void> {
    console.log('↩️  Rollback and Recovery Examples');
    console.log('==================================\n');

    // Create a migration that might fail
    const riskyMigration = createMigration(
      'risky_data_migration',
      '2.2.0',
      'Risky data migration (demonstration)'
    )
      .description('A migration that demonstrates rollback capabilities')
      .addField('User', 'tempField', {
        type: 'string',
        required: false,
        default: 'temp'
      })
      .customOperation('User', async (context) => {
        context.logger.info('Performing risky operation that might fail');
        
        // Simulate a 50% chance of failure for demonstration
        if (Math.random() > 0.5) {
          throw new Error('Simulated operation failure for rollback demonstration');
        }

        context.logger.info('Risky operation completed successfully');
      })
      .build();

    this.migrationManager.registerMigration(riskyMigration);

    try {
      console.log('Running risky migration (may fail)...');
      const result = await this.migrationManager.runMigration(riskyMigration.id);
      console.log(`Migration result: ${result.success ? 'SUCCESS' : 'FAILED'}`);
      
      if (result.success) {
        console.log('Migration succeeded, demonstrating rollback...');
        
        // Demonstrate manual rollback
        const rollbackResult = await this.migrationManager.rollbackMigration(riskyMigration.id);
        console.log(`Rollback result: ${rollbackResult.success ? 'SUCCESS' : 'FAILED'}`);
        console.log(`Rollback duration: ${rollbackResult.duration}ms`);
      }
      
    } catch (error) {
      console.log(`Migration failed as expected: ${error}`);
      
      // Check migration history
      const history = this.migrationManager.getMigrationHistory(riskyMigration.id);
      console.log(`Migration attempts: ${history.length}`);
      
      if (history.length > 0) {
        const lastAttempt = history[history.length - 1];
        console.log(`Last attempt result: ${lastAttempt.success ? 'SUCCESS' : 'FAILED'}`);
        console.log(`Rollback available: ${lastAttempt.rollbackAvailable}`);
      }
    }

    // Demonstrate recovery scenarios
    console.log('\nDemonstrating recovery scenarios...');
    
    const recoveryMigration = createMigration(
      'recovery_migration',
      '2.3.0',
      'Recovery migration with validation'
    )
      .description('Migration with comprehensive pre and post validation')
      .addValidator(
        'pre_migration_check',
        'Validate system state before migration',
        async (context) => {
          context.logger.info('Running pre-migration validation');
          return {
            valid: true,
            errors: [],
            warnings: ['System is ready for migration']
          };
        }
      )
      .addField('Post', 'recoveryField', {
        type: 'string',
        required: false,
        default: 'recovered'
      })
      .addValidator(
        'post_migration_check',
        'Validate migration results',
        async (context) => {
          context.logger.info('Running post-migration validation');
          return {
            valid: true,
            errors: [],
            warnings: ['Migration completed successfully']
          };
        }
      )
      .build();

    this.migrationManager.registerMigration(recoveryMigration);
    console.log('Created recovery migration with validation');

    console.log('');
  }

  async performanceOptimizationExamples(): Promise<void> {
    console.log('🚀 Performance Optimization Migration Examples');
    console.log('===============================================\n');

    // Create migrations that optimize different aspects
    const indexOptimization = createMigration(
      'optimize_search_indexes',
      '3.0.0',
      'Optimize search and query performance'
    )
      .description('Add indexes and computed fields for better query performance')
      .createIndex('User', ['email'], { unique: true, name: 'user_email_unique' })
      .createIndex('User', ['username'], { unique: true, name: 'user_username_unique' })
      .createIndex('Post', ['userId', 'createdAt'], { name: 'user_posts_timeline' })
      .createIndex('Post', ['isPublic', 'popularityScore'], { name: 'public_popular_posts' })
      .createIndex('Comment', ['postId', 'createdAt'], { name: 'post_comments_timeline' })
      .build();

    const dataArchiving = createMigration(
      'archive_old_data',
      '3.1.0',
      'Archive old inactive data'
    )
      .description('Move old inactive data to archive tables for better performance')
      .addField('Post', 'isArchived', {
        type: 'boolean',
        required: false,
        default: false
      })
      .addField('Comment', 'isArchived', {
        type: 'boolean',
        required: false,
        default: false
      })
      .customOperation('Post', async (context) => {
        context.logger.info('Archiving old posts');
        
        const cutoffDate = Date.now() - (365 * 24 * 60 * 60 * 1000); // 1 year ago
        const posts = await context.databaseManager.getAllRecords('Post');
        
        let archivedCount = 0;
        for (const post of posts) {
          if ((post.lastActivityAt || post.createdAt || 0) < cutoffDate && 
              (post.engagementScore || 0) < 5) {
            post.isArchived = true;
            await context.databaseManager.updateRecord('Post', post);
            archivedCount++;
          }
        }
        
        context.logger.info(`Archived ${archivedCount} old posts`);
      })
      .build();

    const cacheOptimization = createMigration(
      'optimize_cache_fields',
      '3.2.0',
      'Add cache-friendly computed fields'
    )
      .description('Add denormalized fields to reduce query complexity')
      .addField('User', 'postCount', {
        type: 'number',
        required: false,
        default: 0
      })
      .addField('User', 'totalEngagement', {
        type: 'number',
        required: false,
        default: 0
      })
      .addField('Post', 'commentCount', {
        type: 'number',
        required: false,
        default: 0
      })
      .customOperation('User', async (context) => {
        context.logger.info('Computing user statistics');
        
        const users = await context.databaseManager.getAllRecords('User');
        
        for (const user of users) {
          const posts = await context.databaseManager.getRelatedRecords('Post', 'userId', user.id);
          const totalEngagement = posts.reduce((sum: number, post: any) => 
            sum + (post.engagementScore || 0), 0);
          
          user.postCount = posts.length;
          user.totalEngagement = totalEngagement;
          
          await context.databaseManager.updateRecord('User', user);
        }
      })
      .build();

    // Register performance migrations
    [indexOptimization, dataArchiving, cacheOptimization].forEach(migration => {
      this.migrationManager.registerMigration(migration);
      console.log(`✅ Registered: ${migration.name}`);
    });

    console.log('\nPerformance optimization migrations created:');
    console.log('- Search index optimization');
    console.log('- Data archiving for old content');
    console.log('- Cache-friendly denormalized fields');

    console.log('');
  }

  async crossModelMigrationExamples(): Promise<void> {
    console.log('🔗 Cross-Model Migration Examples');
    console.log('=================================\n');

    // Migration that affects multiple models and their relationships
    const relationshipRestructure = createMigration(
      'restructure_follow_system',
      '4.0.0',
      'Restructure follow system with categories'
    )
      .description('Add follow categories and mutual follow detection')
      .addField('Follow', 'category', {
        type: 'string',
        required: false,
        default: 'general'
      })
      .addField('Follow', 'isMutual', {
        type: 'boolean',
        required: false,
        default: false
      })
      .addField('Follow', 'strength', {
        type: 'number',
        required: false,
        default: 1
      })
      .customOperation('Follow', async (context) => {
        context.logger.info('Analyzing follow relationships');
        
        const follows = await context.databaseManager.getAllRecords('Follow');
        const mutualMap = new Map<string, Set<string>>();
        
        // Build mutual follow map
        follows.forEach((follow: any) => {
          if (!mutualMap.has(follow.followerId)) {
            mutualMap.set(follow.followerId, new Set());
          }
          mutualMap.get(follow.followerId)!.add(follow.followingId);
        });
        
        // Update mutual status
        for (const follow of follows) {
          const reverseExists = mutualMap.get(follow.followingId)?.has(follow.followerId);
          follow.isMutual = Boolean(reverseExists);
          
          // Calculate relationship strength based on mutual status and activity
          follow.strength = follow.isMutual ? 2 : 1;
          
          await context.databaseManager.updateRecord('Follow', follow);
        }
      })
      .build();

    const contentCategorization = createMigration(
      'add_content_categories',
      '4.1.0',
      'Add content categorization system'
    )
      .description('Add categories and tags to posts and improve content discovery')
      .addField('Post', 'category', {
        type: 'string',
        required: false,
        default: 'general'
      })
      .addField('Post', 'subcategory', {
        type: 'string',
        required: false
      })
      .addField('Post', 'autoTags', {
        type: 'array',
        required: false,
        default: []
      })
      .transformData('Post', (post) => {
        // Auto-categorize posts based on content
        const content = (post.content || '').toLowerCase();
        let category = 'general';
        let autoTags: string[] = [];
        
        if (content.includes('tech') || content.includes('programming')) {
          category = 'technology';
          autoTags.push('tech');
        } else if (content.includes('art') || content.includes('design')) {
          category = 'creative';
          autoTags.push('art');
        } else if (content.includes('news') || content.includes('update')) {
          category = 'news';
          autoTags.push('news');
        }
        
        // Extract hashtags as auto tags
        const hashtags = content.match(/#\w+/g) || [];
        autoTags.push(...hashtags.map(tag => tag.slice(1)));
        
        return {
          ...post,
          category,
          autoTags: [...new Set(autoTags)] // Remove duplicates
        };
      })
      .build();

    // Register cross-model migrations
    [relationshipRestructure, contentCategorization].forEach(migration => {
      this.migrationManager.registerMigration(migration);
      console.log(`✅ Registered: ${migration.name}`);
    });

    console.log('\nCross-model migrations demonstrate:');
    console.log('- Complex relationship analysis and updates');
    console.log('- Multi-model data transformation');
    console.log('- Automatic content categorization');

    console.log('');
  }

  async versionManagementExamples(): Promise<void> {
    console.log('📋 Version Management Examples');
    console.log('==============================\n');

    // Demonstrate migration ordering and dependencies
    const allMigrations = this.migrationManager.getMigrations();
    
    console.log('Migration dependency chain:');
    allMigrations.forEach(migration => {
      const deps = migration.dependencies?.join(', ') || 'None';
      console.log(`- ${migration.name} (v${migration.version}) depends on: ${deps}`);
    });

    // Show pending migrations in order
    const pendingMigrations = this.migrationManager.getPendingMigrations();
    console.log(`\nPending migrations (${pendingMigrations.length}):`);
    pendingMigrations.forEach((migration, index) => {
      console.log(`${index + 1}. ${migration.name} (v${migration.version})`);
    });

    // Demonstrate batch migration with different strategies
    console.log('\nRunning pending migrations with different strategies:');
    
    if (pendingMigrations.length > 0) {
      console.log('\n1. Dry run all pending migrations:');
      try {
        const dryRunResults = await this.migrationManager.runPendingMigrations({
          dryRun: true,
          stopOnError: false
        });
        
        console.log(`Dry run completed: ${dryRunResults.length} migrations processed`);
        dryRunResults.forEach(result => {
          console.log(`- ${result.migrationId}: ${result.success ? 'SUCCESS' : 'FAILED'}`);
        });
        
      } catch (error) {
        console.error(`Dry run failed: ${error}`);
      }

      console.log('\n2. Run migrations with stop-on-error:');
      try {
        const results = await this.migrationManager.runPendingMigrations({
          stopOnError: true,
          batchSize: 25
        });
        
        console.log(`Migration batch completed: ${results.length} migrations`);
        
      } catch (error) {
        console.error(`Migration batch stopped due to error: ${error}`);
      }
    }

    // Show migration history and statistics
    const history = this.migrationManager.getMigrationHistory();
    console.log(`\nMigration history (${history.length} total runs):`);
    
    history.slice(0, 5).forEach(result => {
      console.log(`- ${result.migrationId}: ${result.success ? 'SUCCESS' : 'FAILED'} ` +
                 `(${result.duration}ms, ${result.recordsProcessed} records)`);
    });

    // Show active migrations (should be empty in examples)
    const activeMigrations = this.migrationManager.getActiveMigrations();
    console.log(`\nActive migrations: ${activeMigrations.length}`);

    console.log('');
  }

  async demonstrateAdvancedFeatures(): Promise<void> {
    console.log('🔬 Advanced Migration Features');
    console.log('==============================\n');

    // Create a migration with complex validation
    const complexValidation = createMigration(
      'complex_validation_example',
      '5.0.0',
      'Migration with complex validation'
    )
      .description('Demonstrates advanced validation and error handling')
      .addValidator(
        'check_data_consistency',
        'Verify data consistency across models',
        async (context) => {
          const errors: string[] = [];
          const warnings: string[] = [];
          
          // Simulate complex validation
          const users = await context.databaseManager.getAllRecords('User');
          const posts = await context.databaseManager.getAllRecords('Post');
          
          // Check for orphaned posts
          const userIds = new Set(users.map((u: any) => u.id));
          const orphanedPosts = posts.filter((p: any) => !userIds.has(p.userId));
          
          if (orphanedPosts.length > 0) {
            warnings.push(`Found ${orphanedPosts.length} orphaned posts`);
          }
          
          return { valid: errors.length === 0, errors, warnings };
        }
      )
      .addField('User', 'validationField', {
        type: 'string',
        required: false,
        default: 'validated'
      })
      .build();

    // Create a migration that handles large datasets
    const largeMigration = createMigration(
      'large_dataset_migration',
      '5.1.0',
      'Migration optimized for large datasets'
    )
      .description('Demonstrates batch processing and progress tracking')
      .customOperation('Post', async (context) => {
        context.logger.info('Processing large dataset with progress tracking');
        
        const totalRecords = 10000; // Simulate large dataset
        const batchSize = 100;
        
        for (let i = 0; i < totalRecords; i += batchSize) {
          const progress = ((i / totalRecords) * 100).toFixed(1);
          context.logger.info(`Processing batch ${i / batchSize + 1}, Progress: ${progress}%`);
          
          // Simulate processing time
          await new Promise(resolve => setTimeout(resolve, 10));
          
          context.progress.processedRecords = i + batchSize;
          context.progress.estimatedTimeRemaining = 
            ((totalRecords - i) / batchSize) * 10; // Rough estimate
        }
      })
      .build();

    console.log('Created advanced feature demonstrations:');
    console.log('- Complex multi-model validation');
    console.log('- Large dataset processing with progress tracking');
    console.log('- Error handling and recovery strategies');

    console.log('');
  }
}

// Usage function
export async function runMigrationExamples(
  orbitDBService: any,
  ipfsService: any
): Promise<void> {
  const framework = new SocialPlatformFramework();

  try {
    await framework.initialize(orbitDBService, ipfsService, 'development');

    // Create sample data first
    await createSampleDataForMigrations(framework);

    // Run migration examples
    const examples = new MigrationExamples(framework);
    await examples.runAllExamples();
    await examples.demonstrateAdvancedFeatures();

    // Show final migration statistics
    const migrationManager = (examples as any).migrationManager;
    const allMigrations = migrationManager.getMigrations();
    const history = migrationManager.getMigrationHistory();

    console.log('📊 Final Migration Statistics:');
    console.log('=============================');
    console.log(`Total migrations registered: ${allMigrations.length}`);
    console.log(`Total migration runs: ${history.length}`);
    console.log(`Successful runs: ${history.filter((h: any) => h.success).length}`);
    console.log(`Failed runs: ${history.filter((h: any) => !h.success).length}`);
    
    const totalDuration = history.reduce((sum: number, h: any) => sum + h.duration, 0);
    console.log(`Total migration time: ${totalDuration}ms`);
    
    const totalRecords = history.reduce((sum: number, h: any) => sum + h.recordsProcessed, 0);
    console.log(`Total records processed: ${totalRecords}`);

  } catch (error) {
    console.error('❌ Migration examples failed:', error);
  } finally {
    await framework.stop();
  }
}

async function createSampleDataForMigrations(framework: SocialPlatformFramework): Promise<void> {
  console.log('🗄️  Creating sample data for migration testing...\n');

  try {
    // Create users without timestamps (to demonstrate migration)
    const users = [];
    for (let i = 0; i < 5; i++) {
      const user = await framework.createUser({
        username: `migrationuser${i}`,
        email: `migration${i}@example.com`,
        bio: `Migration test user ${i}`
      });
      users.push(user);
    }

    // Create posts with basic structure
    const posts = [];
    for (let i = 0; i < 10; i++) {
      const user = users[i % users.length];
      const post = await framework.createPost(user.id, {
        title: `Migration Test Post ${i}`,
        content: `This is test content for migration testing. Post ${i} with various content types.`,
        tags: ['migration', 'test'],
        isPublic: true
      });
      posts.push(post);
    }

    // Create comments
    for (let i = 0; i < 15; i++) {
      const user = users[i % users.length];
      const post = posts[i % posts.length];
      await framework.createComment(
        user.id,
        post.id,
        `Migration test comment ${i}`
      );
    }

    // Create follow relationships
    for (let i = 0; i < users.length; i++) {
      for (let j = 0; j < users.length; j++) {
        if (i !== j && Math.random() > 0.6) {
          await framework.followUser(users[i].id, users[j].id);
        }
      }
    }

    console.log(`✅ Created sample data: ${users.length} users, ${posts.length} posts, 15 comments\n`);

  } catch (error) {
    console.warn('⚠️  Some sample data creation failed:', error);
  }
}