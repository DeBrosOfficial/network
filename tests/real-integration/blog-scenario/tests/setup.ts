import axios, { AxiosResponse } from 'axios';

export interface TestNode {
  id: string;
  baseUrl: string;
  port: number;
}

export interface TestUser {
  id?: string;
  username: string;
  email: string;
  displayName?: string;
  avatar?: string;
  roles?: string[];
}

export interface TestCategory {
  id?: string;
  name: string;
  description?: string;
  color?: string;
}

export interface TestPost {
  id?: string;
  title: string;
  content: string;
  excerpt?: string;
  authorId: string;
  categoryId?: string;
  tags?: string[];
  status?: 'draft' | 'published' | 'archived';
}

export interface TestComment {
  id?: string;
  content: string;
  postId: string;
  authorId: string;
  parentId?: string;
}

export class BlogTestHelper {
  private nodes: TestNode[];
  private timeout: number;

  constructor() {
    this.nodes = [
      { id: 'blog-node-1', baseUrl: 'http://blog-node-1:3000', port: 3000 },
      { id: 'blog-node-2', baseUrl: 'http://blog-node-2:3000', port: 3000 },
      { id: 'blog-node-3', baseUrl: 'http://blog-node-3:3000', port: 3000 }
    ];
    this.timeout = 30000; // 30 seconds
  }

  getNodes(): TestNode[] {
    return this.nodes;
  }

  getRandomNode(): TestNode {
    return this.nodes[Math.floor(Math.random() * this.nodes.length)];
  }

  async waitForNodesReady(): Promise<void> {
    const maxRetries = 30;
    const retryDelay = 1000;

    for (const node of this.nodes) {
      let retries = 0;
      let healthy = false;

      while (retries < maxRetries && !healthy) {
        try {
          const response = await axios.get(`${node.baseUrl}/health`, {
            timeout: 5000
          });
          
          if (response.status === 200 && response.data.status === 'healthy') {
            console.log(`✅ Node ${node.id} is healthy`);
            healthy = true;
          }
        } catch (error) {
          retries++;
          console.log(`⏳ Waiting for ${node.id} to be ready (attempt ${retries}/${maxRetries})`);
          await this.sleep(retryDelay);
        }
      }

      if (!healthy) {
        throw new Error(`Node ${node.id} failed to become healthy after ${maxRetries} attempts`);
      }
    }

    // Additional wait for inter-node connectivity
    console.log('⏳ Waiting for nodes to establish connectivity...');
    await this.sleep(5000);
  }

