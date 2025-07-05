/**
 * Complete DebrosFramework Example
 * 
 * This example demonstrates the complete DebrosFramework in action,
 * showcasing all major features and capabilities in a real-world scenario:
 * - Framework initialization with all components
 * - Model definition with decorators and relationships
 * - Database operations and querying
 * - Automatic features (pinning, PubSub, caching)
 * - Migration system for schema evolution
 * - Performance monitoring and optimization
 * - Error handling and recovery
 */

import { 
  DebrosFramework,
  BaseModel,
  Model,
  Field,
  BelongsTo,
  HasMany,
  BeforeCreate,
  AfterCreate,
  createMigration,
  DEVELOPMENT_CONFIG,
  PRODUCTION_CONFIG
} from '../src/framework';

// Define comprehensive models for a decentralized social platform

@Model({
  scope: 'global',
  type: 'docstore',
  pinning: { strategy: 'fixed', factor: 3 },
  sharding: { strategy: 'hash', count: 8, key: 'id' }
})
export class User extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  username!: string;

  @Field({ type: 'string', required: true, unique: true })
  email!: string;

  @Field({ type: 'string', required: false })
  bio?: string;

  @Field({ type: 'string', required: false })
  profilePicture?: string;

  @Field({ type: 'boolean', default: false })
  isVerified!: boolean;

  @Field({ type: 'number', default: 0 })
  followerCount!: number;

  @Field({ type: 'number', default: 0 })
  followingCount!: number;

  @Field({ type: 'object', default: {} })
  settings!: any;

  @HasMany(Post, 'userId')
  posts!: Post[];

  @HasMany(Follow, 'followerId')
  following!: Follow[];

  @BeforeCreate()
  async validateUser() {
    if (this.username.length < 3) {
      throw new Error('Username must be at least 3 characters long');
    }
    
    if (!this.email.includes('@')) {
      throw new Error('Invalid email format');
    }
  }

  @AfterCreate()
  async setupUserDefaults() {
    this.settings = {
      theme: 'light',
      notifications: true,
      privacy: 'public',
      createdAt: Date.now()
    };
  }

  // Custom methods
  async updateProfile(updates: { bio?: string; profilePicture?: string }): Promise<void> {
    Object.assign(this, updates);
    await this.save();
  }

  async getPopularPosts(limit: number = 10): Promise<Post[]> {
    return await Post
      .whereUser(this.id)
      .where('isPublic', '=', true)
      .orderBy('likeCount', 'desc')
      .limit(limit)
      .exec();
  }
}

@Model({
  scope: 'user',
  type: 'docstore',
  pinning: { strategy: 'popularity', factor: 1.5 }
})
export class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title!: string;

  @Field({ type: 'string', required: true })
  content!: string;

  @Field({ type: 'string', required: true })
  userId!: string;

  @Field({ type: 'boolean', default: true })
  isPublic!: boolean;

  @Field({ type: 'array', default: [] })
  tags!: string[];

  @Field({ type: 'number', default: 0 })
  likeCount!: number;

  @Field({ type: 'number', default: 0 })
  commentCount!: number;

  @Field({ type: 'string', default: 'text' })
  contentType!: string;

  @Field({ type: 'object', default: {} })
  metadata!: any;

  @BelongsTo(User, 'userId')
  author!: User;

  @HasMany(Comment, 'postId')
  comments!: Comment[];

  @BeforeCreate()
  async processContent() {
    // Auto-detect content type and extract metadata
    this.metadata = {
      wordCount: this.content.split(' ').length,
      hasLinks: /https?:\/\//.test(this.content),
      hashtags: this.extractHashtags(),
      readTime: Math.ceil(this.content.split(' ').length / 200) // Reading speed
    };

    if (this.metadata.hasLinks) {
      this.contentType = 'rich';
    }
  }

  private extractHashtags(): string[] {
    const hashtags = this.content.match(/#\w+/g) || [];
    return hashtags.map(tag => tag.slice(1).toLowerCase());
  }

  async toggleLike(): Promise<void> {
    this.likeCount += 1;
    await this.save();
  }

  async addComment(userId: string, content: string): Promise<Comment> {
    const comment = await Comment.create({
      content,
      userId,
      postId: this.id
    });

    this.commentCount += 1;
    await this.save();

    return comment;
  }
}

