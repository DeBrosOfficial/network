---
sidebar_position: 1
---

# Basic Usage Examples

This guide provides practical examples of using DebrosFramework for common development tasks. These examples will help you understand how to implement typical application features using the framework.

## Setting Up Your First Application

### 1. Project Setup

```bash
mkdir my-debros-app
cd my-debros-app
npm init -y
npm install debros-framework @orbitdb/core @helia/helia
npm install --save-dev typescript @types/node ts-node
```

### 2. Basic Configuration

Create `src/config.ts`:

```typescript
export const config = {
  ipfs: {
    // IPFS configuration
    addresses: {
      swarm: ['/ip4/0.0.0.0/tcp/4001'],
      api: '/ip4/127.0.0.1/tcp/5001',
      gateway: '/ip4/127.0.0.1/tcp/8080',
    },
  },
  orbitdb: {
    // OrbitDB configuration
    directory: './orbitdb',
  },
  framework: {
    // Framework configuration
    cacheSize: 1000,
    enableQueryOptimization: true,
    enableAutomaticPinning: true,
  },
};
```

## Simple Blog Application

Let's build a simple blog application to demonstrate basic DebrosFramework usage.

### 1. Define Models

Create `src/models/User.ts`:

```typescript
import { BaseModel, Model, Field, HasMany, BeforeCreate, AfterCreate } from 'debros-framework';
import { Post } from './Post';

@Model({
  scope: 'global',
  type: 'docstore',
  sharding: {
    strategy: 'hash',
    count: 4,
    key: 'id',
  },
})
export class User extends BaseModel {
  @Field({
    type: 'string',
    required: true,
    unique: true,
    minLength: 3,
    maxLength: 20,
    validate: (username: string) => /^[a-zA-Z0-9_]+$/.test(username),
  })
  username: string;

  @Field({
    type: 'string',
    required: true,
    unique: true,
    validate: (email: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email),
    transform: (email: string) => email.toLowerCase(),
  })
  email: string;

  @Field({ type: 'string', required: false, maxLength: 500 })
  bio?: string;

  @Field({ type: 'string', required: false })
  avatarUrl?: string;

  @Field({ type: 'boolean', required: false, default: true })
  isActive: boolean;

  @Field({ type: 'number', required: false, default: () => Date.now() })
  registeredAt: number;

  @Field({ type: 'number', required: false, default: () => Date.now() })
  lastLoginAt: number;

  // Relationships
  @HasMany(() => Post, 'authorId')
  posts: Post[];

  @BeforeCreate()
  setupNewUser() {
    this.registeredAt = Date.now();
    this.lastLoginAt = Date.now();
    this.isActive = true;
  }

  @AfterCreate()
  async afterUserCreated() {
    console.log(`New user created: ${this.username}`);
    // Here you could send a welcome email, create default settings, etc.
  }

  // Helper methods
  updateLastLogin() {
    this.lastLoginAt = Date.now();
    return this.save();
  }

  async getPostCount(): Promise<number> {
    return await Post.query().where('authorId', this.id).count();
  }

  async getRecentPosts(limit: number = 5): Promise<Post[]> {
    return await Post.query()
      .where('authorId', this.id)
      .orderBy('createdAt', 'desc')
      .limit(limit)
      .find();
  }
}
```

Create `src/models/Post.ts`:

