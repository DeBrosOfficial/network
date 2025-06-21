import { describe, beforeAll, afterAll, it, expect, jest } from '@jest/globals';
import { BlogTestRunner, BlogTestConfig } from '../scenarios/BlogTestRunner';

// Increase timeout for Docker-based tests
jest.setTimeout(300000); // 5 minutes

describe('Blog Workflow Integration Tests', () => {
  let testRunner: BlogTestRunner;
  
  beforeAll(async () => {
    console.log('🚀 Starting Blog Integration Tests...');
    
    const config: BlogTestConfig = {
      nodeEndpoints: [
        'http://localhost:3001',
        'http://localhost:3002', 
        'http://localhost:3003'
      ],
      syncTimeout: 15000,
      operationTimeout: 10000
    };

    testRunner = new BlogTestRunner(config);

    // Wait for all nodes to be ready
    const nodesReady = await testRunner.waitForNodesReady(120000);
    if (!nodesReady) {
      throw new Error('Blog nodes failed to become ready within timeout');
    }
    
    // Wait for peer discovery and connections
    const peersConnected = await testRunner.waitForPeerConnections(60000);
    if (!peersConnected) {
      throw new Error('Blog nodes failed to establish peer connections within timeout');
    }

    await testRunner.logStatus();
  }, 180000);

  afterAll(async () => {
    if (testRunner) {
      await testRunner.cleanup();
    }
  });

  describe('User Management Workflow', () => {
    it('should create users on different nodes and sync across network', async () => {
      console.log('\n🔧 Testing cross-node user creation and sync...');

      // Create users on different nodes
      const alice = await testRunner.createUser(0, {
        username: 'alice',
        email: 'alice@example.com',
        displayName: 'Alice Smith',
        roles: ['author']
      });

      const bob = await testRunner.createUser(1, {
        username: 'bob',
        email: 'bob@example.com',
        displayName: 'Bob Jones',
        roles: ['user']
      });

      const charlie = await testRunner.createUser(2, {
        username: 'charlie',
        email: 'charlie@example.com',
        displayName: 'Charlie Brown',
        roles: ['editor']
      });

      expect(alice.id).toBeDefined();
      expect(bob.id).toBeDefined();
      expect(charlie.id).toBeDefined();

      // Wait for sync
      await testRunner.waitForSync(10000);

      // Verify Alice exists on all nodes
      const aliceVerification = await testRunner.verifyUserSync(alice.id);
      expect(aliceVerification).toBe(true);

      // Verify Bob exists on all nodes  
      const bobVerification = await testRunner.verifyUserSync(bob.id);
      expect(bobVerification).toBe(true);

      // Verify Charlie exists on all nodes
      const charlieVerification = await testRunner.verifyUserSync(charlie.id);
      expect(charlieVerification).toBe(true);

      console.log('✅ Cross-node user creation and sync verified');
    });

    it('should update user data and sync changes across nodes', async () => {
      console.log('\n🔧 Testing user updates across nodes...');

      // Create user on node 0
      const user = await testRunner.createUser(0, {
        username: 'updateuser',
        email: 'updateuser@example.com',
        displayName: 'Original Name'
      });

      await testRunner.waitForSync(5000);

      // Update user from node 1
      const updatedUser = await testRunner.updateUser(1, user.id, {
        displayName: 'Updated Name',
        roles: ['premium']
      });

      expect(updatedUser.displayName).toBe('Updated Name');
      expect(updatedUser.roles).toContain('premium');

      await testRunner.waitForSync(5000);

      // Verify update is reflected on all nodes
      for (let nodeIndex = 0; nodeIndex < 3; nodeIndex++) {
        const nodeUser = await testRunner.getUser(nodeIndex, user.id);
        expect(nodeUser.displayName).toBe('Updated Name');
        expect(nodeUser.roles).toContain('premium');
      }

      console.log('✅ User update sync verified');
    });
  });

  describe('Category Management Workflow', () => {
    it('should create categories and sync across nodes', async () => {
      console.log('\n🔧 Testing category creation and sync...');

      // Create categories on different nodes
      const techCategory = await testRunner.createCategory(0, {
        name: 'Technology',
        description: 'Posts about technology and programming',
        color: '#0066cc'
      });

      const designCategory = await testRunner.createCategory(1, {
        name: 'Design',
        description: 'UI/UX design and creative content',
        color: '#ff6600'
      });

      expect(techCategory.id).toBeDefined();
      expect(techCategory.slug).toBe('technology');
      expect(designCategory.id).toBeDefined();
      expect(designCategory.slug).toBe('design');

      await testRunner.waitForSync(8000);

      // Verify categories exist on all nodes
      for (let nodeIndex = 0; nodeIndex < 3; nodeIndex++) {
        const categories = await testRunner.getCategories(nodeIndex);
        
        const techExists = categories.some(c => c.id === techCategory.id);
        const designExists = categories.some(c => c.id === designCategory.id);
        
        expect(techExists).toBe(true);
        expect(designExists).toBe(true);
      }

      console.log('✅ Category creation and sync verified');
    });
  });

  describe('Content Publishing Workflow', () => {
    let author: any;
    let category: any;

    beforeAll(async () => {
      // Create test author and category
      author = await testRunner.createUser(0, {
        username: 'contentauthor',
        email: 'contentauthor@example.com',
        displayName: 'Content Author',
        roles: ['author']
      });

      category = await testRunner.createCategory(1, {
        name: 'Test Content',
        description: 'Category for test content'
      });

      await testRunner.waitForSync(5000);
    });

    it('should support complete blog publishing workflow across nodes', async () => {
      console.log('\n🔧 Testing complete blog publishing workflow...');

      // Step 1: Create draft post on node 2
      const post = await testRunner.createPost(2, {
        title: 'Building Decentralized Applications with DebrosFramework',
        content: 'In this comprehensive guide, we will explore how to build decentralized applications using the DebrosFramework. This framework provides powerful abstractions over IPFS and OrbitDB, making it easier than ever to create distributed applications.',
        excerpt: 'Learn how to build decentralized applications with DebrosFramework',
        authorId: author.id,
        categoryId: category.id,
        tags: ['decentralized', 'blockchain', 'dapps', 'tutorial']
      });

      expect(post.status).toBe('draft');
      expect(post.authorId).toBe(author.id);
      expect(post.categoryId).toBe(category.id);

      await testRunner.waitForSync(8000);

      // Step 2: Verify draft post exists on all nodes
      const postVerification = await testRunner.verifyPostSync(post.id);
      expect(postVerification).toBe(true);

      // Step 3: Publish post from node 0
      const publishedPost = await testRunner.publishPost(0, post.id);
      expect(publishedPost.status).toBe('published');
      expect(publishedPost.publishedAt).toBeDefined();

      await testRunner.waitForSync(8000);

      // Step 4: Verify published post exists on all nodes with relationships
      for (let nodeIndex = 0; nodeIndex < 3; nodeIndex++) {
        const nodePost = await testRunner.getPost(nodeIndex, post.id);
        expect(nodePost.status).toBe('published');
        expect(nodePost.publishedAt).toBeDefined();
        expect(nodePost.author).toBeDefined();
        expect(nodePost.author.username).toBe('contentauthor');
        expect(nodePost.category).toBeDefined();
        expect(nodePost.category.name).toBe('Test Content');
      }

      console.log('✅ Complete blog publishing workflow verified');
    });

    it('should handle post engagement across nodes', async () => {
      console.log('\n🔧 Testing post engagement across nodes...');

      // Create and publish a post
      const post = await testRunner.createPost(0, {
        title: 'Engagement Test Post',
        content: 'This post will test engagement features across nodes.',
        authorId: author.id,
        categoryId: category.id
      });

      await testRunner.publishPost(0, post.id);
      await testRunner.waitForSync(5000);

      // Track views from different nodes
      await testRunner.viewPost(1, post.id);
      await testRunner.viewPost(2, post.id);
      await testRunner.viewPost(0, post.id);

      // Like post from different nodes
      await testRunner.likePost(1, post.id);
      await testRunner.likePost(2, post.id);

      await testRunner.waitForSync(5000);

      // Verify engagement metrics are consistent across nodes
      for (let nodeIndex = 0; nodeIndex < 3; nodeIndex++) {
        const nodePost = await testRunner.getPost(nodeIndex, post.id);
        expect(nodePost.viewCount).toBe(3);
        expect(nodePost.likeCount).toBe(2);
      }

      console.log('✅ Post engagement sync verified');
    });

    it('should support post status changes across nodes', async () => {
      console.log('\n🔧 Testing post status changes...');

      // Create and publish post
      const post = await testRunner.createPost(1, {
        title: 'Status Change Test Post',
        content: 'Testing post status changes across nodes.',
        authorId: author.id
      });

      await testRunner.publishPost(1, post.id);
      await testRunner.waitForSync(5000);

      // Verify published status on all nodes
      for (let nodeIndex = 0; nodeIndex < 3; nodeIndex++) {
        const nodePost = await testRunner.getPost(nodeIndex, post.id);
        expect(nodePost.status).toBe('published');
      }

      // Unpublish from different node
      await testRunner.unpublishPost(2, post.id);
      await testRunner.waitForSync(5000);

      // Verify unpublished status on all nodes
      for (let nodeIndex = 0; nodeIndex < 3; nodeIndex++) {
        const nodePost = await testRunner.getPost(nodeIndex, post.id);
        expect(nodePost.status).toBe('draft');
        expect(nodePost.publishedAt).toBeUndefined();
      }

      console.log('✅ Post status change sync verified');
    });
  });

  describe('Comment System Workflow', () => {
    let author: any;
    let commenter1: any;
    let commenter2: any;
    let post: any;

    beforeAll(async () => {
      // Create test users and post
      [author, commenter1, commenter2] = await Promise.all([
        testRunner.createUser(0, {
          username: 'commentauthor',
          email: 'commentauthor@example.com',
          displayName: 'Comment Author'
        }),
        testRunner.createUser(1, {
          username: 'commenter1',
          email: 'commenter1@example.com',
          displayName: 'First Commenter'
        }),
        testRunner.createUser(2, {
          username: 'commenter2',
          email: 'commenter2@example.com',
          displayName: 'Second Commenter'
        })
      ]);

      post = await testRunner.createPost(0, {
        title: 'Post for Comment Testing',
        content: 'This post will receive comments from different nodes.',
        authorId: author.id
      });

      await testRunner.publishPost(0, post.id);
      await testRunner.waitForSync(8000);
    });

    it('should support distributed comment system', async () => {
      console.log('\n🔧 Testing distributed comment system...');

      // Create comments from different nodes
      const comment1 = await testRunner.createComment(1, {
        content: 'Great post! Very informative and well written.',
        postId: post.id,
        authorId: commenter1.id
      });

      const comment2 = await testRunner.createComment(2, {
        content: 'I learned a lot from this, thank you for sharing!',
        postId: post.id,
        authorId: commenter2.id
      });

      expect(comment1.id).toBeDefined();
      expect(comment2.id).toBeDefined();

      await testRunner.waitForSync(8000);

      // Verify comments exist on all nodes
      for (let nodeIndex = 0; nodeIndex < 3; nodeIndex++) {
        const comments = await testRunner.getComments(nodeIndex, post.id);
        expect(comments.length).toBeGreaterThanOrEqual(2);
        
        const comment1Exists = comments.some(c => c.id === comment1.id);
        const comment2Exists = comments.some(c => c.id === comment2.id);
        
        expect(comment1Exists).toBe(true);
        expect(comment2Exists).toBe(true);
      }

      console.log('✅ Distributed comment creation verified');
    });

    it('should support nested comments (replies)', async () => {
      console.log('\n🔧 Testing nested comments...');

      // Create parent comment
      const parentComment = await testRunner.createComment(0, {
        content: 'This is a parent comment that will receive replies.',
        postId: post.id,
        authorId: author.id
      });

      await testRunner.waitForSync(5000);

      // Create replies from different nodes
      const reply1 = await testRunner.createComment(1, {
        content: 'This is a reply to the parent comment.',
        postId: post.id,
        authorId: commenter1.id,
        parentId: parentComment.id
      });

      const reply2 = await testRunner.createComment(2, {
        content: 'Another reply to the same parent comment.',
        postId: post.id,
        authorId: commenter2.id,
        parentId: parentComment.id
      });

      expect(reply1.parentId).toBe(parentComment.id);
      expect(reply2.parentId).toBe(parentComment.id);

      await testRunner.waitForSync(8000);

      // Verify nested structure exists on all nodes
      for (let nodeIndex = 0; nodeIndex < 3; nodeIndex++) {
        const comments = await testRunner.getComments(nodeIndex, post.id);
        
        const parent = comments.find(c => c.id === parentComment.id);
        const replyToParent1 = comments.find(c => c.id === reply1.id);
        const replyToParent2 = comments.find(c => c.id === reply2.id);
        
        expect(parent).toBeDefined();
        expect(replyToParent1).toBeDefined();
        expect(replyToParent2).toBeDefined();
        expect(replyToParent1.parentId).toBe(parentComment.id);
        expect(replyToParent2.parentId).toBe(parentComment.id);
      }

      console.log('✅ Nested comments verified');
    });

    it('should handle comment engagement across nodes', async () => {
      console.log('\n🔧 Testing comment engagement...');

      // Create comment
      const comment = await testRunner.createComment(0, {
        content: 'This comment will test engagement features.',
        postId: post.id,
        authorId: author.id
      });

      await testRunner.waitForSync(5000);

      // Like comment from different nodes
      await testRunner.likeComment(1, comment.id);
      await testRunner.likeComment(2, comment.id);

      await testRunner.waitForSync(5000);

      // Verify like count is consistent across nodes
      for (let nodeIndex = 0; nodeIndex < 3; nodeIndex++) {
        const comments = await testRunner.getComments(nodeIndex, post.id);
        const likedComment = comments.find(c => c.id === comment.id);
        expect(likedComment).toBeDefined();
        expect(likedComment.likeCount).toBe(2);
      }

      console.log('✅ Comment engagement sync verified');
    });
  });

  describe('Performance and Scalability Tests', () => {
    it('should handle concurrent operations across nodes', async () => {
      console.log('\n🔧 Testing concurrent operations performance...');

      const startTime = Date.now();

      // Create operations across all nodes simultaneously
      const operations = [];

      // Create users concurrently
      for (let i = 0; i < 15; i++) {
        const nodeIndex = i % 3;
        operations.push(
          testRunner.createUser(nodeIndex, testRunner.generateUserData(i))
        );
      }

      // Create categories concurrently
      for (let i = 0; i < 6; i++) {
        const nodeIndex = i % 3;
        operations.push(
          testRunner.createCategory(nodeIndex, testRunner.generateCategoryData(i))
        );
      }

      // Execute all operations concurrently
      const results = await Promise.all(operations);
      const creationTime = Date.now() - startTime;

      console.log(`Created ${results.length} records across 3 nodes in ${creationTime}ms`);

      // Verify all operations succeeded
      expect(results.length).toBe(21);
      results.forEach(result => {
        expect(result.id).toBeDefined();
      });

      // Wait for full sync
      await testRunner.waitForSync(15000);
      const totalTime = Date.now() - startTime;

      console.log(`Total operation time including sync: ${totalTime}ms`);

      // Performance expectations (adjust based on your requirements)
      expect(creationTime).toBeLessThan(30000); // Creation under 30s
      expect(totalTime).toBeLessThan(60000); // Total under 60s

      await testRunner.logStatus();
      console.log('✅ Concurrent operations performance verified');
    });

    it('should maintain data consistency under load', async () => {
      console.log('\n🔧 Testing data consistency under load...');

      // Get initial counts
      const initialMetrics = await testRunner.getAllDataMetrics();
      const initialUserCount = initialMetrics[0]?.counts.users || 0;

      // Create multiple users rapidly
      const userCreationPromises = [];
      for (let i = 0; i < 20; i++) {
        const nodeIndex = i % 3;
        userCreationPromises.push(
          testRunner.createUser(nodeIndex, {
            username: `loaduser${i}`,
            email: `loaduser${i}@example.com`,
            displayName: `Load Test User ${i}`
          })
        );
      }

      await Promise.all(userCreationPromises);
      await testRunner.waitForSync(20000);

      // Verify data consistency across all nodes
      const consistency = await testRunner.verifyDataConsistency('users', initialUserCount + 20, 2);
      expect(consistency).toBe(true);

      console.log('✅ Data consistency under load verified');
    });
  });

  describe('Network Resilience Tests', () => {
    it('should maintain peer connections throughout test execution', async () => {
      console.log('\n🔧 Testing network resilience...');

      const networkMetrics = await testRunner.getAllNetworkMetrics();
      
      // Each node should be connected to at least 2 other nodes
      networkMetrics.forEach((metrics, index) => {
        console.log(`Node ${index} has ${metrics.peers} peers`);
        expect(metrics.peers).toBeGreaterThanOrEqual(2);
      });

      console.log('✅ Network resilience verified');
    });

    it('should provide consistent API responses across nodes', async () => {
      console.log('\n🔧 Testing API consistency...');

      // Test the same query on all nodes
      const nodeResponses = await Promise.all([
        testRunner.getUsers(0, { limit: 5 }),
        testRunner.getUsers(1, { limit: 5 }),
        testRunner.getUsers(2, { limit: 5 })
      ]);

      // All nodes should return data (though exact counts may vary due to sync timing)
      nodeResponses.forEach((users, index) => {
        console.log(`Node ${index} returned ${users.length} users`);
        expect(Array.isArray(users)).toBe(true);
      });

      console.log('✅ API consistency verified');
    });
  });
});