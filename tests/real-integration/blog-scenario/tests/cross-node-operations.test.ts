import { describe, test, expect, beforeAll } from '@jest/globals';
import { blogTestHelper, TestUser, TestCategory, TestPost, TestComment } from './setup';

describe('Cross-Node Operations', () => {
  let users: TestUser[] = [];
  let categories: TestCategory[] = [];
  let posts: TestPost[] = [];

  beforeAll(async () => {
    console.log('🔄 Waiting for all nodes to be ready...');
    await blogTestHelper.waitForNodesReady();
    console.log('✅ All nodes are ready for cross-node testing');
  }, 60000);

  describe('Distributed Content Creation', () => {
    test('should create users on different nodes', async () => {
      const nodes = blogTestHelper.getNodes();
      
      // Create one user on each node
      for (let i = 0; i < nodes.length; i++) {
        const testData = blogTestHelper.generateTestData();
        const user = await blogTestHelper.createUser(testData.user, nodes[i].id);
        
        expect(user).toBeDefined();
        expect(user.id).toBeDefined();
        users.push(user);
        
        console.log(`✅ Created user ${user.username} on ${nodes[i].id}`);
      }
      
      expect(users).toHaveLength(3);
    }, 45000);

    test('should verify users are replicated across all nodes', async () => {
      // Wait for replication
      await blogTestHelper.sleep(3000);
      
      for (const user of users) {
        for (const node of blogTestHelper.getNodes()) {
          const retrievedUser = await blogTestHelper.getUser(user.id!, node.id);
          
          expect(retrievedUser).toBeDefined();
          expect(retrievedUser!.id).toBe(user.id);
          expect(retrievedUser!.username).toBe(user.username);
          
          console.log(`✅ User ${user.username} found on ${node.id}`);
        }
      }
    }, 45000);

    test('should create categories on different nodes', async () => {
      const nodes = blogTestHelper.getNodes();
      
      for (let i = 0; i < nodes.length; i++) {
        const testData = blogTestHelper.generateTestData();
        const category = await blogTestHelper.createCategory(testData.category, nodes[i].id);
        
        expect(category).toBeDefined();
        expect(category.id).toBeDefined();
        categories.push(category);
        
        console.log(`✅ Created category ${category.name} on ${nodes[i].id}`);
      }
      
      expect(categories).toHaveLength(3);
    }, 45000);

    test('should create posts with cross-node relationships', async () => {
      const nodes = blogTestHelper.getNodes();
      
      // Create posts where author and category are from different nodes
      for (let i = 0; i < nodes.length; i++) {
        const authorIndex = i;
        const categoryIndex = (i + 1) % nodes.length; // Use next node's category
        const nodeIndex = (i + 2) % nodes.length; // Create on third node
        
        const testData = blogTestHelper.generateTestData();
        const postData = testData.post(users[authorIndex].id!, categories[categoryIndex].id!);
        
        const post = await blogTestHelper.createPost(postData, nodes[nodeIndex].id);
        
        expect(post).toBeDefined();
        expect(post.id).toBeDefined();
        expect(post.authorId).toBe(users[authorIndex].id);
        expect(post.categoryId).toBe(categories[categoryIndex].id);
        posts.push(post);
        
        console.log(
          `✅ Created post "${post.title}" on ${nodes[nodeIndex].id} ` +
          `(author from node-${authorIndex + 1}, category from node-${categoryIndex + 1})`
        );
      }
      
      expect(posts).toHaveLength(3);
    }, 45000);

    test('should verify cross-node posts are accessible from all nodes', async () => {
      // Wait for replication
      await blogTestHelper.sleep(3000);
      
      for (const post of posts) {
        for (const node of blogTestHelper.getNodes()) {
          const retrievedPost = await blogTestHelper.getPost(post.id!, node.id);
          
          expect(retrievedPost).toBeDefined();
          expect(retrievedPost!.id).toBe(post.id);
          expect(retrievedPost!.title).toBe(post.title);
          expect(retrievedPost!.authorId).toBe(post.authorId);
          expect(retrievedPost!.categoryId).toBe(post.categoryId);
        }
      }
      
      console.log('✅ All cross-node posts are accessible from all nodes');
    }, 45000);
  });

  describe('Concurrent Operations', () => {
    test('should handle concurrent likes on same post from different nodes', async () => {
      const post = posts[0];
      const nodes = blogTestHelper.getNodes();
      
      // Perform concurrent likes from all nodes
      const likePromises = nodes.map(node => 
        blogTestHelper.likePost(post.id!, node.id)
      );
      
      const results = await Promise.all(likePromises);
      
      // All should succeed
      results.forEach(result => {
        expect(result).toBeDefined();
        expect(result.likeCount).toBeGreaterThan(0);
      });
      
      // Wait for eventual consistency
      await blogTestHelper.sleep(3000);
      
      // Verify final like count is consistent across nodes
      const finalCounts: number[] = [];
      for (const node of nodes) {
        const updatedPost = await blogTestHelper.getPost(post.id!, node.id);
        expect(updatedPost).toBeDefined();
        finalCounts.push(updatedPost!.likeCount || 0);
      }
      
      // All nodes should have the same final count
      const uniqueCounts = [...new Set(finalCounts)];
      expect(uniqueCounts).toHaveLength(1);
      
      console.log(`✅ Concurrent likes handled, final count: ${finalCounts[0]}`);
    }, 45000);

    test('should handle simultaneous comment creation', async () => {
      const post = posts[1];
      const nodes = blogTestHelper.getNodes();
      
      // Create comments simultaneously from different nodes
      const commentPromises = nodes.map((node, index) => {
        const testData = blogTestHelper.generateTestData();
        const commentData = testData.comment(post.id!, users[index].id!);
        return blogTestHelper.createComment(commentData, node.id);
      });
      
      const comments = await Promise.all(commentPromises);
      
      // All comments should be created successfully
      comments.forEach((comment, index) => {
        expect(comment).toBeDefined();
        expect(comment.id).toBeDefined();
        expect(comment.postId).toBe(post.id);
        expect(comment.authorId).toBe(users[index].id);
      });
      
      // Wait for replication
      await blogTestHelper.sleep(3000);
      
      // Verify all comments are visible from all nodes
      for (const node of nodes) {
        const result = await blogTestHelper.getPostComments(post.id!, node.id);
        expect(result.comments.length).toBeGreaterThanOrEqual(3);
        
        // Verify all our comments are present
        for (const comment of comments) {
          const found = result.comments.find(c => c.id === comment.id);
          expect(found).toBeDefined();
        }
      }
      
      console.log(`✅ Created ${comments.length} simultaneous comments`);
    }, 45000);
  });

  describe('Load Distribution', () => {
    test('should distribute read operations across nodes', async () => {
      const readCounts = new Map<string, number>();
      const totalReads = 30;
      
      // Perform multiple reads and track which nodes are used
      for (let i = 0; i < totalReads; i++) {
        const randomPost = posts[Math.floor(Math.random() * posts.length)];
        const node = blogTestHelper.getRandomNode();
        
        const post = await blogTestHelper.getPost(randomPost.id!, node.id);
        expect(post).toBeDefined();
        
        readCounts.set(node.id, (readCounts.get(node.id) || 0) + 1);
      }
      
      // Verify reads were distributed across nodes
      const nodeIds = blogTestHelper.getNodes().map(n => n.id);
      nodeIds.forEach(nodeId => {
        const count = readCounts.get(nodeId) || 0;
        console.log(`${nodeId}: ${count} reads`);
        expect(count).toBeGreaterThan(0); // Each node should have at least one read
      });
      
      console.log('✅ Read operations distributed across all nodes');
    }, 45000);

    test('should verify consistent data across all read operations', async () => {
      // Read the same post from all nodes multiple times
      const post = posts[0];
      const readResults: TestPost[] = [];
      
      for (let i = 0; i < 10; i++) {
        for (const node of blogTestHelper.getNodes()) {
          const result = await blogTestHelper.getPost(post.id!, node.id);
          expect(result).toBeDefined();
          readResults.push(result!);
        }
      }
      
      // Verify all reads return identical data
      readResults.forEach(result => {
        expect(result.id).toBe(post.id);
        expect(result.title).toBe(post.title);
        expect(result.content).toBe(post.content);
        expect(result.authorId).toBe(post.authorId);
        expect(result.categoryId).toBe(post.categoryId);
      });
      
      console.log(`✅ ${readResults.length} read operations returned consistent data`);
    }, 45000);
  });

  describe('Network Metrics', () => {
    test('should show network connectivity between nodes', async () => {
      for (const node of blogTestHelper.getNodes()) {
        const metrics = await blogTestHelper.getNodeMetrics(node.id);
        
        expect(metrics).toBeDefined();
        expect(metrics.nodeId).toBe(node.id);
        
        console.log(`📊 ${node.id} framework metrics:`, {
          services: metrics.services,
          environment: metrics.environment,
          features: metrics.features
        });
      }
    }, 30000);

    test('should verify data consistency across all nodes', async () => {
      const allMetrics = [];
      
      for (const node of blogTestHelper.getNodes()) {
        const metrics = await blogTestHelper.getDataMetrics(node.id);
        allMetrics.push(metrics);
        
        console.log(`📊 ${node.id} data counts:`, metrics.counts);
      }
      
      // Verify all nodes have the same data counts (eventual consistency)
      const firstCounts = allMetrics[0].counts;
      allMetrics.forEach((metrics, index) => {
        expect(metrics.counts.users).toBe(firstCounts.users);
        expect(metrics.counts.categories).toBe(firstCounts.categories);
        expect(metrics.counts.posts).toBe(firstCounts.posts);
        
        console.log(`✅ Node ${index + 1} data counts match reference`);
      });
      
      console.log('✅ Data consistency verified across all nodes');
    }, 30000);
  });
});