@Model({
  scope: 'user',
  type: 'docstore'
})
export class Comment extends BaseModel {
  @Field({ type: 'string', required: true })
  content!: string;

  @Field({ type: 'string', required: true })
  userId!: string;

  @Field({ type: 'string', required: true })
  postId!: string;

  @Field({ type: 'string', required: false })
  parentId?: string;

  @Field({ type: 'number', default: 0 })
  likeCount!: number;

  @Field({ type: 'number', default: 0 })
  threadDepth!: number;

  @BelongsTo(User, 'userId')
  author!: User;

  @BelongsTo(Post, 'postId')
  post!: Post;

  @BelongsTo(Comment, 'parentId')
  parent?: Comment;

  @HasMany(Comment, 'parentId')
  replies!: Comment[];
}

@Model({
  scope: 'global',
  type: 'keyvalue'
})
export class Follow extends BaseModel {
  @Field({ type: 'string', required: true })
  followerId!: string;

  @Field({ type: 'string', required: true })
  followingId!: string;

  @Field({ type: 'boolean', default: false })
  isMutual!: boolean;

  @Field({ type: 'string', default: 'general' })
  category!: string;

  @BelongsTo(User, 'followerId')
  follower!: User;

  @BelongsTo(User, 'followingId')
  following!: User;
}

export class CompleteFrameworkExample {
  private framework: DebrosFramework;
  private sampleUsers: User[] = [];
  private samplePosts: Post[] = [];

  constructor() {
    // Initialize framework with comprehensive configuration
    this.framework = new DebrosFramework({
      ...DEVELOPMENT_CONFIG,
      features: {
        autoMigration: true,
        automaticPinning: true,
        pubsub: true,
        queryCache: true,
        relationshipCache: true
      },
      performance: {
        queryTimeout: 30000,
        migrationTimeout: 300000,
        maxConcurrentOperations: 200,
        batchSize: 100
      },
      monitoring: {
        enableMetrics: true,
        logLevel: 'info',
        metricsInterval: 30000
      }
    });
  }

  async runCompleteExample(): Promise<void> {
    console.log('🎯 Running Complete DebrosFramework Example');
    console.log('==========================================\n');

    try {
      await this.initializeFramework();
      await this.setupModelsAndMigrations();
      await this.demonstrateModelOperations();
      await this.demonstrateQuerySystem();
      await this.demonstrateRelationships();
      await this.demonstrateAutomaticFeatures();
      await this.demonstratePerformanceOptimization();
      await this.demonstrateErrorHandling();
      await this.showFrameworkStatistics();

      console.log('✅ Complete framework example finished successfully!\n');

    } catch (error) {
      console.error('❌ Framework example failed:', error);
      throw error;
    } finally {
      await this.cleanup();
    }
  }

  async initializeFramework(): Promise<void> {
    console.log('🚀 Initializing DebrosFramework');
    console.log('===============================\n');

    // In a real application, you would pass actual OrbitDB and IPFS instances
    const mockOrbitDB = this.createMockOrbitDB();
    const mockIPFS = this.createMockIPFS();

    await this.framework.initialize(mockOrbitDB, mockIPFS);

    // Register models
    this.framework.registerModel(User);
    this.framework.registerModel(Post);
    this.framework.registerModel(Comment);
    this.framework.registerModel(Follow);

    console.log('Framework initialization completed');
    console.log(`Status: ${this.framework.getStatus().healthy ? 'Healthy' : 'Unhealthy'}`);
    console.log(`Environment: ${this.framework.getStatus().environment}`);
    console.log('');
  }

