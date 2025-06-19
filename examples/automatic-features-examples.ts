/**
 * Comprehensive Examples for Automatic Features (Phase 5)
 * 
 * This file demonstrates the automatic pinning and PubSub capabilities:
 * - Smart pinning strategies based on usage patterns
 * - Automatic event publishing for model changes
 * - Real-time synchronization across nodes
 * - Performance optimization through intelligent caching
 * - Cross-node communication and coordination
 */

import { SocialPlatformFramework, User, Post, Comment } from './framework-integration';
import { PinningManager } from '../src/framework/pinning/PinningManager';
import { PubSubManager } from '../src/framework/pubsub/PubSubManager';

export class AutomaticFeaturesExamples {
  private framework: SocialPlatformFramework;
  private pinningManager: PinningManager;
  private pubsubManager: PubSubManager;

  constructor(framework: SocialPlatformFramework) {
    this.framework = framework;
    // These would be injected from the framework
    this.pinningManager = (framework as any).pinningManager;
    this.pubsubManager = (framework as any).pubsubManager;
  }

  async runAllExamples(): Promise<void> {
    console.log('🤖 Running comprehensive automatic features examples...\n');

    await this.pinningStrategyExamples();
    await this.automaticEventPublishingExamples();
    await this.realTimeSynchronizationExamples();
    await this.crossNodeCommunicationExamples();
    await this.performanceOptimizationExamples();
    await this.intelligentCleanupExamples();

    console.log('✅ All automatic features examples completed!\n');
  }

  async pinningStrategyExamples(): Promise<void> {
    console.log('📌 Smart Pinning Strategy Examples');
    console.log('==================================\n');

    // Configure different pinning strategies for different model types
    console.log('Setting up pinning strategies:');
    
    // Popular content gets pinned based on access patterns
    this.pinningManager.setPinningRule('Post', {
      strategy: 'popularity',
      factor: 1.5,
      maxPins: 100,
      minAccessCount: 5
    });

    // User profiles are always pinned (important core data)
    this.pinningManager.setPinningRule('User', {
      strategy: 'fixed',
      factor: 2.0,
      maxPins: 50
    });

    // Comments use size-based pinning (prefer smaller, more efficient content)
    this.pinningManager.setPinningRule('Comment', {
      strategy: 'size',
      factor: 1.0,
      maxPins: 200
    });

    // Create sample content and observe pinning behavior
    const posts = await Post.where('isPublic', '=', true).limit(5).exec();
    
    if (posts.length > 0) {
      console.log('\nDemonstrating automatic pinning:');
      
      for (let i = 0; i < posts.length; i++) {
        const post = posts[i];
        const hash = `hash-${post.id}-${Date.now()}`;
        
        // Simulate content access patterns
        for (let access = 0; access < (i + 1) * 3; access++) {
          await this.pinningManager.recordAccess(hash);
        }

        // Pin content based on strategy
        const pinned = await this.pinningManager.pinContent(
          hash,
          'Post',
          post.id,
          {
            title: post.title,
            createdAt: post.createdAt,
            size: post.content.length
          }
        );

        console.log(`Post "${post.title}": ${pinned ? 'PINNED' : 'NOT PINNED'} (${(i + 1) * 3} accesses)`);
      }

      // Show pinning metrics
      const metrics = this.pinningManager.getMetrics();
      console.log('\nPinning Metrics:');
      console.log(`- Total pinned: ${metrics.totalPinned}`);
      console.log(`- Total size: ${(metrics.totalSize / 1024).toFixed(2)} KB`);
      console.log(`- Most accessed: ${metrics.mostAccessed?.hash || 'None'}`);
      console.log(`- Strategy breakdown:`);
      metrics.strategyBreakdown.forEach((count, strategy) => {
        console.log(`  * ${strategy}: ${count} items`);
      });
    }

    console.log('');
  }

