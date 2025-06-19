/**
 * Comprehensive Query Examples for DebrosFramework
 * 
 * This file demonstrates all the query capabilities implemented in Phase 3:
 * - Basic and advanced filtering
 * - User-scoped vs global queries
 * - Relationship loading
 * - Aggregations and analytics
 * - Query optimization and caching
 * - Pagination and chunked processing
 */

import { SocialPlatformFramework, User, Post, Comment, Follow } from './framework-integration';

export class QueryExamples {
  private framework: SocialPlatformFramework;

  constructor(framework: SocialPlatformFramework) {
    this.framework = framework;
  }

  async runAllExamples(): Promise<void> {
    console.log('🚀 Running comprehensive query examples...\n');

    await this.basicQueries();
    await this.userScopedQueries();
    await this.relationshipQueries();
    await this.aggregationQueries();
    await this.advancedFiltering();
    await this.paginationExamples();
    await this.cacheExamples();
    await this.optimizationExamples();

    console.log('✅ All query examples completed!\n');
  }

  async basicQueries(): Promise<void> {
    console.log('📊 Basic Query Examples');
    console.log('========================\n');

    // Simple equality
    const publicPosts = await Post
      .where('isPublic', '=', true)
      .limit(5)
      .exec();
    console.log(`Found ${publicPosts.length} public posts`);

    // Multiple conditions
    const recentPublicPosts = await Post
      .where('isPublic', '=', true)
      .where('createdAt', '>', Date.now() - 86400000) // Last 24 hours
      .orderBy('createdAt', 'desc')
      .limit(10)
      .exec();
    console.log(`Found ${recentPublicPosts.length} recent public posts`);

    // Using whereIn
    const specificUsers = await User
      .whereIn('username', ['alice', 'bob', 'charlie'])
      .exec();
    console.log(`Found ${specificUsers.length} specific users`);

    // Find by ID
    if (publicPosts.length > 0) {
      const singlePost = await Post.find(publicPosts[0].id);
      console.log(`Found post: ${singlePost?.title || 'Not found'}`);
    }

    console.log('');
  }

  async userScopedQueries(): Promise<void> {
    console.log('👤 User-Scoped Query Examples');
    console.log('==============================\n');

    // Get all users first
    const users = await User.limit(3).exec();
    if (users.length === 0) {
      console.log('No users found for user-scoped examples');
      return;
    }

    const userId = users[0].id;

    // Single user query (efficient - direct database access)
    const userPosts = await Post
      .whereUser(userId)
      .orderBy('createdAt', 'desc')
      .limit(10)
      .exec();
    console.log(`Found ${userPosts.length} posts for user ${userId}`);

    // Multiple users query
    const multiUserPosts = await Post
      .whereUserIn(users.map(u => u.id))
      .where('isPublic', '=', true)
      .limit(20)
      .exec();
    console.log(`Found ${multiUserPosts.length} posts from ${users.length} users`);

    // Global query on user-scoped data (uses global index)
    const allPublicPosts = await Post
      .where('isPublic', '=', true)
      .orderBy('createdAt', 'desc')
      .limit(15)
      .exec();
    console.log(`Found ${allPublicPosts.length} public posts across all users`);

    console.log('');
  }

  async relationshipQueries(): Promise<void> {
    console.log('🔗 Relationship Query Examples');
    console.log('===============================\n');

    // Load posts with their authors
    const postsWithAuthors = await Post
      .where('isPublic', '=', true)
      .load(['author'])
      .limit(5)
      .exec();
    console.log(`Loaded ${postsWithAuthors.length} posts with authors`);

    // Load posts with comments and authors
    const postsWithComments = await Post
      .where('isPublic', '=', true)
      .load(['comments', 'author'])
      .limit(3)
      .exec();
    console.log(`Loaded ${postsWithComments.length} posts with comments and authors`);

    // Load user with their posts
    const users = await User.limit(2).exec();
    if (users.length > 0) {
      const userWithPosts = await User
        .where('id', '=', users[0].id)
        .load(['posts'])
        .first();
      
      if (userWithPosts) {
        console.log(`User ${userWithPosts.username} has posts loaded`);
      }
    }

    console.log('');
  }

