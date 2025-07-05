/**
 * Comprehensive Relationship Examples for DebrosFramework
 * 
 * This file demonstrates all the relationship loading capabilities implemented in Phase 4:
 * - Lazy and eager loading
 * - Relationship caching
 * - Cross-database relationship resolution
 * - Advanced loading with constraints
 * - Performance optimization techniques
 */

import { SocialPlatformFramework, User, Post, Comment, Follow } from './framework-integration';

export class RelationshipExamples {
  private framework: SocialPlatformFramework;

  constructor(framework: SocialPlatformFramework) {
    this.framework = framework;
  }

  async runAllExamples(): Promise<void> {
    console.log('🔗 Running comprehensive relationship examples...\n');

    await this.basicRelationshipLoading();
    await this.eagerLoadingExamples();
    await this.lazyLoadingExamples();
    await this.constrainedLoadingExamples();
    await this.cacheOptimizationExamples();
    await this.crossDatabaseRelationships();
    await this.performanceExamples();

    console.log('✅ All relationship examples completed!\n');
  }

  async basicRelationshipLoading(): Promise<void> {
    console.log('🔗 Basic Relationship Loading');
    console.log('==============================\n');

    // Get a post and load its author (BelongsTo)
    const posts = await Post.where('isPublic', '=', true).limit(3).exec();
    
    if (posts.length > 0) {
      const post = posts[0];
      console.log(`Loading author for post: ${post.title}`);
      
      const author = await post.loadRelation('author');
      console.log(`Author loaded: ${author?.username || 'Unknown'}`);

      // Load comments for the post (HasMany)
      console.log(`Loading comments for post: ${post.title}`);
      const comments = await post.loadRelation('comments');
      console.log(`Comments loaded: ${Array.isArray(comments) ? comments.length : 0} comment(s)`);

      // Check what relationships are loaded
      console.log(`Loaded relationships: ${post.getLoadedRelations().join(', ')}`);
    }

    // Get a user and load their posts (HasMany)
    const users = await User.limit(2).exec();
    if (users.length > 0) {
      const user = users[0];
      console.log(`\nLoading posts for user: ${user.username}`);
      
      const userPosts = await user.loadRelation('posts');
      console.log(`Posts loaded: ${Array.isArray(userPosts) ? userPosts.length : 0} post(s)`);
    }

    console.log('');
  }

  async eagerLoadingExamples(): Promise<void> {
    console.log('⚡ Eager Loading Examples');
    console.log('==========================\n');

    // Load multiple posts with their authors and comments in one go
    console.log('Loading posts with authors and comments (eager loading):');
    const posts = await Post
      .where('isPublic', '=', true)
      .limit(5)
      .exec();

    if (posts.length > 0) {
      // Eager load relationships for all posts at once
      const startTime = Date.now();
      await posts[0].load(['author', 'comments']);
      const singleLoadTime = Date.now() - startTime;

      // Now eager load for all posts
      const eagerStartTime = Date.now();
      await this.framework.relationshipManager.eagerLoadRelationships(
        posts, 
        ['author', 'comments']
      );
      const eagerLoadTime = Date.now() - eagerStartTime;

      console.log(`Single post relationship loading: ${singleLoadTime}ms`);
      console.log(`Eager loading for ${posts.length} posts: ${eagerLoadTime}ms`);
      console.log(`Efficiency gain: ${((singleLoadTime * posts.length) / eagerLoadTime).toFixed(2)}x faster`);

      // Verify relationships are loaded
      let loadedCount = 0;
      for (const post of posts) {
        if (post.isRelationLoaded('author') && post.isRelationLoaded('comments')) {
          loadedCount++;
        }
      }
      console.log(`Successfully loaded relationships for ${loadedCount}/${posts.length} posts`);
    }

    // Load users with their posts
    console.log('\nLoading users with their posts (eager loading):');
    const users = await User.limit(3).exec();
    
    if (users.length > 0) {
      await this.framework.relationshipManager.eagerLoadRelationships(
        users, 
        ['posts', 'following']
      );

      for (const user of users) {
        const posts = user.getRelation('posts') || [];
        const following = user.getRelation('following') || [];
        console.log(`User ${user.username}: ${posts.length} posts, ${following.length} following`);
      }
    }

    console.log('');
  }