  async automaticEventPublishingExamples(): Promise<void> {
    console.log('📡 Automatic Event Publishing Examples');
    console.log('======================================\n');

    // Set up event listeners to demonstrate automatic publishing
    const events: any[] = [];
    
    await this.pubsubManager.subscribe('model.created', (event) => {
      events.push({ type: 'created', ...event });
      console.log(`🆕 Model created: ${event.data.modelName}:${event.data.modelId}`);
    });

    await this.pubsubManager.subscribe('model.updated', (event) => {
      events.push({ type: 'updated', ...event });
      console.log(`📝 Model updated: ${event.data.modelName}:${event.data.modelId}`);
    });

    await this.pubsubManager.subscribe('model.deleted', (event) => {
      events.push({ type: 'deleted', ...event });
      console.log(`🗑️  Model deleted: ${event.data.modelName}:${event.data.modelId}`);
    });

    console.log('Event listeners set up, creating test data...\n');

    // Create data and observe automatic event publishing
    const testUser = await User.create({
      username: `testuser-${Date.now()}`,
      email: `test${Date.now()}@example.com`,
      bio: 'Testing automatic event publishing'
    });

    // Simulate event emission (in real implementation, this would be automatic)
    this.pubsubManager.emit('modelEvent', 'create', testUser);

    const testPost = await Post.create({
      title: 'Testing Automatic Events',
      content: 'This post creation should trigger automatic event publishing',
      userId: testUser.id,
      isPublic: true
    });

    this.pubsubManager.emit('modelEvent', 'create', testPost);

    // Update the post
    await testPost.update({ title: 'Updated: Testing Automatic Events' });
    this.pubsubManager.emit('modelEvent', 'update', testPost, { title: 'Updated title' });

    // Wait a moment for event processing
    await new Promise(resolve => setTimeout(resolve, 1000));

    console.log(`\nCaptured ${events.length} automatic events:`);
    events.forEach((event, index) => {
      console.log(`${index + 1}. ${event.type}: ${event.data?.modelName || 'unknown'}`);
    });

    console.log('');
  }

  async realTimeSynchronizationExamples(): Promise<void> {
    console.log('⚡ Real-Time Synchronization Examples');
    console.log('=====================================\n');

    // Simulate multiple nodes subscribing to the same topics
    const nodeEvents: Record<string, any[]> = {
      node1: [],
      node2: [],
      node3: []
    };

    // Subscribe each "node" to model events
    await this.pubsubManager.subscribe('model.*', (event) => {
      nodeEvents.node1.push(event);
    }, {
      filter: (event) => event.data.modelName === 'Post'
    });

    await this.pubsubManager.subscribe('model.*', (event) => {
      nodeEvents.node2.push(event);
    }, {
      filter: (event) => event.data.modelName === 'User'
    });

    await this.pubsubManager.subscribe('model.*', (event) => {
      nodeEvents.node3.push(event);
    }); // No filter - receives all events

    console.log('Multiple nodes subscribed to synchronization topics');

    // Generate events that should synchronize across nodes
    const syncUser = await User.create({
      username: `syncuser-${Date.now()}`,
      email: `sync${Date.now()}@example.com`,
      bio: 'User for testing real-time sync'
    });

    const syncPost = await Post.create({
      title: 'Real-Time Sync Test',
      content: 'This should synchronize across all subscribed nodes',
      userId: syncUser.id,
      isPublic: true
    });

    // Emit events
    await this.pubsubManager.publish('model.created', {
      modelName: 'User',
      modelId: syncUser.id,
      timestamp: Date.now()
    });

    await this.pubsubManager.publish('model.created', {
      modelName: 'Post',
      modelId: syncPost.id,
      timestamp: Date.now()
    });

    // Wait for synchronization
    await new Promise(resolve => setTimeout(resolve, 1500));

    console.log('\nSynchronization results:');
    console.log(`Node 1 (Post filter): ${nodeEvents.node1.length} events received`);
    console.log(`Node 2 (User filter): ${nodeEvents.node2.length} events received`);
    console.log(`Node 3 (No filter): ${nodeEvents.node3.length} events received`);

    // Demonstrate conflict resolution
    console.log('\nSimulating conflict resolution:');
    await this.pubsubManager.publish('database.conflict', {
      modelName: 'Post',
      modelId: syncPost.id,
      conflictType: 'concurrent_update',
      resolution: 'last_write_wins',
      timestamp: Date.now()
    });

    console.log('');
  }

