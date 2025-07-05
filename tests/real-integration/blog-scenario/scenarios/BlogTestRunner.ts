import { ApiClient } from '../../shared/utils/ApiClient';
import { SyncWaiter } from '../../shared/utils/SyncWaiter';
import { CreateUserRequest, CreateCategoryRequest, CreatePostRequest, CreateCommentRequest } from '../models/BlogModels';

export interface BlogTestConfig {
  nodeEndpoints: string[];
  syncTimeout: number;
  operationTimeout: number;
}

export class BlogTestRunner {
  private apiClients: ApiClient[];
  private syncWaiter: SyncWaiter;

  constructor(private config: BlogTestConfig) {
    this.apiClients = config.nodeEndpoints.map(endpoint => new ApiClient(endpoint));
    this.syncWaiter = new SyncWaiter(this.apiClients);
  }

  // Initialization and setup
  async waitForNodesReady(timeout: number = 60000): Promise<boolean> {
    console.log('🔧 Waiting for blog nodes to be ready...');
    return await this.syncWaiter.waitForNodesReady(timeout);
  }

  async waitForPeerConnections(timeout: number = 30000): Promise<boolean> {
    console.log('🔧 Waiting for peer connections...');
    return await this.syncWaiter.waitForPeerConnections(2, timeout);
  }

  async waitForSync(timeout: number = 10000): Promise<void> {
    await this.syncWaiter.waitForSync(timeout);
  }

  // User operations
  async createUser(nodeIndex: number, userData: CreateUserRequest): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.post('/api/users', userData);
    