  async setupModelsAndMigrations(): Promise<void> {
    console.log('🔄 Setting Up Models and Migrations');
    console.log('===================================\n');

    // Create sample migrations
    const addProfileEnhancements = createMigration(
      'add_profile_enhancements',
      '1.1.0',
      'Add profile enhancements to User model'
    )
      .description('Add profile picture and verification status to users')
      .addField('User', 'profilePicture', {
        type: 'string',
        required: false
      })
      .addField('User', 'isVerified', {
        type: 'boolean',
        default: false
      })
      .build();

    const addPostMetadata = createMigration(
      'add_post_metadata',
      '1.2.0',
      'Add metadata to Post model'
    )
      .description('Add content metadata and engagement metrics')
      .addField('Post', 'contentType', {
        type: 'string',
        default: 'text'
      })
      .addField('Post', 'metadata', {
        type: 'object',
        default: {}
      })
      .transformData('Post', (post) => {
        return {
          ...post,
          metadata: {
            wordCount: post.content ? post.content.split(' ').length : 0,
            transformedAt: Date.now()
          }
        };
      })
      .build();

    // Register migrations
    await this.framework.registerMigration(addProfileEnhancements);
    await this.framework.registerMigration(addPostMetadata);

    // Run pending migrations
    const pendingMigrations = this.framework.getPendingMigrations();
    console.log(`Found ${pendingMigrations.length} pending migrations`);

    if (pendingMigrations.length > 0) {
      const migrationManager = this.framework.getMigrationManager();
      if (migrationManager) {
        const results = await migrationManager.runPendingMigrations({
          dryRun: false,
          stopOnError: true
        });
        console.log(`Completed ${results.filter(r => r.success).length} migrations`);
      }
    }

    console.log('');
  }

  async demonstrateModelOperations(): Promise<void> {
    console.log('👥 Demonstrating Model Operations');
    console.log('=================================\n');

    // Create users with validation and hooks
    console.log('Creating users...');
    for (let i = 0; i < 5; i++) {
      const user = await User.create({
        username: `frameuser${i}`,
        email: `frameuser${i}@example.com`,
        bio: `Framework test user ${i} with comprehensive features`,
        isVerified: i < 2 // First two users are verified
      });
      
      this.sampleUsers.push(user);
      console.log(`✅ Created user: ${user.username} (verified: ${user.isVerified})`);
    }

    // Create posts with automatic content processing
    console.log('\nCreating posts...');
    for (let i = 0; i < 10; i++) {
      const user = this.sampleUsers[i % this.sampleUsers.length];
      const post = await Post.create({
        title: `Framework Demo Post ${i + 1}`,
        content: `This is a comprehensive demo post ${i + 1} showcasing the DebrosFramework capabilities. #framework #demo #orbitdb ${i % 3 === 0 ? 'https://example.com' : ''}`,
        userId: user.id,
        isPublic: true,
        tags: ['framework', 'demo', 'test']
      });
      
      this.samplePosts.push(post);
      console.log(`✅ Created post: "${post.title}" by ${user.username}`);
      console.log(`   Content type: ${post.contentType}, Word count: ${post.metadata.wordCount}`);
    }

    // Create comments and follows
    console.log('\nCreating interactions...');
    let commentCount = 0;
    let followCount = 0;

    for (let i = 0; i < 15; i++) {
      const user = this.sampleUsers[Math.floor(Math.random() * this.sampleUsers.length)];
      const post = this.samplePosts[Math.floor(Math.random() * this.samplePosts.length)];
      
      await Comment.create({
        content: `This is comment ${i + 1} on the framework demo post. Great work!`,
        userId: user.id,
        postId: post.id
      });
      commentCount++;
    }

    // Create follow relationships
    for (let i = 0; i < this.sampleUsers.length; i++) {
      for (let j = 0; j < this.sampleUsers.length; j++) {
        if (i !== j && Math.random() > 0.6) {
          await Follow.create({
            followerId: this.sampleUsers[i].id,
            followingId: this.sampleUsers[j].id,
            category: 'general'
          });
          followCount++;
        }
      }
    }

    console.log(`✅ Created ${commentCount} comments and ${followCount} follow relationships`);
    console.log('');
  }