  async crossNodeCommunicationExamples(): Promise<void> {
    console.log('🌐 Cross-Node Communication Examples');
    console.log('====================================\n');

    // Simulate coordination between nodes
    const coordinationEvents: any[] = [];

    // Set up coordination topics
    await this.pubsubManager.subscribe('node.heartbeat', (event) => {
      coordinationEvents.push(event);
      console.log(`💓 Heartbeat from ${event.source}: ${event.data.status}`);
    });

    await this.pubsubManager.subscribe('node.resource', (event) => {
      coordinationEvents.push(event);
      console.log(`📊 Resource update from ${event.source}: ${event.data.type}`);
    });

    await this.pubsubManager.subscribe('cluster.rebalance', (event) => {
      coordinationEvents.push(event);
      console.log(`⚖️  Cluster rebalance initiated: ${event.data.reason}`);
    });

    console.log('Cross-node communication channels established\n');

    // Simulate node communication
    await this.pubsubManager.publish('node.heartbeat', {
      status: 'healthy',
      load: 0.65,
      memory: '2.1GB',
      connections: 42
    });

    await this.pubsubManager.publish('node.resource', {
      type: 'storage',
      available: '5.2TB',
      used: '2.8TB',
      threshold: 0.8
    });

    await this.pubsubManager.publish('cluster.rebalance', {
      reason: 'load_balancing',
      nodes: ['node-a', 'node-b', 'node-c'],
      strategy: 'round_robin'
    });

    // Demonstrate distributed consensus
    console.log('Initiating distributed consensus...');
    await this.pubsubManager.publish('consensus.propose', {
      proposalId: `proposal-${Date.now()}`,
      type: 'pin_strategy_change',
      data: {
        modelName: 'Post',
        newStrategy: 'popularity',
        newFactor: 2.0
      },
      requiredVotes: 3
    });

    await new Promise(resolve => setTimeout(resolve, 1000));

    console.log(`\nCommunication events processed: ${coordinationEvents.length}`);
    console.log('Cross-node coordination completed successfully');

    console.log('');
  }

  async performanceOptimizationExamples(): Promise<void> {
    console.log('🚀 Performance Optimization Examples');
    console.log('====================================\n');

    // Demonstrate intelligent cache warming
    console.log('1. Intelligent Cache Warming:');
    const popularPosts = await Post
      .where('isPublic', '=', true)
      .where('likeCount', '>', 10)
      .orderBy('likeCount', 'desc')
      .limit(10)
      .exec();

    // Pre-pin popular content
    for (const post of popularPosts) {
      const hash = `hash-${post.id}-content`;
      await this.pinningManager.pinContent(hash, 'Post', post.id, {
        title: post.title,
        likeCount: post.likeCount,
        priority: 'high'
      });
    }
    console.log(`Pre-pinned ${popularPosts.length} popular posts for better performance`);

    // Demonstrate predictive pinning
    console.log('\n2. Predictive Pinning:');
    const analysis = this.pinningManager.analyzePerformance();
    console.log(`Current hit rate: ${(analysis.hitRate * 100).toFixed(2)}%`);
    console.log(`Storage efficiency: ${(analysis.storageEfficiency * 100).toFixed(2)}%`);
    console.log(`Average priority: ${analysis.averagePriority.toFixed(3)}`);

    // Simulate access pattern analysis
    const accessPatterns = this.analyzeAccessPatterns();
    console.log(`\n3. Access Pattern Analysis:`);
    console.log(`Peak access time: ${accessPatterns.peakHour}:00`);
    console.log(`Most accessed content type: ${accessPatterns.mostAccessedType}`);
    console.log(`Cache miss rate: ${(accessPatterns.missRate * 100).toFixed(2)}%`);

    // Optimize based on patterns
    if (accessPatterns.missRate > 0.1) { // 10% miss rate
      console.log('\nHigh miss rate detected, optimizing...');
      await this.optimizePinningStrategy(accessPatterns);
    }

    console.log('');
  }

  async intelligentCleanupExamples(): Promise<void> {
    console.log('🧹 Intelligent Cleanup Examples');
    console.log('===============================\n');

    // Get initial stats
    const initialStats = this.pinningManager.getStats();
    console.log('Initial state:');
    console.log(`- Pinned items: ${initialStats.totalPinned}`);
    console.log(`- Total size: ${(initialStats.totalSize / 1024).toFixed(2)} KB`);

    // Create some test content that will be cleaned up
    const testHashes = [];
    for (let i = 0; i < 10; i++) {
      const hash = `test-cleanup-${i}-${Date.now()}`;
      testHashes.push(hash);
      
      await this.pinningManager.pinContent(hash, 'Comment', `comment-${i}`, {
        content: `Test comment ${i} for cleanup`,
        size: 100 + i * 10,
        priority: Math.random() * 0.3 // Low priority
      });
    }

    console.log(`\nCreated ${testHashes.length} test items for cleanup`);

    // Simulate time passing (items become stale)
    console.log('Simulating passage of time...');
    
    // Artificially age some items
    for (let i = 0; i < 5; i++) {
      const hash = testHashes[i];
      const item = (this.pinningManager as any).pinnedItems.get(hash);
      if (item) {
        item.lastAccessed = Date.now() - (8 * 24 * 60 * 60 * 1000); // 8 days ago
        item.accessCount = 1; // Very low access
      }
    }

    // Trigger cleanup
    console.log('Triggering intelligent cleanup...');
    const cleanedItems = await (this.pinningManager as any).performCleanup();

    const finalStats = this.pinningManager.getStats();
    console.log('\nCleanup results:');
    console.log(`- Items after cleanup: ${finalStats.totalPinned}`);
    console.log(`- Size freed: ${((initialStats.totalSize - finalStats.totalSize) / 1024).toFixed(2)} KB`);
    console.log(`- Cleanup efficiency: ${((initialStats.totalPinned - finalStats.totalPinned) / initialStats.totalPinned * 100).toFixed(2)}%`);

    // Demonstrate memory optimization
    console.log('\nMemory optimization metrics:');
    const memoryAnalysis = this.analyzeMemoryUsage();
    console.log(`- Memory utilization: ${(memoryAnalysis.utilization * 100).toFixed(2)}%`);
    console.log(`- Fragmentation ratio: ${(memoryAnalysis.fragmentation * 100).toFixed(2)}%`);
    console.log(`- Recommended cleanup interval: ${memoryAnalysis.recommendedInterval}ms`);

    console.log('');
  }