  async lazyLoadingExamples(): Promise<void> {
    console.log('💤 Lazy Loading Examples');
    console.log('=========================\n');

    const posts = await Post.where('isPublic', '=', true).limit(2).exec();
    
    if (posts.length > 0) {
      const post = posts[0];
      
      console.log('Demonstrating lazy loading behavior:');
      console.log(`Post title: ${post.title}`);
      console.log(`Author loaded initially: ${post.isRelationLoaded('author')}`);
      
      // First access triggers loading
      console.log('Accessing author (triggers lazy loading)...');
      const author = await post.loadRelation('author');
      console.log(`Author: ${author?.username || 'Unknown'}`);
      console.log(`Author loaded after access: ${post.isRelationLoaded('author')}`);
      
      // Second access uses cached value
      console.log('Accessing author again (uses cache)...');
      const authorAgain = post.getRelation('author');
      console.log(`Author (cached): ${authorAgain?.username || 'Unknown'}`);
      
      // Reload relationship (clears cache and reloads)
      console.log('Reloading author relationship...');
      const reloadedAuthor = await post.reloadRelation('author');
      console.log(`Reloaded author: ${reloadedAuthor?.username || 'Unknown'}`);
    }

    console.log('');
  }

  async constrainedLoadingExamples(): Promise<void> {
    console.log('🎯 Constrained Loading Examples');
    console.log('=================================\n');

    const posts = await Post.where('isPublic', '=', true).limit(3).exec();
    
    if (posts.length > 0) {
      const post = posts[0];
      
      // Load only recent comments
      console.log(`Loading recent comments for post: ${post.title}`);
      const recentComments = await post.loadRelationWithConstraints('comments', (query) =>
        query.where('createdAt', '>', Date.now() - 86400000) // Last 24 hours
             .orderBy('createdAt', 'desc')
             .limit(5)
      );
      console.log(`Recent comments loaded: ${Array.isArray(recentComments) ? recentComments.length : 0}`);

      // Load comments with minimum length
      console.log(`Loading substantive comments (>50 chars):`);
      const substantiveComments = await post.loadRelationWithConstraints('comments', (query) =>
        query.whereRaw('LENGTH(content) > ?', [50])
             .orderBy('createdAt', 'desc')
             .limit(3)
      );
      console.log(`Substantive comments: ${Array.isArray(substantiveComments) ? substantiveComments.length : 0}`);
    }

    // Load user posts with constraints
    const users = await User.limit(2).exec();
    if (users.length > 0) {
      const user = users[0];
      
      console.log(`\nLoading popular posts for user: ${user.username}`);
      const popularPosts = await user.loadRelationWithConstraints('posts', (query) =>
        query.where('likeCount', '>', 5)
             .where('isPublic', '=', true)
             .orderBy('likeCount', 'desc')
             .limit(10)
      );
      console.log(`Popular posts: ${Array.isArray(popularPosts) ? popularPosts.length : 0}`);
    }

    console.log('');
  }

  async cacheOptimizationExamples(): Promise<void> {
    console.log('🚀 Cache Optimization Examples');
    console.log('===============================\n');

    // Get cache stats before
    const statsBefore = this.framework.relationshipManager.getRelationshipCacheStats();
    console.log('Relationship cache stats before:');
    console.log(`- Total entries: ${statsBefore.cache.totalEntries}`);
    console.log(`- Hit rate: ${(statsBefore.cache.hitRate * 100).toFixed(2)}%`);

    // Load relationships multiple times to demonstrate caching
    const posts = await Post.where('isPublic', '=', true).limit(3).exec();
    
    if (posts.length > 0) {
      console.log('\nLoading relationships multiple times (should hit cache):');
      
      for (let i = 0; i < 3; i++) {
        const startTime = Date.now();
        await posts[0].loadRelation('author');
        await posts[0].loadRelation('comments');
        const duration = Date.now() - startTime;
        console.log(`Iteration ${i + 1}: ${duration}ms`);
      }
    }

    // Warm up cache
    console.log('\nWarming up relationship cache:');
    const allPosts = await Post.limit(5).exec();
    const allUsers = await User.limit(3).exec();
    
    await this.framework.relationshipManager.warmupRelationshipCache(
      allPosts, 
      ['author', 'comments']
    );
    
    await this.framework.relationshipManager.warmupRelationshipCache(
      allUsers, 
      ['posts', 'following']
    );

    // Get cache stats after
    const statsAfter = this.framework.relationshipManager.getRelationshipCacheStats();
    console.log('\nRelationship cache stats after warmup:');
    console.log(`- Total entries: ${statsAfter.cache.totalEntries}`);
    console.log(`- Hit rate: ${(statsAfter.cache.hitRate * 100).toFixed(2)}%`);
    console.log(`- Memory usage: ${(statsAfter.cache.memoryUsage / 1024).toFixed(2)} KB`);

    // Show cache performance analysis
    const performance = statsAfter.performance;
    console.log('\nCache performance analysis:');
    console.log(`- Average age: ${(performance.averageAge / 1000).toFixed(2)} seconds`);
    console.log(`- Relationship types in cache:`);
    performance.relationshipTypes.forEach((count, type) => {
      console.log(`  * ${type}: ${count} entries`);
    });

    console.log('');
  }