  async demonstrateQuerySystem(): Promise<void> {
    console.log('🔍 Demonstrating Advanced Query System');
    console.log('======================================\n');

    // Complex queries with caching
    console.log('1. Complex filtering and sorting:');
    const popularPosts = await Post
      .where('isPublic', '=', true)
      .where('likeCount', '>', 0)
      .orderBy('likeCount', 'desc')
      .orderBy('createdAt', 'desc')
      .limit(5)
      .exec();
    
    console.log(`Found ${popularPosts.length} popular posts`);

    // User-scoped queries
    console.log('\n2. User-scoped queries:');
    const userPosts = await Post
      .whereUser(this.sampleUsers[0].id)
      .where('isPublic', '=', true)
      .exec();
    
    console.log(`User ${this.sampleUsers[0].username} has ${userPosts.length} public posts`);

    // Aggregation queries
    console.log('\n3. Aggregation queries:');
    const totalPosts = await Post.count();
    const totalPublicPosts = await Post.where('isPublic', '=', true).count();
    const averageLikes = await Post.avg('likeCount');
    
    console.log(`Total posts: ${totalPosts}`);
    console.log(`Public posts: ${totalPublicPosts}`);
    console.log(`Average likes: ${averageLikes.toFixed(2)}`);

    // Query with relationships
    console.log('\n4. Queries with relationships:');
    const postsWithAuthors = await Post
      .where('isPublic', '=', true)
      .with(['author'])
      .limit(3)
      .exec();
    
    console.log('Posts with preloaded authors:');
    postsWithAuthors.forEach(post => {
      const author = post.getRelation('author');
      console.log(`- "${post.title}" by ${author ? author.username : 'Unknown'}`);
    });

    console.log('');
  }

  async demonstrateRelationships(): Promise<void> {
    console.log('🔗 Demonstrating Relationship System');
    console.log('====================================\n');

    const user = this.sampleUsers[0];
    const post = this.samplePosts[0];

    // Lazy loading
    console.log('1. Lazy loading relationships:');
    console.log(`Loading posts for user: ${user.username}`);
    const userPosts = await user.loadRelation('posts');
    console.log(`Loaded ${Array.isArray(userPosts) ? userPosts.length : 0} posts`);

    console.log(`\nLoading comments for post: "${post.title}"`);
    const comments = await post.loadRelation('comments');
    console.log(`Loaded ${Array.isArray(comments) ? comments.length : 0} comments`);

    // Eager loading
    console.log('\n2. Eager loading for multiple items:');
    const relationshipManager = this.framework.getRelationshipManager();
    if (relationshipManager) {
      await relationshipManager.eagerLoadRelationships(
        this.samplePosts.slice(0, 3),
        ['author', 'comments']
      );
      
      console.log('Eager loaded author and comments for 3 posts:');
      this.samplePosts.slice(0, 3).forEach((post, index) => {
        const author = post.getRelation('author');
        const comments = post.getRelation('comments') || [];
        console.log(`${index + 1}. "${post.title}" by ${author ? author.username : 'Unknown'} (${comments.length} comments)`);
      });
    }

    // Relationship constraints
    console.log('\n3. Constrained relationship loading:');
    const recentComments = await post.loadRelationWithConstraints('comments', (query) =>
      query.where('createdAt', '>', Date.now() - 86400000) // Last 24 hours
           .orderBy('createdAt', 'desc')
           .limit(3)
    );
    
    console.log(`Loaded ${Array.isArray(recentComments) ? recentComments.length : 0} recent comments`);

    console.log('');
  }

