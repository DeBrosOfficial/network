import { describe, test, expect, beforeAll, afterAll } from '@jest/globals';
import { blogTestHelper, TestUser, TestCategory, TestPost, TestComment } from './setup';

describe('Blog Basic Operations', () => {
  let testUser: TestUser;
  let testCategory: TestCategory;
  let testPost: TestPost;
  let testComment: TestComment;

  beforeAll(async () => {
    console.log('🔄 Waiting for all nodes to be ready...');
    await blogTestHelper.waitForNodesReady();
    console.log('✅ All nodes are ready for testing');
  }, 60000); // 60 second timeout for setup

  describe('User Management', () => {
    test('should create a user successfully', async () => {
      const testData = blogTestHelper.generateTestData();
      
      testUser = await blogTestHelper.createUser(testData.user);
      
      expect(testUser).toBeDefined();
      expect(testUser.id).toBeDefined();
      expect(testUser.username).toBe(testData.user.username);
      expect(testUser.email).toBe(testData.user.email);
      expect(testUser.displayName).toBe(testData.user.displayName);
      
      console.log(`✅ Created user: ${testUser.username} (${testUser.id})`);
    }, 30000);

    test('should retrieve user by ID from all nodes', async () => {
      expect(testUser?.id).toBeDefined();
      
      // Test retrieval from each node
      for (const node of blogTestHelper.getNodes()) {
        const retrievedUser = await blogTestHelper.getUser(testUser.id!, node.id);
        
        expect(retrievedUser).toBeDefined();
        expect(retrievedUser!.id).toBe(testUser.id);
        expect(retrievedUser!.username).toBe(testUser.username);
        expect(retrievedUser!.email).toBe(testUser.email);
        
        console.log(`✅ Retrieved user from ${node.id}: ${retrievedUser!.username}`);
      }
    }, 30000);

    test('should list users with pagination', async () => {
      const result = await blogTestHelper.listUsers();
      
      expect(result).toBeDefined();
      expect(result.users).toBeInstanceOf(Array);
      expect(result.users.length).toBeGreaterThan(0);
      expect(result.page).toBeDefined();
      expect(result.limit).toBeDefined();
      
      // Verify our test user is in the list
      const foundUser = result.users.find(u => u.id === testUser.id);
      expect(foundUser).toBeDefined();
      
      console.log(`✅ Listed ${result.users.length} users`);
    }, 30000);
  });

  describe('Category Management', () => {
    test('should create a category successfully', async () => {
      const testData = blogTestHelper.generateTestData();
      
      testCategory = await blogTestHelper.createCategory(testData.category);
      
      expect(testCategory).toBeDefined();
      expect(testCategory.id).toBeDefined();
      expect(testCategory.name).toBe(testData.category.name);
      expect(testCategory.description).toBe(testData.category.description);
      expect(testCategory.color).toBe(testData.category.color);
      
      console.log(`✅ Created category: ${testCategory.name} (${testCategory.id})`);
    }, 30000);

    test('should retrieve category from all nodes', async () => {
      expect(testCategory?.id).toBeDefined();
      
      for (const node of blogTestHelper.getNodes()) {
        const retrievedCategory = await blogTestHelper.getCategory(testCategory.id!, node.id);
        
        expect(retrievedCategory).toBeDefined();
        expect(retrievedCategory!.id).toBe(testCategory.id);
        expect(retrievedCategory!.name).toBe(testCategory.name);
        
        console.log(`✅ Retrieved category from ${node.id}: ${retrievedCategory!.name}`);
      }
    }, 30000);

    test('should list all categories', async () => {
      const result = await blogTestHelper.listCategories();
      
      expect(result).toBeDefined();
      expect(result.categories).toBeInstanceOf(Array);
      expect(result.categories.length).toBeGreaterThan(0);
      
      // Verify our test category is in the list
      const foundCategory = result.categories.find(c => c.id === testCategory.id);
      expect(foundCategory).toBeDefined();
      
      console.log(`✅ Listed ${result.categories.length} categories`);
    }, 30000);
  });

  describe('Post Management', () => {
    test('should create a post successfully', async () => {
      expect(testUser?.id).toBeDefined();
      expect(testCategory?.id).toBeDefined();
      
      const testData = blogTestHelper.generateTestData();
      const postData = testData.post(testUser.id!, testCategory.id!);
      
      testPost = await blogTestHelper.createPost(postData);
      
      expect(testPost).toBeDefined();
      expect(testPost.id).toBeDefined();
      expect(testPost.title).toBe(postData.title);
      expect(testPost.content).toBe(postData.content);
      expect(testPost.authorId).toBe(testUser.id);
      expect(testPost.categoryId).toBe(testCategory.id);
      expect(testPost.status).toBe('draft');
      
      console.log(`✅ Created post: ${testPost.title} (${testPost.id})`);
    }, 30000);

    test('should retrieve post with relationships from all nodes', async () => {
      expect(testPost?.id).toBeDefined();
      
      for (const node of blogTestHelper.getNodes()) {
        const retrievedPost = await blogTestHelper.getPost(testPost.id!, node.id);
        
        expect(retrievedPost).toBeDefined();
        expect(retrievedPost!.id).toBe(testPost.id);
        expect(retrievedPost!.title).toBe(testPost.title);
        expect(retrievedPost!.authorId).toBe(testUser.id);
        expect(retrievedPost!.categoryId).toBe(testCategory.id);
        
        console.log(`✅ Retrieved post from ${node.id}: ${retrievedPost!.title}`);
      }
    }, 30000);

    test('should publish post and update status', async () => {
      expect(testPost?.id).toBeDefined();
      
      const publishedPost = await blogTestHelper.publishPost(testPost.id!);
      
      expect(publishedPost).toBeDefined();
      expect(publishedPost.status).toBe('published');
      expect(publishedPost.publishedAt).toBeDefined();
      
      // Verify status change is replicated across nodes
      await blogTestHelper.waitForDataReplication(async () => {
        for (const node of blogTestHelper.getNodes()) {
          const post = await blogTestHelper.getPost(testPost.id!, node.id);
          if (!post || post.status !== 'published') {
            return false;
          }
        }
        return true;
      }, 15000);
      
      console.log(`✅ Published post: ${publishedPost.title}`);
    }, 30000);

    test('should like post and increment count', async () => {
      expect(testPost?.id).toBeDefined();
      
      const result = await blogTestHelper.likePost(testPost.id!);
      
      expect(result).toBeDefined();
      expect(result.likeCount).toBeGreaterThan(0);
      
      console.log(`✅ Liked post, count: ${result.likeCount}`);
    }, 30000);
  });

  describe('Comment Management', () => {
    test('should create a comment successfully', async () => {
      expect(testPost?.id).toBeDefined();
      expect(testUser?.id).toBeDefined();
      
      const testData = blogTestHelper.generateTestData();
      const commentData = testData.comment(testPost.id!, testUser.id!);
      
      testComment = await blogTestHelper.createComment(commentData);
      
      expect(testComment).toBeDefined();
      expect(testComment.id).toBeDefined();
      expect(testComment.content).toBe(commentData.content);
      expect(testComment.postId).toBe(testPost.id);
      expect(testComment.authorId).toBe(testUser.id);
      
      console.log(`✅ Created comment: ${testComment.id}`);
    }, 30000);

    test('should retrieve post comments from all nodes', async () => {
      expect(testPost?.id).toBeDefined();
      expect(testComment?.id).toBeDefined();
      
      for (const node of blogTestHelper.getNodes()) {
        const result = await blogTestHelper.getPostComments(testPost.id!, node.id);
        
        expect(result).toBeDefined();
        expect(result.comments).toBeInstanceOf(Array);
        
        // Find our test comment
        const foundComment = result.comments.find(c => c.id === testComment.id);
        expect(foundComment).toBeDefined();
        expect(foundComment!.content).toBe(testComment.content);
        
        console.log(`✅ Retrieved ${result.comments.length} comments from ${node.id}`);
      }
    }, 30000);
  });

  describe('Data Metrics', () => {
    test('should get data metrics from all nodes', async () => {
      for (const node of blogTestHelper.getNodes()) {
        const metrics = await blogTestHelper.getDataMetrics(node.id);
        
        expect(metrics).toBeDefined();
        expect(metrics.nodeId).toBe(node.id);
        expect(metrics.counts).toBeDefined();
        expect(metrics.counts.users).toBeGreaterThan(0);
        expect(metrics.counts.categories).toBeGreaterThan(0);
        expect(metrics.counts.posts).toBeGreaterThan(0);
        expect(metrics.counts.comments).toBeGreaterThan(0);
        
        console.log(`✅ ${node.id} metrics:`, JSON.stringify(metrics.counts, null, 2));
      }
    }, 30000);
  });
});