```typescript
import {
  BaseModel,
  Model,
  Field,
  BelongsTo,
  HasMany,
  BeforeCreate,
  BeforeUpdate,
} from 'debros-framework';
import { User } from './User';
import { Comment } from './Comment';

@Model({
  scope: 'user',
  type: 'docstore',
  sharding: {
    strategy: 'user',
    count: 2,
    key: 'authorId',
  },
})
export class Post extends BaseModel {
  @Field({ type: 'string', required: true, minLength: 1, maxLength: 200 })
  title: string;

  @Field({ type: 'string', required: true, minLength: 1, maxLength: 10000 })
  content: string;

  @Field({ type: 'string', required: true })
  authorId: string;

  @Field({ type: 'array', required: false, default: [] })
  tags: string[];

  @Field({ type: 'boolean', required: false, default: false })
  isPublished: boolean;

  @Field({ type: 'number', required: false, default: 0 })
  viewCount: number;

  @Field({ type: 'number', required: false, default: 0 })
  likeCount: number;

  @Field({ type: 'number', required: false, default: () => Date.now() })
  createdAt: number;

  @Field({ type: 'number', required: false, default: () => Date.now() })
  updatedAt: number;

  @Field({ type: 'number', required: false })
  publishedAt?: number;

  // Relationships
  @BelongsTo(() => User, 'authorId')
  author: User;

  @HasMany(() => Comment, 'postId')
  comments: Comment[];

  @BeforeCreate()
  setupNewPost() {
    this.createdAt = Date.now();
    this.updatedAt = Date.now();
    this.viewCount = 0;
    this.likeCount = 0;
  }

  @BeforeUpdate()
  updateTimestamp() {
    this.updatedAt = Date.now();
  }

  // Helper methods
  async publish(): Promise<void> {
    this.isPublished = true;
    this.publishedAt = Date.now();
    await this.save();
  }

  async unpublish(): Promise<void> {
    this.isPublished = false;
    this.publishedAt = undefined;
    await this.save();
  }

  async incrementViews(): Promise<void> {
    this.viewCount += 1;
    await this.save();
  }

  async like(): Promise<void> {
    this.likeCount += 1;
    await this.save();
  }

  async unlike(): Promise<void> {
    if (this.likeCount > 0) {
      this.likeCount -= 1;
      await this.save();
    }
  }

  addTag(tag: string): void {
    const normalizedTag = tag.toLowerCase().trim();
    if (normalizedTag && !this.tags.includes(normalizedTag)) {
      this.tags.push(normalizedTag);
    }
  }

  removeTag(tag: string): void {
    this.tags = this.tags.filter((t) => t !== tag.toLowerCase().trim());
  }

  getExcerpt(length: number = 150): string {
    if (this.content.length <= length) {
      return this.content;
    }
    return this.content.substring(0, length).trim() + '...';
  }

  getReadingTime(): number {
    const wordsPerMinute = 200;
    const wordCount = this.content.split(/\s+/).length;
    return Math.ceil(wordCount / wordsPerMinute);
  }
}
```

Create `src/models/Comment.ts`:

```typescript
import { BaseModel, Model, Field, BelongsTo, BeforeCreate } from 'debros-framework';
import { User } from './User';
import { Post } from './Post';

@Model({
  scope: 'user',
  type: 'docstore',
  sharding: {
    strategy: 'user',
    count: 2,
    key: 'authorId',
  },
})
export class Comment extends BaseModel {
  @Field({ type: 'string', required: true, minLength: 1, maxLength: 1000 })
  content: string;

  @Field({ type: 'string', required: true })
  postId: string;

  @Field({ type: 'string', required: true })
  authorId: string;

  @Field({ type: 'boolean', required: false, default: true })
  isApproved: boolean;

  @Field({ type: 'number', required: false, default: 0 })
  likeCount: number;

  @Field({ type: 'number', required: false, default: () => Date.now() })
  createdAt: number;

  // Relationships
  @BelongsTo(() => Post, 'postId')
  post: Post;

  @BelongsTo(() => User, 'authorId')
  author: User;

  @BeforeCreate()
  setupNewComment() {
    this.createdAt = Date.now();
    this.isApproved = true; // Auto-approve for now
  }

  // Helper methods
  async approve(): Promise<void> {
    this.isApproved = true;
    await this.save();
  }

  async reject(): Promise<void> {
    this.isApproved = false;
    await this.save();
  }

  async like(): Promise<void> {
    this.likeCount += 1;
    await this.save();
  }

  async unlike(): Promise<void> {
    if (this.likeCount > 0) {
      this.likeCount -= 1;
      await this.save();
    }
  }
}
```

Create `src/models/index.ts`:

```typescript
export { User } from './User';
export { Post } from './Post';
export { Comment } from './Comment';
```

### 2. Application Service

Create `src/BlogService.ts`:

```typescript
import { DebrosFramework } from 'debros-framework';
import { User, Post, Comment } from './models';

export class BlogService {
  private framework: DebrosFramework;

  constructor(framework: DebrosFramework) {
    this.framework = framework;
  }

  // User operations
  async createUser(userData: {
    username: string;
    email: string;
    bio?: string;
    avatarUrl?: string;
  }): Promise<User> {
    return await User.create(userData);
  }

  async getUserByUsername(username: string): Promise<User | null> {
    return await User.query().where('username', username).findOne();
  }

  async getUserById(id: string): Promise<User | null> {
    return await User.findById(id);
  }

  async updateUserProfile(
    userId: string,
    updates: {
      bio?: string;
      avatarUrl?: string;
    },
  ): Promise<User> {
    const user = await User.findById(userId);
    if (!user) {
      throw new Error('User not found');
    }

    if (updates.bio !== undefined) user.bio = updates.bio;
    if (updates.avatarUrl !== undefined) user.avatarUrl = updates.avatarUrl;

    await user.save();
    return user;
  }

  // Post operations
  async createPost(
    authorId: string,
    postData: {
      title: string;
      content: string;
      tags?: string[];
    },
  ): Promise<Post> {
    const post = await Post.create({
      title: postData.title,
      content: postData.content,
      authorId,
      tags: postData.tags || [],
    });

    return post;
  }

  async getPostById(id: string): Promise<Post | null> {
    const post = await Post.findById(id);
    if (post) {
      await post.incrementViews(); // Track view
    }
    return post;
  }

  async getPostWithDetails(id: string): Promise<Post | null> {
    return await Post.query().where('id', id).with(['author', 'comments.author']).findOne();
  }

  async getPublishedPosts(
    options: {
      page?: number;
      limit?: number;
      tag?: string;
      authorId?: string;
    } = {},
  ): Promise<{ posts: Post[]; total: number }> {
    const { page = 1, limit = 10, tag, authorId } = options;

    let query = Post.query().where('isPublished', true).with(['author']);

    if (tag) {
      query = query.where('tags', 'includes', tag);
    }

    if (authorId) {
      query = query.where('authorId', authorId);
    }

    const result = await query.orderBy('publishedAt', 'desc').paginate(page, limit);

    return {
      posts: result.data,
      total: result.total,
    };
  }

  async getUserPosts(userId: string, includeUnpublished: boolean = false): Promise<Post[]> {
    let query = Post.query().where('authorId', userId);

    if (!includeUnpublished) {
      query = query.where('isPublished', true);
    }

    return await query.orderBy('createdAt', 'desc').find();
  }

  async updatePost(
    postId: string,
    updates: {
      title?: string;
      content?: string;
      tags?: string[];
    },
  ): Promise<Post> {
    const post = await Post.findById(postId);
    if (!post) {
      throw new Error('Post not found');
    }

    if (updates.title !== undefined) post.title = updates.title;
    if (updates.content !== undefined) post.content = updates.content;
    if (updates.tags !== undefined) post.tags = updates.tags;

    await post.save();
    return post;
  }

  async deletePost(postId: string): Promise<void> {
    const post = await Post.findById(postId);
    if (!post) {
      throw new Error('Post not found');
    }

    // Delete associated comments first
    const comments = await Comment.query().where('postId', postId).find();

    for (const comment of comments) {
      await comment.delete();
    }

    await post.delete();
  }

  // Comment operations
  async createComment(authorId: string, postId: string, content: string): Promise<Comment> {
    const comment = await Comment.create({
      content,
      postId,
      authorId,
    });

    return comment;
  }

  async getPostComments(postId: string): Promise<Comment[]> {
    return await Comment.query()
      .where('postId', postId)
      .where('isApproved', true)
      .with(['author'])
      .orderBy('createdAt', 'asc')
      .find();
  }

  async deleteComment(commentId: string): Promise<void> {
    const comment = await Comment.findById(commentId);
    if (!comment) {
      throw new Error('Comment not found');
    }

    await comment.delete();
  }

  // Search and discovery
  async searchPosts(
    query: string,
    options: {
      limit?: number;
      tags?: string[];
    } = {},
  ): Promise<Post[]> {
    const { limit = 20, tags } = options;

    let searchQuery = Post.query()
      .where('isPublished', true)
      .where((q) => {
        q.where('title', 'like', `%${query}%`).orWhere('content', 'like', `%${query}%`);
      });

    if (tags && tags.length > 0) {
      searchQuery = searchQuery.where('tags', 'includes any', tags);
    }

    return await searchQuery
      .with(['author'])
      .orderBy('likeCount', 'desc')
      .orderBy('createdAt', 'desc')
      .limit(limit)
      .find();
  }

  async getPopularPosts(timeframe: 'day' | 'week' | 'month' | 'all' = 'week'): Promise<Post[]> {
    let sinceTime: number;

    switch (timeframe) {
      case 'day':
        sinceTime = Date.now() - 24 * 60 * 60 * 1000;
        break;
      case 'week':
        sinceTime = Date.now() - 7 * 24 * 60 * 60 * 1000;
        break;
      case 'month':
        sinceTime = Date.now() - 30 * 24 * 60 * 60 * 1000;
        break;
      default:
        sinceTime = 0;
    }

    let query = Post.query().where('isPublished', true);

    if (sinceTime > 0) {
      query = query.where('publishedAt', '>', sinceTime);
    }

    return await query
      .with(['author'])
      .orderBy('likeCount', 'desc')
      .orderBy('viewCount', 'desc')
      .limit(10)
      .find();
  }

  async getTags(): Promise<Array<{ tag: string; count: number }>> {
    // Get all published posts
    const posts = await Post.query().where('isPublished', true).select(['tags']).find();

    // Count tags
    const tagCounts = new Map<string, number>();

    posts.forEach((post) => {
      post.tags.forEach((tag) => {
        tagCounts.set(tag, (tagCounts.get(tag) || 0) + 1);
      });
    });

    // Convert to array and sort by count
    return Array.from(tagCounts.entries())
      .map(([tag, count]) => ({ tag, count }))
      .sort((a, b) => b.count - a.count);
  }

  // Analytics
  async getUserStats(userId: string): Promise<{
    postCount: number;
    totalViews: number;
    totalLikes: number;
    commentCount: number;
  }> {
    const [posts, comments] = await Promise.all([
      Post.query().where('authorId', userId).find(),
      Comment.query().where('authorId', userId).count(),
    ]);

    const totalViews = posts.reduce((sum, post) => sum + post.viewCount, 0);
    const totalLikes = posts.reduce((sum, post) => sum + post.likeCount, 0);

    return {
      postCount: posts.length,
      totalViews,
      totalLikes,
      commentCount: comments,
    };
  }

  async getSystemStats(): Promise<{
    userCount: number;
    postCount: number;
    commentCount: number;
    publishedPostCount: number;
  }> {
    const [userCount, postCount, commentCount, publishedPostCount] = await Promise.all([
      User.query().count(),
      Post.query().count(),
      Comment.query().count(),
      Post.query().where('isPublished', true).count(),
    ]);

    return {
      userCount,
      postCount,
      commentCount,
      publishedPostCount,
    };
  }
}
```