  async demonstrateAutomaticFeatures(): Promise<void> {
    console.log('🤖 Demonstrating Automatic Features');
    console.log('===================================\n');

    // Pinning system
    console.log('1. Automatic pinning system:');
    const pinningManager = this.framework.getPinningManager();
    if (pinningManager) {
      // Setup pinning rules
      pinningManager.setPinningRule('Post', {
        strategy: 'popularity',
        factor: 1.5,
        maxPins: 50
      });

      pinningManager.setPinningRule('User', {
        strategy: 'fixed',
        factor: 2.0,
        maxPins: 20
      });

      // Simulate content pinning
      for (let i = 0; i < 5; i++) {
        const post = this.samplePosts[i];
        const hash = `content-hash-${post.id}`;
        
        // Simulate access
        await pinningManager.recordAccess(hash);
        await pinningManager.recordAccess(hash);
        
        const pinned = await pinningManager.pinContent(hash, 'Post', post.id, {
          title: post.title,
          likeCount: post.likeCount
        });
        
        console.log(`Post "${post.title}": ${pinned ? 'PINNED' : 'NOT PINNED'}`);
      }

      const pinningStats = pinningManager.getStats();
      console.log(`Pinning stats: ${pinningStats.totalPinned} items pinned`);
    }

    // PubSub system
    console.log('\n2. PubSub event system:');
    const pubsubManager = this.framework.getPubSubManager();
    if (pubsubManager) {
      let eventCount = 0;
      
      // Subscribe to model events
      await pubsubManager.subscribe('model.*', (event) => {
        eventCount++;
        console.log(`📡 Event: ${event.type} for ${event.data?.modelName || 'unknown'}`);
      });

      // Simulate model events
      await pubsubManager.publish('model.created', {
        modelName: 'Post',
        modelId: 'demo-post-1',
        userId: 'demo-user-1'
      });

      await pubsubManager.publish('model.updated', {
        modelName: 'User',
        modelId: 'demo-user-1',
        changes: { bio: 'Updated bio' }
      });

      // Wait for event processing
      await new Promise(resolve => setTimeout(resolve, 1000));
      
      console.log(`Processed ${eventCount} events`);
      
      const pubsubStats = pubsubManager.getStats();
      console.log(`PubSub stats: ${pubsubStats.totalPublished} published, ${pubsubStats.totalReceived} received`);
    }

    console.log('');
  }

  async demonstratePerformanceOptimization(): Promise<void> {
    console.log('🚀 Demonstrating Performance Features');
    console.log('=====================================\n');

    // Cache warming
    console.log('1. Cache warming and optimization:');
    await this.framework.warmupCaches();

    // Query performance comparison
    console.log('\n2. Query performance comparison:');
    const startTime = Date.now();
    
    // First query (cold cache)
    await Post.where('isPublic', '=', true).limit(5).exec();
    const coldTime = Date.now() - startTime;
    
    const warmStartTime = Date.now();
    // Second query (warm cache)
    await Post.where('isPublic', '=', true).limit(5).exec();
    const warmTime = Date.now() - warmStartTime;
    
    console.log(`Cold cache query: ${coldTime}ms`);
    console.log(`Warm cache query: ${warmTime}ms`);
    console.log(`Performance improvement: ${coldTime > 0 ? (coldTime / Math.max(warmTime, 1)).toFixed(2) : 'N/A'}x`);

    // Relationship loading optimization
    console.log('\n3. Relationship loading optimization:');
    const relationshipManager = this.framework.getRelationshipManager();
    if (relationshipManager) {
      const stats = relationshipManager.getRelationshipCacheStats();
      console.log(`Relationship cache: ${stats.cache.totalEntries} entries`);
      console.log(`Cache hit rate: ${(stats.cache.hitRate * 100).toFixed(2)}%`);
    }

    console.log('');
  }