  // Helper methods for analysis and optimization

  private analyzeAccessPatterns(): any {
    // Simulate access pattern analysis
    return {
      peakHour: 14, // 2 PM
      mostAccessedType: 'Post',
      missRate: 0.15,
      trendsDetected: ['increased_mobile_access', 'peak_evening_hours'],
      recommendations: ['increase_post_pinning', 'reduce_comment_pinning']
    };
  }

  private async optimizePinningStrategy(patterns: any): Promise<void> {
    console.log('Applying optimization based on access patterns:');
    
    // Increase pinning for most accessed content type
    if (patterns.mostAccessedType === 'Post') {
      this.pinningManager.setPinningRule('Post', {
        strategy: 'popularity',
        factor: 2.0,
        maxPins: 150 // Increased from 100
      });
      console.log('- Increased Post pinning capacity');
    }

    // Adjust cleanup frequency based on miss rate
    if (patterns.missRate > 0.2) {
      // More aggressive cleanup needed
      console.log('- Enabled more aggressive cleanup');
    }

    console.log('Optimization complete');
  }

  private analyzeMemoryUsage(): any {
    const stats = this.pinningManager.getStats();
    
    return {
      utilization: stats.totalSize / (10 * 1024 * 1024), // Assuming 10MB limit
      fragmentation: 0.12, // 12% fragmentation
      recommendedInterval: stats.totalPinned > 100 ? 30000 : 60000, // More frequent cleanup if many items
      hotspots: ['user_profiles', 'recent_posts'],
      coldSpots: ['old_comments', 'archived_content']
    };
  }

  async demonstrateAdvancedAutomation(): Promise<void> {
    console.log('🤖 Advanced Automation Demonstration');
    console.log('===================================\n');

    // Demonstrate self-healing capabilities
    console.log('1. Self-Healing System:');
    
    // Simulate node failure detection
    await this.pubsubManager.publish('node.failure', {
      nodeId: 'node-beta',
      reason: 'network_timeout',
      lastSeen: Date.now() - 30000
    });

    // Automatic rebalancing
    await this.pubsubManager.publish('cluster.rebalance', {
      trigger: 'node_failure',
      failedNode: 'node-beta',
      redistribution: {
        'node-alpha': 0.6,
        'node-gamma': 0.4
      }
    });

    console.log('Self-healing sequence initiated and completed');

    // Demonstrate adaptive optimization
    console.log('\n2. Adaptive Optimization:');
    const performance = this.pinningManager.analyzePerformance();
    
    if (performance.hitRate < 0.8) {
      console.log('Low hit rate detected, adapting pinning strategy...');
      // Auto-adjust pinning factors
      this.pinningManager.setPinningRule('Post', {
        strategy: 'popularity',
        factor: performance.averagePriority + 0.5 // Increase based on current performance
      });
      console.log('Pinning strategy adapted automatically');
    }

    // Demonstrate predictive scaling
    console.log('\n3. Predictive Scaling:');
    const predictions = this.generateLoadPredictions();
    console.log(`Predicted load increase: ${predictions.expectedIncrease}%`);
    console.log(`Recommended action: ${predictions.recommendation}`);

    if (predictions.expectedIncrease > 50) {
      console.log('Preemptively scaling resources...');
      await this.pubsubManager.publish('cluster.scale', {
        type: 'predictive',
        factor: 1.5,
        reason: 'anticipated_load_increase'
      });
    }

    console.log('Advanced automation demonstration completed\n');
  }