  async crossDatabaseRelationships(): Promise<void> {
    console.log('🌐 Cross-Database Relationship Examples');
    console.log('=========================================\n');

    // This demonstrates relationships that span across user databases and global databases
    
    // Get users (stored in global database)
    const users = await User.limit(2).exec();
    
    if (users.length >= 2) {
      const user1 = users[0];
      const user2 = users[1];
      
      console.log(`Loading cross-database relationships:`);
      console.log(`User 1: ${user1.username} (global DB)`);
      console.log(`User 2: ${user2.username} (global DB)`);
      
      // Load posts for user1 (stored in user1's database)
      const user1Posts = await user1.loadRelation('posts');
      console.log(`User 1 posts (from user DB): ${Array.isArray(user1Posts) ? user1Posts.length : 0}`);
      
      // Load posts for user2 (stored in user2's database)
      const user2Posts = await user2.loadRelation('posts');
      console.log(`User 2 posts (from user DB): ${Array.isArray(user2Posts) ? user2Posts.length : 0}`);
      
      // Load followers relationship (stored in global database)
      const user1Following = await user1.loadRelation('following');
      console.log(`User 1 following (from global DB): ${Array.isArray(user1Following) ? user1Following.length : 0}`);
      
      // Demonstrate the complexity: Post (user DB) -> Author (global DB) -> Posts (back to user DB)
      if (Array.isArray(user1Posts) && user1Posts.length > 0) {
        const post = user1Posts[0];
        console.log(`\nDemonstrating complex cross-DB relationship chain:`);
        console.log(`Post: "${post.title}" (from user DB)`);
        
        const author = await post.loadRelation('author');
        console.log(`-> Author: ${author?.username || 'Unknown'} (from global DB)`);
        
        if (author) {
          const authorPosts = await author.loadRelation('posts');
          console.log(`-> Author's posts: ${Array.isArray(authorPosts) ? authorPosts.length : 0} (back to user DB)`);
        }
      }
    }

    console.log('');
  }

  async performanceExamples(): Promise<void> {
    console.log('📈 Performance Examples');
    console.log('========================\n');

    // Compare different loading strategies
    const posts = await Post.where('isPublic', '=', true).limit(10).exec();
    
    if (posts.length > 0) {
      console.log(`Performance comparison for ${posts.length} posts:\n`);
      
      // Strategy 1: Sequential loading (N+1 problem)
      console.log('1. Sequential loading (N+1 queries):');
      const sequentialStart = Date.now();
      for (const post of posts) {
        await post.loadRelation('author');
      }
      const sequentialTime = Date.now() - sequentialStart;
      console.log(`   Time: ${sequentialTime}ms (${(sequentialTime / posts.length).toFixed(2)}ms per post)`);
      
      // Clear loaded relationships for fair comparison
      posts.forEach(post => {
        post._loadedRelations.clear();
      });
      
      // Strategy 2: Eager loading (optimal)
      console.log('\n2. Eager loading (optimized):');
      const eagerStart = Date.now();
      await this.framework.relationshipManager.eagerLoadRelationships(posts, ['author']);
      const eagerTime = Date.now() - eagerStart;
      console.log(`   Time: ${eagerTime}ms (${(eagerTime / posts.length).toFixed(2)}ms per post)`);
      console.log(`   Performance improvement: ${(sequentialTime / eagerTime).toFixed(2)}x faster`);
      
      // Strategy 3: Cached loading (fastest for repeated access)
      console.log('\n3. Cached loading (repeated access):');
      const cachedStart = Date.now();
      await this.framework.relationshipManager.eagerLoadRelationships(posts, ['author']);
      const cachedTime = Date.now() - cachedStart;
      console.log(`   Time: ${cachedTime}ms (cache hit)`);
      console.log(`   Cache efficiency: ${(eagerTime / Math.max(cachedTime, 1)).toFixed(2)}x faster than first load`);
    }

    // Memory usage demonstration
    console.log('\nMemory usage analysis:');
    const memoryStats = this.framework.relationshipManager.getRelationshipCacheStats();
    console.log(`- Cache entries: ${memoryStats.cache.totalEntries}`);
    console.log(`- Memory usage: ${(memoryStats.cache.memoryUsage / 1024).toFixed(2)} KB`);
    console.log(`- Average per entry: ${memoryStats.cache.totalEntries > 0 ? (memoryStats.cache.memoryUsage / memoryStats.cache.totalEntries).toFixed(2) : 0} bytes`);

    // Cache cleanup demonstration
    console.log('\nCache cleanup:');
    const expiredCount = this.framework.relationshipManager.cleanupExpiredCache();
    console.log(`- Cleaned up ${expiredCount} expired entries`);
    
    // Model-based invalidation
    const invalidatedCount = this.framework.relationshipManager.invalidateModelCache('User');
    console.log(`- Invalidated ${invalidatedCount} User-related cache entries`);

    console.log('');
  }