  async demonstrateErrorHandling(): Promise<void> {
    console.log('⚠️  Demonstrating Error Handling');
    console.log('=================================\n');

    // Validation errors
    console.log('1. Model validation errors:');
    try {
      await User.create({
        username: 'x', // Too short
        email: 'invalid-email' // Invalid format
      });
    } catch (error: any) {
      console.log(`✅ Caught validation error: ${error.message}`);
    }

    // Query errors
    console.log('\n2. Query timeout handling:');
    try {
      // Simulate slow query
      const result = await Post.where('nonExistentField', '=', 'value').exec();
      console.log(`Query result: ${result.length} items`);
    } catch (error: any) {
      console.log(`✅ Handled query error gracefully: ${error.message}`);
    }

    // Migration rollback
    console.log('\n3. Migration error recovery:');
    const migrationManager = this.framework.getMigrationManager();
    if (migrationManager) {
      try {
        const riskyMigration = createMigration(
          'risky_migration',
          '99.0.0',
          'Intentionally failing migration'
        )
          .customOperation('Post', async () => {
            throw new Error('Simulated migration failure');
          })
          .build();

        await migrationManager.registerMigration(riskyMigration);
        await migrationManager.runMigration(riskyMigration.id);
      } catch (error: any) {
        console.log(`✅ Migration failed as expected and rolled back: ${error.message}`);
      }
    }

    console.log('');
  }

  async showFrameworkStatistics(): Promise<void> {
    console.log('📊 Framework Statistics');
    console.log('=======================\n');

    const status = this.framework.getStatus();
    const metrics = this.framework.getMetrics();

    console.log('Status:');
    console.log(`- Initialized: ${status.initialized}`);
    console.log(`- Healthy: ${status.healthy}`);
    console.log(`- Version: ${status.version}`);
    console.log(`- Environment: ${status.environment}`);
    console.log(`- Services: ${Object.entries(status.services).map(([name, status]) => `${name}:${status}`).join(', ')}`);

    console.log('\nMetrics:');
    console.log(`- Uptime: ${(metrics.uptime / 1000).toFixed(2)} seconds`);
    console.log(`- Total models: ${metrics.totalModels}`);
    console.log(`- Queries executed: ${metrics.queriesExecuted}`);
    console.log(`- Migrations run: ${metrics.migrationsRun}`);
    console.log(`- Cache hit rate: ${(metrics.cacheHitRate * 100).toFixed(2)}%`);
    console.log(`- Average query time: ${metrics.averageQueryTime.toFixed(2)}ms`);

    console.log('\nMemory Usage:');
    console.log(`- Query cache: ${(metrics.memoryUsage.queryCache / 1024).toFixed(2)} KB`);
    console.log(`- Relationship cache: ${(metrics.memoryUsage.relationshipCache / 1024).toFixed(2)} KB`);
    console.log(`- Total: ${(metrics.memoryUsage.total / 1024).toFixed(2)} KB`);

    console.log('');
  }

  async cleanup(): Promise<void> {
    console.log('🧹 Cleaning up framework...');
    await this.framework.stop();
    console.log('✅ Framework stopped and cleaned up');
  }

  // Mock service creation (in real usage, these would be actual services)
  private createMockOrbitDB(): any {
    return {
      create: async () => ({ add: async () => {}, get: async () => [], all: async () => [] }),
      open: async () => ({ add: async () => {}, get: async () => [], all: async () => [] }),
      disconnect: async () => {},
      stores: {}
    };
  }

  private createMockIPFS(): any {
    return {
      add: async () => ({ cid: 'mock-cid' }),
      cat: async () => Buffer.from('mock data'),
      pin: { add: async () => {}, rm: async () => {} },
      pubsub: {
        subscribe: async () => {},
        unsubscribe: async () => {},
        publish: async () => {}
      },
      object: { stat: async () => ({ CumulativeSize: 1024 }) }
    };
  }
}

// Usage function
export async function runCompleteFrameworkExample(): Promise<void> {
  const example = new CompleteFrameworkExample();
  await example.runCompleteExample();
}

// Run if called directly
if (require.main === module) {
  runCompleteFrameworkExample().catch(console.error);
}