### 3. Main Application

Create `src/index.ts`:

```typescript
import { DebrosFramework } from 'debros-framework';
import { BlogService } from './BlogService';
import { User, Post, Comment } from './models';
import { setupServices } from './setup';

async function main() {
  try {
    console.log('🚀 Starting DebrosFramework Blog Application...');

    // Initialize services
    const { orbitDBService, ipfsService } = await setupServices();

    // Initialize framework
    const framework = new DebrosFramework();
    await framework.initialize(orbitDBService, ipfsService);

    console.log('✅ Framework initialized successfully');

    // Create blog service
    const blogService = new BlogService(framework);

    // Demo: Create sample data
    await createSampleData(blogService);

    // Demo: Query data
    await demonstrateQueries(blogService);

    // Keep running for demo
    console.log('📝 Blog application is running...');
    console.log('Press Ctrl+C to stop');

    // Graceful shutdown
    process.on('SIGINT', async () => {
      console.log('\n🛑 Shutting down...');
      await framework.stop();
      process.exit(0);
    });
  } catch (error) {
    console.error('❌ Application failed to start:', error);
    process.exit(1);
  }
}

async function createSampleData(blogService: BlogService) {
  console.log('\n📝 Creating sample data...');

  // Create users
  const alice = await blogService.createUser({
    username: 'alice',
    email: 'alice@example.com',
    bio: 'Tech enthusiast and blogger',
    avatarUrl: 'https://example.com/avatars/alice.jpg',
  });

  const bob = await blogService.createUser({
    username: 'bob',
    email: 'bob@example.com',
    bio: 'Developer and writer',
  });

  console.log(`✅ Created users: ${alice.username}, ${bob.username}`);

  // Create posts
  const post1 = await blogService.createPost(alice.id, {
    title: 'Getting Started with DebrosFramework',
    content: 'DebrosFramework makes it easy to build decentralized applications...',
    tags: ['debros', 'tutorial', 'decentralized'],
  });

  const post2 = await blogService.createPost(bob.id, {
    title: 'Building Scalable dApps',
    content: 'Scalability is crucial for decentralized applications...',
    tags: ['scaling', 'dapps', 'blockchain'],
  });

  // Publish posts
  await post1.publish();
  await post2.publish();

  console.log(`✅ Created and published posts: "${post1.title}", "${post2.title}"`);

  // Create comments
  const comment1 = await blogService.createComment(
    bob.id,
    post1.id,
    'Great introduction to DebrosFramework!',
  );

  const comment2 = await blogService.createComment(
    alice.id,
    post2.id,
    'Very insightful article about scaling.',
  );

  console.log(`✅ Created ${[comment1, comment2].length} comments`);

  // Add some interactions
  await post1.like();
  await post1.like();
  await post2.like();
  await comment1.like();

  console.log('✅ Added sample interactions (likes)');
}

async function demonstrateQueries(blogService: BlogService) {
  console.log('\n🔍 Demonstrating queries...');

  // Get all published posts
  const { posts, total } = await blogService.getPublishedPosts({ limit: 10 });
  console.log(`📚 Found ${total} published posts:`);
  posts.forEach((post) => {
    console.log(
      `  - "${post.title}" by ${post.author.username} (${post.likeCount} likes, ${post.viewCount} views)`,
    );
  });

  // Search posts
  const searchResults = await blogService.searchPosts('DebrosFramework');
  console.log(`\n🔍 Search results for "DebrosFramework": ${searchResults.length} posts`);
  searchResults.forEach((post) => {
    console.log(`  - "${post.title}" by ${post.author.username}`);
  });

  // Get popular posts
  const popularPosts = await blogService.getPopularPosts('week');
  console.log(`\n⭐ Popular posts this week: ${popularPosts.length}`);
  popularPosts.forEach((post) => {
    console.log(`  - "${post.title}" (${post.likeCount} likes)`);
  });

  // Get tags
  const tags = await blogService.getTags();
  console.log(`\n🏷️  Popular tags:`);
  tags.slice(0, 5).forEach(({ tag, count }) => {
    console.log(`  - ${tag}: ${count} posts`);
  });

  // Get user stats
  const users = await User.query().find();
  for (const user of users) {
    const stats = await blogService.getUserStats(user.id);
    console.log(`\n📊 Stats for ${user.username}:`);
    console.log(`  - Posts: ${stats.postCount}`);
    console.log(`  - Total views: ${stats.totalViews}`);
    console.log(`  - Total likes: ${stats.totalLikes}`);
    console.log(`  - Comments: ${stats.commentCount}`);
  }

  // System stats
  const systemStats = await blogService.getSystemStats();
  console.log(`\n🌐 System stats:`);
  console.log(`  - Users: ${systemStats.userCount}`);
  console.log(`  - Posts: ${systemStats.postCount} (${systemStats.publishedPostCount} published)`);
  console.log(`  - Comments: ${systemStats.commentCount}`);
}

// Run the application
main().catch(console.error);
```