  async demonstrateAdvancedFeatures(): Promise<void> {
    console.log('🔬 Advanced Relationship Features');
    console.log('==================================\n');

    const posts = await Post.where('isPublic', '=', true).limit(3).exec();
    
    if (posts.length > 0) {
      const post = posts[0];
      
      // Demonstrate conditional loading
      console.log('Conditional relationship loading:');
      if (!post.isRelationLoaded('author')) {
        console.log('- Author not loaded, loading now...');
        await post.loadRelation('author');
      } else {
        console.log('- Author already loaded, using cached version');
      }
      
      // Demonstrate partial loading with pagination
      console.log('\nPaginated relationship loading:');
      const page1Comments = await post.loadRelationWithConstraints('comments', (query) =>
        query.orderBy('createdAt', 'desc').limit(5).offset(0)
      );
      console.log(`- Page 1: ${Array.isArray(page1Comments) ? page1Comments.length : 0} comments`);
      
      const page2Comments = await post.loadRelationWithConstraints('comments', (query) =>
        query.orderBy('createdAt', 'desc').limit(5).offset(5)
      );
      console.log(`- Page 2: ${Array.isArray(page2Comments) ? page2Comments.length : 0} comments`);
      
      // Demonstrate relationship statistics
      console.log('\nRelationship loading statistics:');
      const modelClass = post.constructor as any;
      const relationships = Array.from(modelClass.relationships?.keys() || []);
      console.log(`- Available relationships: ${relationships.join(', ')}`);
      console.log(`- Currently loaded: ${post.getLoadedRelations().join(', ')}`);
    }

    console.log('');
  }
}

// Usage example
export async function runRelationshipExamples(
  orbitDBService: any, 
  ipfsService: any
): Promise<void> {
  const framework = new SocialPlatformFramework();
  
  try {
    await framework.initialize(orbitDBService, ipfsService, 'development');

    // Ensure we have sample data
    await createSampleDataForRelationships(framework);

    // Run relationship examples
    const examples = new RelationshipExamples(framework);
    await examples.runAllExamples();
    await examples.demonstrateAdvancedFeatures();

    // Show final relationship cache statistics
    const finalStats = framework.relationshipManager.getRelationshipCacheStats();
    console.log('📊 Final Relationship Cache Statistics:');
    console.log(JSON.stringify(finalStats, null, 2));

  } catch (error) {
    console.error('❌ Relationship examples failed:', error);
  } finally {
    await framework.stop();
  }
}

async function createSampleDataForRelationships(framework: SocialPlatformFramework): Promise<void> {
  console.log('🗄️  Creating sample data for relationship examples...\n');

  try {
    // Create users
    const alice = await framework.createUser({
      username: 'alice',
      email: 'alice@example.com',
      bio: 'Framework developer and relationship expert'
    });

    const bob = await framework.createUser({
      username: 'bob',
      email: 'bob@example.com',
      bio: 'Database architect'
    });

    const charlie = await framework.createUser({
      username: 'charlie',
      email: 'charlie@example.com',
      bio: 'Performance optimization specialist'
    });

    // Create posts with relationships
    const post1 = await framework.createPost(alice.id, {
      title: 'Understanding Relationships in Distributed Databases',
      content: 'Relationships across distributed databases present unique challenges...',
      tags: ['relationships', 'distributed', 'databases'],
      isPublic: true
    });

    const post2 = await framework.createPost(bob.id, {
      title: 'Optimizing Cross-Database Queries',
      content: 'When data spans multiple databases, query optimization becomes crucial...',
      tags: ['optimization', 'queries', 'performance'],
      isPublic: true
    });

    const post3 = await framework.createPost(alice.id, {
      title: 'Caching Strategies for Relationships',
      content: 'Effective caching can dramatically improve relationship loading performance...',
      tags: ['caching', 'performance', 'relationships'],
      isPublic: true
    });

    // Create comments to establish relationships
    await framework.createComment(bob.id, post1.id, 'Great explanation of the distributed relationship challenges!');
    await framework.createComment(charlie.id, post1.id, 'This helped me understand the complexity involved.');
    await framework.createComment(alice.id, post2.id, 'Excellent optimization techniques, Bob!');
    await framework.createComment(charlie.id, post2.id, 'These optimizations improved our app performance by 3x.');
    await framework.createComment(bob.id, post3.id, 'Caching relationships was a game-changer for our system.');

    // Create follow relationships
    await framework.followUser(alice.id, bob.id);
    await framework.followUser(bob.id, charlie.id);
    await framework.followUser(charlie.id, alice.id);
    await framework.followUser(alice.id, charlie.id);

    console.log('✅ Sample relationship data created successfully!\n');

  } catch (error) {
    console.warn('⚠️  Some sample data creation failed:', error);
  }
}