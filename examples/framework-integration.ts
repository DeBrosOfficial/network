/**
 * Example: Integrating DebrosFramework with existing OrbitDB/IPFS services
 * 
 * This example shows how to:
 * 1. Initialize the framework with your existing services
 * 2. Create models with different scopes and configurations
 * 3. Use the framework for CRUD operations
 * 4. Handle user-scoped vs global data
 */

import { 
  BaseModel, 
  Model, 
  Field, 
  BelongsTo, 
  HasMany,
  ModelRegistry,
  DatabaseManager,
  ShardManager,
  FrameworkOrbitDBService,
  FrameworkIPFSService,
  ConfigManager,
  QueryCache,
  RelationshipManager
} from '../src/framework';

// Example models for a social platform
@Model({ 
  scope: 'global',
  type: 'docstore',
  pinning: { strategy: 'fixed', factor: 3 }
})
export class User extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  username!: string;

  @Field({ type: 'string', required: true })
  email!: string;

  @Field({ type: 'string', required: false })
  bio?: string;

  @Field({ type: 'number', default: 0 })
  followerCount!: number;

  @HasMany(Post, 'userId')
  posts!: Post[];

  @HasMany(Follow, 'followerId')
  following!: Follow[];
}

@Model({ 
  scope: 'user',
  type: 'docstore',
  pinning: { strategy: 'popularity', factor: 2 },
  sharding: { strategy: 'hash', count: 4, key: 'id' }
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

  @BelongsTo(User, 'userId')
  author!: User;

  @HasMany(Comment, 'postId')
  comments!: Comment[];
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

  @BelongsTo(User, 'userId')
  author!: User;

  @BelongsTo(Post, 'postId')
  post!: Post;
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

  @BelongsTo(User, 'followerId')
  follower!: User;

  @BelongsTo(User, 'followingId')
  following!: User;
}

// Framework Integration Class
export class SocialPlatformFramework {
  private databaseManager!: DatabaseManager;
  private shardManager!: ShardManager;
  private configManager!: ConfigManager;
  private queryCache!: QueryCache;
  private relationshipManager!: RelationshipManager;
  private initialized: boolean = false;

  async initialize(
    existingOrbitDBService: any, 
    existingIPFSService: any,
    environment: 'development' | 'production' | 'test' = 'development'
  ): Promise<void> {
    console.log('🚀 Initializing Social Platform Framework...');

    // Create configuration based on environment
    let config;
    switch (environment) {
      case 'production':
        config = ConfigManager.productionConfig();
        break;
      case 'test':
        config = ConfigManager.testConfig();
        break;
      default:
        config = ConfigManager.developmentConfig();
    }

    this.configManager = new ConfigManager(config);

    // Wrap existing services
    const frameworkOrbitDB = new FrameworkOrbitDBService(existingOrbitDBService);
    const frameworkIPFS = new FrameworkIPFSService(existingIPFSService);

    // Initialize services
    await frameworkOrbitDB.init();
    await frameworkIPFS.init();

    // Create framework components
    this.databaseManager = new DatabaseManager(frameworkOrbitDB);
    this.shardManager = new ShardManager();
    this.shardManager.setOrbitDBService(frameworkOrbitDB);

    // Initialize databases for all registered models
    await this.databaseManager.initializeAllDatabases();

    // Create shards for global models that need them
    const globalModels = ModelRegistry.getGlobalModels();
    for (const model of globalModels) {
      if (model.sharding) {
        await this.shardManager.createShards(
          model.modelName, 
          model.sharding, 
          model.dbType
        );
      }
    }

    // Create global indexes for user-scoped models
    const userModels = ModelRegistry.getUserScopedModels();
    for (const model of userModels) {
      const indexName = `${model.modelName}GlobalIndex`;
      await this.shardManager.createGlobalIndex(model.modelName, indexName);
    }

    // Initialize query cache
    const cacheConfig = this.configManager.cacheConfig;
    this.queryCache = new QueryCache(
      cacheConfig?.maxSize || 1000,
      cacheConfig?.ttl || 300000
    );

    // Initialize relationship manager
    this.relationshipManager = new RelationshipManager({
      databaseManager: this.databaseManager,
      shardManager: this.shardManager,
      queryCache: this.queryCache
    });

    // Store framework instance globally for BaseModel access
    (globalThis as any).__debrosFramework = {
      databaseManager: this.databaseManager,
      shardManager: this.shardManager,
      configManager: this.configManager,
      queryCache: this.queryCache,
      relationshipManager: this.relationshipManager
    };

    this.initialized = true;
    console.log('✅ Social Platform Framework initialized successfully!');
  }

  async createUser(userData: { username: string; email: string; bio?: string }): Promise<User> {
    if (!this.initialized) {
      throw new Error('Framework not initialized');
    }

    // Create user in global database
    const user = new User(userData);
    await user.save();

    // Create user-specific databases
    await this.databaseManager.createUserDatabases(user.id);

    console.log(`👤 Created user: ${user.username} (${user.id})`);
    return user;
  }

  async createPost(
    userId: string, 
    postData: { title: string; content: string; tags?: string[]; isPublic?: boolean }
  ): Promise<Post> {
    if (!this.initialized) {
      throw new Error('Framework not initialized');
    }

    const post = new Post({
      ...postData,
      userId
    });

    await post.save();

    // Add to global index for cross-user queries
    const globalIndexName = 'PostGlobalIndex';
    await this.shardManager.addToGlobalIndex(globalIndexName, post.id, {
      id: post.id,
      userId: post.userId,
      title: post.title,
      isPublic: post.isPublic,
      createdAt: post.createdAt,
      tags: post.tags
    });

    console.log(`📝 Created post: ${post.title} by user ${userId}`);
    return post;
  }

  async createComment(
    userId: string,
    postId: string,
    content: string
  ): Promise<Comment> {
    if (!this.initialized) {
      throw new Error('Framework not initialized');
    }

    const comment = new Comment({
      content,
      userId,
      postId
    });

    await comment.save();

    console.log(`💬 Created comment on post ${postId} by user ${userId}`);
    return comment;
  }

  async followUser(followerId: string, followingId: string): Promise<Follow> {
    if (!this.initialized) {
      throw new Error('Framework not initialized');
    }

    const follow = new Follow({
      followerId,
      followingId
    });

    await follow.save();

    console.log(`👥 User ${followerId} followed user ${followingId}`);
    return follow;
  }

  // Fully functional query methods
  async getPublicPosts(limit: number = 10): Promise<Post[]> {
    console.log(`🔍 Querying for ${limit} public posts...`);
    
    return await Post
      .where('isPublic', '=', true)
      .orderBy('createdAt', 'desc')
      .limit(limit)
      .exec();
  }

  async getUserPosts(userId: string, limit: number = 20): Promise<Post[]> {
    console.log(`🔍 Getting posts for user ${userId}...`);
    
    return await Post
      .whereUser(userId)
      .orderBy('createdAt', 'desc')
      .limit(limit)
      .exec();
  }

  async searchPosts(searchTerm: string, limit: number = 50): Promise<Post[]> {
    console.log(`🔍 Searching posts for: ${searchTerm}`);
    
    return await Post
      .where('isPublic', '=', true)
      .orWhere(query => {
        query.whereLike('title', searchTerm)
             .whereLike('content', searchTerm);
      })
      .orderBy('createdAt', 'desc')
      .limit(limit)
      .exec();
  }

  async getPostsWithComments(userId: string, limit: number = 10): Promise<Post[]> {
    console.log(`🔍 Getting posts with comments for user ${userId}...`);
    
    const posts = await Post
      .whereUser(userId)
      .orderBy('createdAt', 'desc')
      .limit(limit)
      .exec();

    // Load relationships for all posts
    await this.relationshipManager.eagerLoadRelationships(posts, ['comments', 'author']);
    
    return posts;
  }

  async getPostsWithFilteredComments(userId: string, minCommentLength: number = 10): Promise<Post[]> {
    console.log(`🔍 Getting posts with filtered comments for user ${userId}...`);
    
    const posts = await Post
      .whereUser(userId)
      .orderBy('createdAt', 'desc')
      .limit(10)
      .exec();

    // Load comments with constraints
    for (const post of posts) {
      await post.loadRelationWithConstraints('comments', (query) => 
        query.where('content', '>', minCommentLength)
             .orderBy('createdAt', 'desc')
             .limit(5)
      );
      
      // Also load the author
      await post.loadRelation('author');
    }
    
    return posts;
  }

  async getUserStats(userId: string): Promise<any> {
    console.log(`📊 Getting stats for user ${userId}...`);
    
    const [postCount, totalLikes] = await Promise.all([
      Post.whereUser(userId).count(),
      Post.whereUser(userId).sum('likeCount')
    ]);

    return {
      userId,
      postCount,
      totalLikes,
      averageLikes: postCount > 0 ? totalLikes / postCount : 0
    };
  }

  async getFrameworkStats(): Promise<any> {
    if (!this.initialized) {
      throw new Error('Framework not initialized');
    }

    const stats = {
      initialized: this.initialized,
      registeredModels: ModelRegistry.getModelNames(),
      globalModels: ModelRegistry.getGlobalModels().map(m => m.name),
      userScopedModels: ModelRegistry.getUserScopedModels().map(m => m.name),
      shardsInfo: this.shardManager.getAllModelsWithShards().map(modelName => 
        this.shardManager.getShardStatistics(modelName)
      ),
      config: this.configManager.getConfig(),
      cache: {
        query: {
          stats: this.queryCache.getStats(),
          usage: this.queryCache.analyzeUsage(),
          popular: this.queryCache.getPopularEntries(5)
        },
        relationships: this.relationshipManager.getRelationshipCacheStats()
      }
    };

    return stats;
  }

  async explainQuery(query: any): Promise<any> {
    console.log(`📊 Analyzing query...`);
    return query.explain();
  }

  async warmupCache(): Promise<void> {
    console.log(`🔥 Warming up caches...`);
    
    // Warm up query cache
    const commonQueries = [
      Post.where('isPublic', '=', true).orderBy('createdAt', 'desc').limit(10),
      User.orderBy('followerCount', 'desc').limit(20),
      Follow.limit(100)
    ];

    await this.queryCache.warmup(commonQueries);

    // Warm up relationship cache
    const users = await User.limit(5).exec();
    const posts = await Post.where('isPublic', '=', true).limit(10).exec();
    
    if (users.length > 0) {
      await this.relationshipManager.warmupRelationshipCache(users, ['posts', 'following']);
    }
    
    if (posts.length > 0) {
      await this.relationshipManager.warmupRelationshipCache(posts, ['author', 'comments']);
    }
  }

  async stop(): Promise<void> {
    if (!this.initialized) {
      return;
    }

    console.log('🛑 Stopping Social Platform Framework...');

    await this.databaseManager.stop();
    await this.shardManager.stop();
    this.queryCache.clear();
    this.relationshipManager.clearRelationshipCache();

    // Clear global reference
    delete (globalThis as any).__debrosFramework;

    this.initialized = false;
    console.log('✅ Framework stopped successfully');
  }
}

// Example usage function
export async function exampleUsage(orbitDBService: any, ipfsService: any) {
  const framework = new SocialPlatformFramework();
  
  try {
    // Initialize framework with existing services
    await framework.initialize(orbitDBService, ipfsService, 'development');

    // Create some users
    const alice = await framework.createUser({
      username: 'alice',
      email: 'alice@example.com',
      bio: 'Love decentralized tech!'
    });

    const bob = await framework.createUser({
      username: 'bob',
      email: 'bob@example.com',
      bio: 'Building the future'
    });

    // Create posts
    const post1 = await framework.createPost(alice.id, {
      title: 'Welcome to the Decentralized Web',
      content: 'This is my first post using the DebrosFramework!',
      tags: ['web3', 'decentralized', 'orbitdb'],
      isPublic: true
    });

    const post2 = await framework.createPost(bob.id, {
      title: 'Framework Architecture',
      content: 'The new framework handles database partitioning automatically.',
      tags: ['framework', 'architecture'],
      isPublic: true
    });

    // Create comments
    await framework.createComment(bob.id, post1.id, 'Great post Alice!');
    await framework.createComment(alice.id, post2.id, 'Thanks for building this!');

    // Follow users
    await framework.followUser(alice.id, bob.id);

    // Get framework statistics
    const stats = await framework.getFrameworkStats();
    console.log('📊 Framework Statistics:', JSON.stringify(stats, null, 2));

    console.log('✅ Example usage completed successfully!');

    return { framework, users: { alice, bob }, posts: { post1, post2 } };
  } catch (error) {
    console.error('❌ Example usage failed:', error);
    await framework.stop();
    throw error;
  }
}

export { SocialPlatformFramework };