### 4. Service Setup

Create `src/setup.ts`:

```typescript
import { createHelia } from '@helia/helia';
import { createOrbitDB } from '@orbitdb/core';

export async function setupServices() {
  console.log('🔧 Setting up IPFS and OrbitDB services...');

  // Create IPFS instance
  const ipfs = await createHelia({
    // Configure as needed for your environment
  });

  // Create OrbitDB instance
  const orbitdb = await createOrbitDB({ ipfs });

  // Wrap services for DebrosFramework
  const ipfsService = {
    async init() {
      /* Already initialized */
    },
    getHelia: () => ipfs,
    getLibp2pInstance: () => ipfs.libp2p,
    async stop() {
      await ipfs.stop();
    },
  };

  const orbitDBService = {
    async init() {
      /* Already initialized */
    },
    async openDB(name: string, type: string) {
      return await orbitdb.open(name, { type });
    },
    getOrbitDB: () => orbitdb,
    async stop() {
      await orbitdb.stop();
    },
  };

  console.log('✅ Services setup complete');

  return { ipfsService, orbitDBService };
}
```

### 5. Running the Application

Add to your `package.json`:

```json
{
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts"
  }
}
```

Run the application:

```bash
npm run dev
```

## Key Concepts Demonstrated

### 1. Model Definition

- Using decorators to define models and fields
- Implementing validation and transformation
- Setting up relationships between models
- Using hooks for lifecycle management

### 2. Database Scoping

- Global models for shared data (User)
- User-scoped models for private data (Post, Comment)
- Automatic sharding based on model configuration

### 3. Query Operations

- Basic CRUD operations
- Complex queries with filtering and sorting
- Relationship loading (eager and lazy)
- Pagination and search functionality

### 4. Business Logic

- Service layer for application logic
- Model methods for domain-specific operations
- Data validation and transformation
- Analytics and reporting

### 5. Error Handling

- Graceful error handling in service methods
- Validation error handling
- Application lifecycle management

This example provides a solid foundation for building more complex applications with DebrosFramework. You can extend it by adding features like user authentication, real-time notifications, file uploads, and more advanced search capabilities.