    if (response.error) {
      throw new Error(`Failed to create user on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async getUser(nodeIndex: number, userId: string): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.get(`/api/users/${userId}`);
    
    if (response.error) {
      throw new Error(`Failed to get user on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async getUsers(nodeIndex: number, options: { page?: number; limit?: number; search?: string } = {}): Promise<any[]> {
    const client = this.getClient(nodeIndex);
    const queryString = new URLSearchParams(options as any).toString();
    const response = await client.get(`/api/users?${queryString}`);
    
    if (response.error) {
      throw new Error(`Failed to get users on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data.users;
  }

  async updateUser(nodeIndex: number, userId: string, updateData: any): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.put(`/api/users/${userId}`, updateData);
    
    if (response.error) {
      throw new Error(`Failed to update user on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  // Category operations
  async createCategory(nodeIndex: number, categoryData: CreateCategoryRequest): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.post('/api/categories', categoryData);
    
    if (response.error) {
      throw new Error(`Failed to create category on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async getCategory(nodeIndex: number, categoryId: string): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.get(`/api/categories/${categoryId}`);
    
    if (response.error) {
      throw new Error(`Failed to get category on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async getCategories(nodeIndex: number): Promise<any[]> {
    const client = this.getClient(nodeIndex);
    const response = await client.get('/api/categories');
    
    if (response.error) {
      throw new Error(`Failed to get categories on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data.categories;
  }

  // Post operations
  async createPost(nodeIndex: number, postData: CreatePostRequest): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.post('/api/posts', postData);
    
    if (response.error) {
      throw new Error(`Failed to create post on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async getPost(nodeIndex: number, postId: string): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.get(`/api/posts/${postId}`);
    
    if (response.error) {
      throw new Error(`Failed to get post on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async getPosts(nodeIndex: number, options: { 
    page?: number; 
    limit?: number; 
    status?: string;
    authorId?: string;
    categoryId?: string;
    tag?: string;
  } = {}): Promise<any[]> {
    const client = this.getClient(nodeIndex);
    const queryString = new URLSearchParams(options as any).toString();
    const response = await client.get(`/api/posts?${queryString}`);
    
    if (response.error) {
      throw new Error(`Failed to get posts on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data.posts;
  }

  async updatePost(nodeIndex: number, postId: string, updateData: any): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.put(`/api/posts/${postId}`, updateData);
    
    if (response.error) {
      throw new Error(`Failed to update post on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async publishPost(nodeIndex: number, postId: string): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.post(`/api/posts/${postId}/publish`, {});
    
    if (response.error) {
      throw new Error(`Failed to publish post on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async unpublishPost(nodeIndex: number, postId: string): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.post(`/api/posts/${postId}/unpublish`, {});
    
    if (response.error) {
      throw new Error(`Failed to unpublish post on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async likePost(nodeIndex: number, postId: string): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.post(`/api/posts/${postId}/like`, {});
    
    if (response.error) {
      throw new Error(`Failed to like post on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async viewPost(nodeIndex: number, postId: string): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.post(`/api/posts/${postId}/view`, {});
    
    if (response.error) {
      throw new Error(`Failed to view post on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  // Comment operations
  async createComment(nodeIndex: number, commentData: CreateCommentRequest): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.post('/api/comments', commentData);
    
    if (response.error) {
      throw new Error(`Failed to create comment on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async getComments(nodeIndex: number, postId: string): Promise<any[]> {
    const client = this.getClient(nodeIndex);
    const response = await client.get(`/api/posts/${postId}/comments`);
    
    if (response.error) {
      throw new Error(`Failed to get comments on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data.comments;
  }

  async approveComment(nodeIndex: number, commentId: string): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.post(`/api/comments/${commentId}/approve`, {});
    
    if (response.error) {
      throw new Error(`Failed to approve comment on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async likeComment(nodeIndex: number, commentId: string): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.post(`/api/comments/${commentId}/like`, {});
    
    if (response.error) {
      throw new Error(`Failed to like comment on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  // Metrics and monitoring
  async getNetworkMetrics(nodeIndex: number): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.get('/api/metrics/network');
    
    if (response.error) {
      throw new Error(`Failed to get network metrics on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async getDataMetrics(nodeIndex: number): Promise<any> {
    const client = this.getClient(nodeIndex);
    const response = await client.get('/api/metrics/data');
    
    if (response.error) {
      throw new Error(`Failed to get data metrics on node ${nodeIndex}: ${response.error}`);
    }
    
    return response.data;
  }

  async getAllNetworkMetrics(): Promise<any[]> {
    const metrics = [];
    for (let i = 0; i < this.apiClients.length; i++) {
      try {
        const nodeMetrics = await this.getNetworkMetrics(i);
        metrics.push(nodeMetrics);
      } catch (error) {
        console.warn(`Failed to get metrics from node ${i}: ${error.message}`);
      }
    }
    return metrics;
  }

  async getAllDataMetrics(): Promise<any[]> {
    const metrics = [];
    for (let i = 0; i < this.apiClients.length; i++) {
      try {
        const nodeMetrics = await this.getDataMetrics(i);
        metrics.push(nodeMetrics);
      } catch (error) {
        console.warn(`Failed to get data metrics from node ${i}: ${error.message}`);
      }
    }
    return metrics;
  }

  // Data consistency checks
  async verifyDataConsistency(dataType: 'users' | 'posts' | 'comments' | 'categories', expectedCount: number, tolerance: number = 0): Promise<boolean> {
    return await this.syncWaiter.waitForDataConsistency(dataType, expectedCount, this.config.syncTimeout, tolerance);
  }

  async verifyUserSync(userId: string): Promise<boolean> {
    console.log(`🔍 Verifying user ${userId} sync across all nodes...`);
    
    try {
      const userPromises = this.apiClients.map((_, index) => this.getUser(index, userId));
      const users = await Promise.all(userPromises);
      
      // Check if all users have the same data
      const firstUser = users[0];
      const allSame = users.every(user => 
        user.id === firstUser.id &&
        user.username === firstUser.username &&
        user.email === firstUser.email
      );
      
      if (allSame) {
        console.log(`✅ User ${userId} is consistent across all nodes`);
        return true;
      } else {
        console.log(`❌ User ${userId} is not consistent across nodes`);
        return false;
      }
    } catch (error) {
      console.log(`❌ Failed to verify user sync: ${error.message}`);
      return false;
    }
  }

  async verifyPostSync(postId: string): Promise<boolean> {
    console.log(`🔍 Verifying post ${postId} sync across all nodes...`);
    
    try {
      const postPromises = this.apiClients.map((_, index) => this.getPost(index, postId));
      const posts = await Promise.all(postPromises);
      
      // Check if all posts have the same data
      const firstPost = posts[0];
      const allSame = posts.every(post => 
        post.id === firstPost.id &&
        post.title === firstPost.title &&
        post.status === firstPost.status
      );
      
      if (allSame) {
        console.log(`✅ Post ${postId} is consistent across all nodes`);
        return true;
      } else {
        console.log(`❌ Post ${postId} is not consistent across nodes`);
        return false;
      }
    } catch (error) {
      console.log(`❌ Failed to verify post sync: ${error.message}`);
      return false;
    }
  }

  // Utility methods
  private getClient(nodeIndex: number): ApiClient {
    if (nodeIndex >= this.apiClients.length) {
      throw new Error(`Node index ${nodeIndex} is out of range. Available nodes: 0-${this.apiClients.length - 1}`);
    }
    return this.apiClients[nodeIndex];
  }

  async logStatus(): Promise<void> {
    console.log('\n📊 Blog Test Environment Status:');
    console.log(`Total Nodes: ${this.config.nodeEndpoints.length}`);
    
    const [networkMetrics, dataMetrics] = await Promise.all([
      this.getAllNetworkMetrics(),
      this.getAllDataMetrics()
    ]);
    
    networkMetrics.forEach((metrics, index) => {
      const data = dataMetrics[index];
      console.log(`Node ${index} (${metrics.nodeId}):`);
      console.log(`  Peers: ${metrics.peers}`);
      if (data) {
        console.log(`  Data: Users=${data.counts.users}, Posts=${data.counts.posts}, Comments=${data.counts.comments}, Categories=${data.counts.categories}`);
      }
    });
    console.log('');
  }

  async cleanup(): Promise<void> {
    console.log('🧹 Cleaning up blog test environment...');
    // Any cleanup logic if needed
  }

  // Test data generators
  generateUserData(index: number): CreateUserRequest {
    return {
      username: `testuser${index}`,
      email: `testuser${index}@example.com`,
      displayName: `Test User ${index}`,
      roles: ['user']
    };
  }

  generateCategoryData(index: number): CreateCategoryRequest {
    const categories = [
      { name: 'Technology', description: 'Posts about technology and programming' },
      { name: 'Design', description: 'UI/UX design and creative content' },
      { name: 'Business', description: 'Business strategies and entrepreneurship' },
      { name: 'Lifestyle', description: 'Lifestyle and personal development' },
      { name: 'Science', description: 'Scientific discoveries and research' }
    ];
    
    const category = categories[index % categories.length];
    return {
      name: `${category.name} ${Math.floor(index / categories.length) || ''}`.trim(),
      description: category.description
    };
  }

  generatePostData(authorId: string, categoryId?: string, index: number = 0): CreatePostRequest {
    return {
      title: `Test Blog Post ${index + 1}`,
      content: `This is the content of test blog post ${index + 1}. It contains detailed information about the topic and provides valuable insights to readers. The content is long enough to test the system's handling of substantial text data.`,
      excerpt: `This is a test blog post excerpt ${index + 1}`,
      authorId,
      categoryId,
      tags: [`tag${index}`, 'test', 'blog'],
      status: 'draft'
    };
  }

  generateCommentData(postId: string, authorId: string, index: number = 0, parentId?: string): CreateCommentRequest {
    return {
      content: `This is test comment ${index + 1}. It provides feedback on the blog post.`,
      postId,
      authorId,
      parentId
    };
  }
}