  async aggregationQueries(): Promise<void> {
    console.log('📈 Aggregation Query Examples');
    console.log('==============================\n');

    // Count queries
    const totalPosts = await Post.count();
    const publicPostCount = await Post.where('isPublic', '=', true).count();
    console.log(`Total posts: ${totalPosts}, Public: ${publicPostCount}`);

    // Sum and average
    const totalLikes = await Post.sum('likeCount');
    const averageLikes = await Post.avg('likeCount');
    console.log(`Total likes: ${totalLikes}, Average: ${averageLikes.toFixed(2)}`);

    // Min and max
    const oldestPost = await Post.min('createdAt');
    const newestPost = await Post.max('createdAt');
    console.log(`Oldest post: ${new Date(oldestPost).toISOString()}`);
    console.log(`Newest post: ${new Date(newestPost).toISOString()}`);

    // User-specific aggregations
    const users = await User.limit(1).exec();
    if (users.length > 0) {
      const userId = users[0].id;
      const userPostCount = await Post.whereUser(userId).count();
      const userTotalLikes = await Post.whereUser(userId).sum('likeCount');
      console.log(`User ${userId}: ${userPostCount} posts, ${userTotalLikes} total likes`);
    }

    console.log('');
  }

  async advancedFiltering(): Promise<void> {
    console.log('🔍 Advanced Filtering Examples');
    console.log('===============================\n');

    // Date filtering
    const lastWeek = Date.now() - (7 * 24 * 60 * 60 * 1000);
    const recentPosts = await Post
      .whereDate('createdAt', '>', lastWeek)
      .where('isPublic', '=', true)
      .exec();
    console.log(`Found ${recentPosts.length} posts from last week`);

    // Range filtering
    const popularPosts = await Post
      .whereBetween('likeCount', 5, 100)
      .where('isPublic', '=', true)
      .orderBy('likeCount', 'desc')
      .limit(10)
      .exec();
    console.log(`Found ${popularPosts.length} moderately popular posts`);

    // Array filtering
    const techPosts = await Post
      .whereArrayContains('tags', 'tech')
      .where('isPublic', '=', true)
      .exec();
    console.log(`Found ${techPosts.length} tech-related posts`);

    // Text search
    const searchResults = await Post
      .where('isPublic', '=', true)
      .orWhere(query => {
        query.whereLike('title', 'framework')
             .whereLike('content', 'orbitdb');
      })
      .limit(10)
      .exec();
    console.log(`Found ${searchResults.length} posts matching search terms`);

    // Null checks
    const postsWithBio = await User
      .whereNotNull('bio')
      .limit(5)
      .exec();
    console.log(`Found ${postsWithBio.length} users with bios`);

    console.log('');
  }

  async paginationExamples(): Promise<void> {
    console.log('📄 Pagination Examples');
    console.log('=======================\n');

    // Basic pagination
    const page1 = await Post
      .where('isPublic', '=', true)
      .orderBy('createdAt', 'desc')
      .page(1, 5)
      .exec();
    console.log(`Page 1: ${page1.length} posts`);

    // Pagination with metadata
    const paginatedResult = await Post
      .where('isPublic', '=', true)
      .orderBy('createdAt', 'desc')
      .paginate(1, 5);
    
    console.log(`Pagination: ${paginatedResult.currentPage}/${paginatedResult.lastPage}`);
    console.log(`Total: ${paginatedResult.total}, Per page: ${paginatedResult.perPage}`);
    console.log(`Has next: ${paginatedResult.hasNextPage}, Has prev: ${paginatedResult.hasPrevPage}`);

    // Chunked processing
    let processedCount = 0;
    await Post
      .where('isPublic', '=', true)
      .chunk(3, async (posts, page) => {
        processedCount += posts.length;
        console.log(`Processed chunk ${page}: ${posts.length} posts`);
        
        // Stop after processing 2 chunks for demo
        if (page >= 2) return false;
      });
    console.log(`Total processed in chunks: ${processedCount}`);

    console.log('');
  }

  async cacheExamples(): Promise<void> {
    console.log('⚡ Cache Examples');
    console.log('=================\n');

    // First execution (cache miss)
    console.log('First query execution (cache miss):');
    const start1 = Date.now();
    const posts1 = await Post
      .where('isPublic', '=', true)
      .orderBy('createdAt', 'desc')
      .limit(10)
      .exec();
    const duration1 = Date.now() - start1;
    console.log(`Returned ${posts1.length} posts in ${duration1}ms`);

    // Second execution (cache hit)
    console.log('Second query execution (cache hit):');
    const start2 = Date.now();
    const posts2 = await Post
      .where('isPublic', '=', true)
      .orderBy('createdAt', 'desc')
      .limit(10)
      .exec();
    const duration2 = Date.now() - start2;
    console.log(`Returned ${posts2.length} posts in ${duration2}ms`);

    // Cache statistics
    const stats = await this.framework.getFrameworkStats();
    console.log('Cache statistics:', stats.cache.stats);

    console.log('');
  }