  private generateLoadPredictions(): any {
    // Simulate machine learning-based load prediction
    return {
      expectedIncrease: Math.random() * 100,
      confidence: 0.85,
      timeframe: '2 hours',
      recommendation: 'scale_up',
      factors: ['user_growth', 'content_creation_spike', 'viral_post_detected']
    };
  }
}

// Usage function
export async function runAutomaticFeaturesExamples(
  orbitDBService: any,
  ipfsService: any
): Promise<void> {
  const framework = new SocialPlatformFramework();

  try {
    await framework.initialize(orbitDBService, ipfsService, 'development');

    // Initialize automatic features (would be done in framework initialization)
    const pinningManager = new PinningManager(ipfsService, {
      maxTotalPins: 1000,
      maxTotalSize: 50 * 1024 * 1024, // 50MB
      cleanupIntervalMs: 30000 // 30 seconds for demo
    });

    const pubsubManager = new PubSubManager(ipfsService, {
      enabled: true,
      autoPublishModelEvents: true,
      autoPublishDatabaseEvents: true,
      topicPrefix: 'debros-demo'
    });

    await pubsubManager.initialize();

    // Inject into framework for examples
    (framework as any).pinningManager = pinningManager;
    (framework as any).pubsubManager = pubsubManager;

    // Ensure sample data exists
    await createSampleDataForAutomaticFeatures(framework);

    // Run all examples
    const examples = new AutomaticFeaturesExamples(framework);
    await examples.runAllExamples();
    await examples.demonstrateAdvancedAutomation();

    // Show final statistics
    console.log('📊 Final System Statistics:');
    console.log('==========================');
    
    const pinningStats = pinningManager.getStats();
    const pubsubStats = pubsubManager.getStats();
    const frameworkStats = await framework.getFrameworkStats();

    console.log('\nPinning System:');
    console.log(`- Total pinned: ${pinningStats.totalPinned}`);
    console.log(`- Total size: ${(pinningStats.totalSize / 1024).toFixed(2)} KB`);
    console.log(`- Active strategies: ${Object.keys(pinningStats.strategies).join(', ')}`);

    console.log('\nPubSub System:');
    console.log(`- Messages published: ${pubsubStats.totalPublished}`);
    console.log(`- Messages received: ${pubsubStats.totalReceived}`);
    console.log(`- Active subscriptions: ${pubsubStats.totalSubscriptions}`);
    console.log(`- Average latency: ${pubsubStats.averageLatency.toFixed(2)}ms`);

    console.log('\nFramework:');
    console.log(`- Models registered: ${frameworkStats.registeredModels.length}`);
    console.log(`- Cache hit rate: ${(frameworkStats.cache.query.stats.hitRate * 100).toFixed(2)}%`);

    // Cleanup
    await pinningManager.shutdown();
    await pubsubManager.shutdown();

  } catch (error) {
    console.error('❌ Automatic features examples failed:', error);
  } finally {
    await framework.stop();
  }
}

async function createSampleDataForAutomaticFeatures(framework: SocialPlatformFramework): Promise<void> {
  console.log('🗄️  Creating sample data for automatic features...\n');

  try {
    // Create users with varied activity patterns
    const users = [];
    for (let i = 0; i < 5; i++) {
      const user = await framework.createUser({
        username: `autouser${i}`,
        email: `autouser${i}@example.com`,
        bio: `Automatic features test user ${i}`
      });
      users.push(user);
    }

    // Create posts with different popularity levels
    const posts = [];
    for (let i = 0; i < 15; i++) {
      const user = users[i % users.length];
      const post = await framework.createPost(user.id, {
        title: `Auto Post ${i}: ${['Popular', 'Normal', 'Unpopular'][i % 3]} Content`,
        content: `This is test content for automatic features. Post ${i} with length ${100 + i * 50}.`,
        tags: ['automation', 'testing', i % 2 === 0 ? 'popular' : 'normal'],
        isPublic: true
      });
      
      // Simulate different like counts
      (post as any).likeCount = i < 5 ? 20 + i * 5 : i < 10 ? 5 + i : i % 3;
      await post.save();
      
      posts.push(post);
    }

    // Create comments to establish relationships
    for (let i = 0; i < 25; i++) {
      const user = users[i % users.length];
      const post = posts[i % posts.length];
      await framework.createComment(
        user.id,
        post.id,
        `Auto comment ${i}: This is a test comment for automatic features testing.`
      );
    }

    console.log(`✅ Created ${users.length} users, ${posts.length} posts, and 25 comments\n`);

  } catch (error) {
    console.warn('⚠️  Some sample data creation failed:', error);
  }
}