  async sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  // User operations
  async createUser(user: TestUser, nodeId?: string): Promise<TestUser> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    const response = await axios.post(`${node.baseUrl}/api/users`, user, {
      timeout: this.timeout
    });
    return response.data;
  }

  async getUser(userId: string, nodeId?: string): Promise<TestUser | null> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    try {
      const response = await axios.get(`${node.baseUrl}/api/users/${userId}`, {
        timeout: this.timeout
      });
      return response.data;
    } catch (error) {
      if (axios.isAxiosError(error) && error.response?.status === 404) {
        return null;
      }
      throw error;
    }
  }

  async listUsers(nodeId?: string, params?: any): Promise<{ users: TestUser[], page: number, limit: number }> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    const response = await axios.get(`${node.baseUrl}/api/users`, {
      params,
      timeout: this.timeout
    });
    return response.data;
  }

  // Category operations
  async createCategory(category: TestCategory, nodeId?: string): Promise<TestCategory> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    const response = await axios.post(`${node.baseUrl}/api/categories`, category, {
      timeout: this.timeout
    });
    return response.data;
  }

  async getCategory(categoryId: string, nodeId?: string): Promise<TestCategory | null> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    try {
      const response = await axios.get(`${node.baseUrl}/api/categories/${categoryId}`, {
        timeout: this.timeout
      });
      return response.data;
    } catch (error) {
      if (axios.isAxiosError(error) && error.response?.status === 404) {
        return null;
      }
      throw error;
    }
  }

  async listCategories(nodeId?: string): Promise<{ categories: TestCategory[] }> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    const response = await axios.get(`${node.baseUrl}/api/categories`, {
      timeout: this.timeout
    });
    return response.data;
  }

  // Post operations
  async createPost(post: TestPost, nodeId?: string): Promise<TestPost> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    const response = await axios.post(`${node.baseUrl}/api/posts`, post, {
      timeout: this.timeout
    });
    return response.data;
  }

  async getPost(postId: string, nodeId?: string): Promise<TestPost | null> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    try {
      const response = await axios.get(`${node.baseUrl}/api/posts/${postId}`, {
        timeout: this.timeout
      });
      return response.data;
    } catch (error) {
      if (axios.isAxiosError(error) && error.response?.status === 404) {
        return null;
      }
      throw error;
    }
  }

  async publishPost(postId: string, nodeId?: string): Promise<TestPost> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    const response = await axios.post(`${node.baseUrl}/api/posts/${postId}/publish`, {}, {
      timeout: this.timeout
    });
    return response.data;
  }

  async likePost(postId: string, nodeId?: string): Promise<{ likeCount: number }> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    const response = await axios.post(`${node.baseUrl}/api/posts/${postId}/like`, {}, {
      timeout: this.timeout
    });
    return response.data;
  }

  // Comment operations
  async createComment(comment: TestComment, nodeId?: string): Promise<TestComment> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    const response = await axios.post(`${node.baseUrl}/api/comments`, comment, {
      timeout: this.timeout
    });
    return response.data;
  }

  async getPostComments(postId: string, nodeId?: string): Promise<{ comments: TestComment[] }> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    const response = await axios.get(`${node.baseUrl}/api/posts/${postId}/comments`, {
      timeout: this.timeout
    });
    return response.data;
  }

  // Metrics and health
  async getNodeMetrics(nodeId: string): Promise<any> {
    const node = this.getNodeById(nodeId);
    const response = await axios.get(`${node.baseUrl}/api/metrics/framework`, {
      timeout: this.timeout
    });
    return response.data;
  }

  async getDataMetrics(nodeId?: string): Promise<any> {
    const node = nodeId ? this.getNodeById(nodeId) : this.getRandomNode();
    const response = await axios.get(`${node.baseUrl}/api/metrics/data`, {
      timeout: this.timeout
    });
    return response.data;
  }

  // Utility methods
  private getNodeById(nodeId: string): TestNode {
    const node = this.nodes.find(n => n.id === nodeId);
    if (!node) {
      throw new Error(`Node with id ${nodeId} not found`);
    }
    return node;
  }

  async waitForDataReplication(
    checkFunction: () => Promise<boolean>,
    maxWaitMs: number = 10000,
    intervalMs: number = 500
  ): Promise<void> {
    const startTime = Date.now();
    
    while (Date.now() - startTime < maxWaitMs) {
      if (await checkFunction()) {
        return;
      }
      await this.sleep(intervalMs);
    }
    
    throw new Error(`Data replication timeout after ${maxWaitMs}ms`);
  }

  generateTestData() {
    const timestamp = Date.now();
    const random = Math.random().toString(36).substring(7);
    
    return {
      user: {
        username: `testuser_${random}`,
        email: `test_${random}@example.com`,
        displayName: `Test User ${random}`,
        roles: ['user']
      } as TestUser,
      
      category: {
        name: `Test Category ${random}`,
        description: `Test category created at ${timestamp}`,
        color: '#ff0000'
      } as TestCategory,
      
      post: (authorId: string, categoryId?: string) => ({
        title: `Test Post ${random}`,
        content: `This is test content created at ${timestamp}`,
        excerpt: `Test excerpt ${random}`,
        authorId,
        categoryId,
        tags: ['test', 'integration'],
        status: 'draft' as const
      } as TestPost),
      
      comment: (postId: string, authorId: string) => ({
        content: `Test comment created at ${timestamp}`,
        postId,
        authorId
      } as TestComment)
    };
  }
}

export const blogTestHelper = new BlogTestHelper();