  async optimizationExamples(): Promise<void> {
    console.log('🚀 Query Optimization Examples');
    console.log('===============================\n');

    // Query explanation
    const query = Post
      .where('isPublic', '=', true)
      .where('likeCount', '>', 10)
      .orderBy('createdAt', 'desc')
      .limit(20);

    const explanation = await this.framework.explainQuery(query);
    console.log('Query explanation:');
    console.log('- Strategy:', explanation.plan.strategy);
    console.log('- Estimated cost:', explanation.plan.estimatedCost);
    console.log('- Optimizations:', explanation.plan.optimizations);
    console.log('- Suggestions:', explanation.suggestions);

    // Query with index hint
    const optimizedQuery = Post
      .where('isPublic', '=', true)
      .useIndex('post_public_idx')
      .orderBy('createdAt', 'desc')
      .limit(10);

    const optimizedResults = await optimizedQuery.exec();
    console.log(`Optimized query returned ${optimizedResults.length} results`);

    // Disable cache for specific query
    const nonCachedQuery = Post
      .where('isPublic', '=', true)
      .limit(5);

    // Note: This would work with QueryExecutor integration
    // const nonCachedResults = await nonCachedQuery.exec().disableCache();

    console.log('');
  }

  async demonstrateQueryBuilder(): Promise<void> {
    console.log('🔧 QueryBuilder Method Demonstration');
    console.log('=====================================\n');

    // Show various QueryBuilder methods
    const complexQuery = Post
      .where('isPublic', '=', true)
      .whereNotNull('title')
      .whereDateBetween('createdAt', Date.now() - 86400000 * 7, Date.now())
      .whereArrayLength('tags', '>', 0)
      .orderByMultiple([
        { field: 'likeCount', direction: 'desc' },
        { field: 'createdAt', direction: 'desc' }
      ])
      .distinct('userId')
      .limit(15);

    console.log('Complex query SQL representation:');
    console.log(complexQuery.toSQL());

    console.log('\nQuery explanation:');
    console.log(complexQuery.explain());

    // Clone and modify query
    const modifiedQuery = complexQuery.clone()
      .where('likeCount', '>', 5)
      .limit(10);

    console.log('\nModified query SQL:');
    console.log(modifiedQuery.toSQL());

    const results = await modifiedQuery.exec();
    console.log(`\nExecuted complex query, got ${results.length} results`);

    console.log('');
  }
}

// Usage example
export async function runQueryExamples(
  orbitDBService: any, 
  ipfsService: any
): Promise<void> {
  const framework = new SocialPlatformFramework();
  
  try {
    await framework.initialize(orbitDBService, ipfsService, 'development');

    // Create some sample data if needed
    await createSampleData(framework);

    // Run query examples
    const examples = new QueryExamples(framework);
    await examples.runAllExamples();
    await examples.demonstrateQueryBuilder();

    // Show final framework stats
    const stats = await framework.getFrameworkStats();
    console.log('📊 Final Framework Statistics:');
    console.log(JSON.stringify(stats, null, 2));

  } catch (error) {
    console.error('❌ Query examples failed:', error);
  } finally {
    await framework.stop();
  }
}

async function createSampleData(framework: SocialPlatformFramework): Promise<void> {
  console.log('🗄️  Creating sample data for query examples...\n');

  try {
    // Create users
    const alice = await framework.createUser({
      username: 'alice',
      email: 'alice@example.com',
      bio: 'Tech enthusiast and framework developer'
    });

    const bob = await framework.createUser({
      username: 'bob',
      email: 'bob@example.com',
      bio: 'Building decentralized applications'
    });

    const charlie = await framework.createUser({
      username: 'charlie',
      email: 'charlie@example.com'
    });

    // Create posts
    await framework.createPost(alice.id, {
      title: 'Introduction to DebrosFramework',
      content: 'The DebrosFramework makes OrbitDB development much easier...',
      tags: ['framework', 'orbitdb', 'tech'],
      isPublic: true
    });

    await framework.createPost(alice.id, {
      title: 'Advanced Query Patterns',
      content: 'Here are some advanced patterns for querying decentralized data...',
      tags: ['queries', 'patterns', 'tech'],
      isPublic: true
    });

    await framework.createPost(bob.id, {
      title: 'Building Scalable dApps',
      content: 'Scalability is crucial for decentralized applications...',
      tags: ['scalability', 'dapps'],
      isPublic: true
    });

    await framework.createPost(bob.id, {
      title: 'Private Development Notes',
      content: 'Some private thoughts on the framework architecture...',
      tags: ['private', 'notes'],
      isPublic: false
    });

    await framework.createPost(charlie.id, {
      title: 'Getting Started Guide',
      content: 'A comprehensive guide to getting started with the framework...',
      tags: ['guide', 'beginner'],
      isPublic: true
    });

    // Create some follows
    await framework.followUser(alice.id, bob.id);
    await framework.followUser(bob.id, charlie.id);
    await framework.followUser(charlie.id, alice.id);

    console.log('✅ Sample data created successfully!\n');

  } catch (error) {
    console.warn('⚠️  Some sample data creation failed:', error);
  }
}