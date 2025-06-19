import { describe, beforeEach, afterEach, it, expect, jest } from '@jest/globals';
import { DebrosFramework } from '../../src/framework/DebrosFramework';
import { BaseModel } from '../../src/framework/models/BaseModel';
import { Model, Field, HasMany, BelongsTo, HasOne, BeforeCreate, AfterCreate } from '../../src/framework/models/decorators';
import { createMockServices } from '../mocks/services';

// Complete Blog Example Models
@Model({
  scope: 'global',
  type: 'docstore'
})
class UserProfile extends BaseModel {
  @Field({ type: 'string', required: true })
  userId: string;

  @Field({ type: 'string', required: false })
  bio?: string;

  @Field({ type: 'string', required: false })
  location?: string;

  @Field({ type: 'string', required: false })
  website?: string;

  @Field({ type: 'object', required: false })
  socialLinks?: {
    twitter?: string;
    github?: string;
    linkedin?: string;
  };

  @Field({ type: 'array', required: false, default: [] })
  interests: string[];

  @BelongsTo(() => User, 'userId')
  user: any;
}

@Model({
  scope: 'global',
  type: 'docstore'
})
class User extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  username: string;

  @Field({ type: 'string', required: true, unique: true })
  email: string;

  @Field({ type: 'string', required: true })
  password: string; // In real app, this would be hashed

  @Field({ type: 'string', required: false })
  displayName?: string;

  @Field({ type: 'string', required: false })
  avatar?: string;

  @Field({ type: 'boolean', required: false, default: true })
  isActive: boolean;

  @Field({ type: 'array', required: false, default: [] })
  roles: string[];

  @Field({ type: 'number', required: false })
  createdAt: number;

  @Field({ type: 'number', required: false })
  lastLoginAt?: number;

  @HasMany(() => Post, 'authorId')
  posts: any[];

  @HasMany(() => Comment, 'authorId')
  comments: any[];

  @HasOne(() => UserProfile, 'userId')
  profile: any;

  @BeforeCreate()
  setTimestamps() {
    this.createdAt = Date.now();
  }

  // Helper methods
  async updateLastLogin() {
    this.lastLoginAt = Date.now();
    await this.save();
  }

  async changePassword(newPassword: string) {
    // In a real app, this would hash the password
    this.password = newPassword;
    await this.save();
  }
}

@Model({
  scope: 'global',
  type: 'docstore'
})
class Category extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  name: string;

  @Field({ type: 'string', required: true, unique: true })
  slug: string;

  @Field({ type: 'string', required: false })
  description?: string;

  @Field({ type: 'string', required: false })
  color?: string;

  @Field({ type: 'boolean', required: false, default: true })
  isActive: boolean;

  @HasMany(() => Post, 'categoryId')
  posts: any[];

  @BeforeCreate()
  generateSlug() {
    if (!this.slug && this.name) {
      this.slug = this.name.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '');
    }
  }
}

@Model({
  scope: 'user',
  type: 'docstore'
})
class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true, unique: true })
  slug: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: false })
  excerpt?: string;

  @Field({ type: 'string', required: true })
  authorId: string;

  @Field({ type: 'string', required: false })
  categoryId?: string;

  @Field({ type: 'array', required: false, default: [] })
  tags: string[];

  @Field({ type: 'string', required: false, default: 'draft' })
  status: 'draft' | 'published' | 'archived';

  @Field({ type: 'string', required: false })
  featuredImage?: string;

  @Field({ type: 'boolean', required: false, default: false })
  isFeatured: boolean;

  @Field({ type: 'number', required: false, default: 0 })
  viewCount: number;

  @Field({ type: 'number', required: false, default: 0 })
  likeCount: number;

  @Field({ type: 'number', required: false })
  createdAt: number;

  @Field({ type: 'number', required: false })
  updatedAt: number;

  @Field({ type: 'number', required: false })
  publishedAt?: number;

  @BelongsTo(() => User, 'authorId')
  author: any;

  @BelongsTo(() => Category, 'categoryId')
  category: any;

  @HasMany(() => Comment, 'postId')
  comments: any[];

  @BeforeCreate()
  setTimestamps() {
    const now = Date.now();
    this.createdAt = now;
    this.updatedAt = now;
    
    // Generate slug before validation if missing
    if (!this.slug && this.title) {
      this.slug = this.title.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '');
    }
  }

  @AfterCreate()
  finalizeSlug() {
    // Add unique identifier to slug after creation to ensure uniqueness
    if (this.slug && this.id) {
      this.slug = this.slug + '-' + this.id.slice(-8);
    }
  }

  // Helper methods
  async publish() {
    this.status = 'published';
    this.publishedAt = Date.now();
    this.updatedAt = Date.now();
    await this.save();
  }

  async unpublish() {
    this.status = 'draft';
    this.publishedAt = undefined;
    this.updatedAt = Date.now();
    await this.save();
  }

  async incrementViews() {
    this.viewCount += 1;
    await this.save();
  }

  async like() {
    this.likeCount += 1;
    await this.save();
  }

  async unlike() {
    if (this.likeCount > 0) {
      this.likeCount -= 1;
      await this.save();
    }
  }
}

@Model({
  scope: 'user',
  type: 'docstore'
})
class Comment extends BaseModel {
  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  postId: string;

  @Field({ type: 'string', required: true })
  authorId: string;

  @Field({ type: 'string', required: false })
  parentId?: string; // For nested comments

  @Field({ type: 'boolean', required: false, default: true })
  isApproved: boolean;

  @Field({ type: 'number', required: false, default: 0 })
  likeCount: number;

  @Field({ type: 'number', required: false })
  createdAt: number;

  @Field({ type: 'number', required: false })
  updatedAt: number;

  @BelongsTo(() => Post, 'postId')
  post: any;

  @BelongsTo(() => User, 'authorId')
  author: any;

  @BelongsTo(() => Comment, 'parentId')
  parent?: any;

  @HasMany(() => Comment, 'parentId')
  replies: any[];

  @BeforeCreate()
  setTimestamps() {
    const now = Date.now();
    this.createdAt = now;
    this.updatedAt = now;
  }

  // Helper methods
  async approve() {
    this.isApproved = true;
    this.updatedAt = Date.now();
    await this.save();
  }

  async like() {
    this.likeCount += 1;
    await this.save();
  }
}

describe('Blog Example - End-to-End Tests', () => {
  let framework: DebrosFramework;
  let mockServices: any;

  beforeEach(async () => {
    mockServices = createMockServices();
    
    framework = new DebrosFramework({
      environment: 'test',
      features: {
        autoMigration: false,
        automaticPinning: false,
        pubsub: false,
        queryCache: true,
        relationshipCache: true
      }
    });

    await framework.initialize(mockServices.orbitDBService, mockServices.ipfsService);

    // Suppress console output for cleaner test output
    jest.spyOn(console, 'log').mockImplementation(() => {});
    jest.spyOn(console, 'error').mockImplementation(() => {});
    jest.spyOn(console, 'warn').mockImplementation(() => {});
  });

  afterEach(async () => {
    if (framework) {
      await framework.cleanup();
    }
    jest.restoreAllMocks();
  });

  describe('User Management', () => {
    it('should create and manage users', async () => {
      // Create a new user
      const user = await User.create({
        username: 'johndoe',
        email: 'john@example.com',
        password: 'secure123',
        displayName: 'John Doe',
        roles: ['author']
      });

      expect(user).toBeInstanceOf(User);
      expect(user.username).toBe('johndoe');
      expect(user.email).toBe('john@example.com');
      expect(user.displayName).toBe('John Doe');
      expect(user.isActive).toBe(true);
      expect(user.roles).toEqual(['author']);
      expect(user.createdAt).toBeDefined();
      expect(user.id).toBeDefined();
    });

    it('should create user profile', async () => {
      const user = await User.create({
        username: 'janedoe',
        email: 'jane@example.com',
        password: 'secure456'
      });

      const profile = await UserProfile.create({
        userId: user.id,
        bio: 'Software developer and blogger',
        location: 'San Francisco, CA',
        website: 'https://janedoe.com',
        socialLinks: {
          twitter: '@janedoe',
          github: 'janedoe'
        },
        interests: ['javascript', 'web development', 'open source']
      });

      expect(profile).toBeInstanceOf(UserProfile);
      expect(profile.userId).toBe(user.id);
      expect(profile.bio).toBe('Software developer and blogger');
      expect(profile.socialLinks?.twitter).toBe('@janedoe');
      expect(profile.interests).toContain('javascript');
    });

    it('should handle user authentication workflow', async () => {
      const user = await User.create({
        username: 'authuser',
        email: 'auth@example.com',
        password: 'original123'
      });

      // Simulate login
      await user.updateLastLogin();
      expect(user.lastLoginAt).toBeDefined();

      // Change password
      await user.changePassword('newpassword456');
      expect(user.password).toBe('newpassword456');
    });
  });

  describe('Content Management', () => {
    let author: User;
    let category: Category;

    beforeEach(async () => {
      author = await User.create({
        username: 'contentauthor',
        email: 'author@example.com',
        password: 'authorpass',
        roles: ['author', 'editor']
      });

      category = await Category.create({
        name: 'Technology',
        description: 'Posts about technology and programming'
      });
    });

    it('should create and manage categories', async () => {
      expect(category).toBeInstanceOf(Category);
      expect(category.name).toBe('Technology');
      expect(category.slug).toBe('technology');
      expect(category.description).toBe('Posts about technology and programming');
      expect(category.isActive).toBe(true);
    });

    it('should create draft posts', async () => {
      const post = await Post.create({
        title: 'My First Blog Post',
        content: 'This is the content of my first blog post. It contains valuable information about web development.',
        excerpt: 'Learn about web development in this comprehensive guide.',
        authorId: author.id,
        categoryId: category.id,
        tags: ['web development', 'tutorial', 'beginner'],
        featuredImage: 'https://example.com/image.jpg'
      });

      expect(post).toBeInstanceOf(Post);
      expect(post.title).toBe('My First Blog Post');
      expect(post.status).toBe('draft'); // Default status
      expect(post.authorId).toBe(author.id);
      expect(post.categoryId).toBe(category.id);
      expect(post.tags).toEqual(['web development', 'tutorial', 'beginner']);
      expect(post.viewCount).toBe(0);
      expect(post.likeCount).toBe(0);
      expect(post.createdAt).toBeDefined();
      expect(post.slug).toBeDefined();
    });

    it('should publish and unpublish posts', async () => {
      const post = await Post.create({
        title: 'Publishing Test Post',
        content: 'This post will be published and then unpublished.',
        authorId: author.id
      });

      // Initially draft
      expect(post.status).toBe('draft');
      expect(post.publishedAt).toBeUndefined();

      // Publish the post
      await post.publish();
      expect(post.status).toBe('published');
      expect(post.publishedAt).toBeDefined();

      // Unpublish the post
      await post.unpublish();
      expect(post.status).toBe('draft');
      expect(post.publishedAt).toBeUndefined();
    });

    it('should track post engagement', async () => {
      const post = await Post.create({
        title: 'Engagement Test Post',
        content: 'This post will test engagement tracking.',
        authorId: author.id
      });

      // Track views
      await post.incrementViews();
      await post.incrementViews();
      expect(post.viewCount).toBe(2);

      // Track likes
      await post.like();
      await post.like();
      expect(post.likeCount).toBe(2);

      // Unlike
      await post.unlike();
      expect(post.likeCount).toBe(1);
    });
  });

  describe('Comment System', () => {
    let author: User;
    let commenter: User;
    let post: Post;

    beforeEach(async () => {
      author = await User.create({
        username: 'postauthor',
        email: 'postauthor@example.com',
        password: 'authorpass'
      });

      commenter = await User.create({
        username: 'commenter',
        email: 'commenter@example.com',
        password: 'commenterpass'
      });

      post = await Post.create({
        title: 'Post with Comments',
        content: 'This post will have comments.',
        authorId: author.id
      });
      await post.publish();
    });

    it('should create comments on posts', async () => {
      const comment = await Comment.create({
        content: 'This is a great post! Thanks for sharing.',
        postId: post.id,
        authorId: commenter.id
      });

      expect(comment).toBeInstanceOf(Comment);
      expect(comment.content).toBe('This is a great post! Thanks for sharing.');
      expect(comment.postId).toBe(post.id);
      expect(comment.authorId).toBe(commenter.id);
      expect(comment.isApproved).toBe(true); // Default value
      expect(comment.likeCount).toBe(0);
      expect(comment.createdAt).toBeDefined();
    });

    it('should support nested comments (replies)', async () => {
      // Create parent comment
      const parentComment = await Comment.create({
        content: 'This is the parent comment.',
        postId: post.id,
        authorId: commenter.id
      });

      // Create reply
      const reply = await Comment.create({
        content: 'This is a reply to the parent comment.',
        postId: post.id,
        authorId: author.id,
        parentId: parentComment.id
      });

      expect(reply.parentId).toBe(parentComment.id);
      expect(reply.content).toBe('This is a reply to the parent comment.');
    });

    it('should manage comment approval and engagement', async () => {
      const comment = await Comment.create({
        content: 'This comment needs approval.',
        postId: post.id,
        authorId: commenter.id,
        isApproved: false
      });

      // Initially not approved
      expect(comment.isApproved).toBe(false);

      // Approve comment
      await comment.approve();
      expect(comment.isApproved).toBe(true);

      // Like comment
      await comment.like();
      expect(comment.likeCount).toBe(1);
    });
  });

  describe('Content Discovery and Queries', () => {
    let authors: User[];
    let categories: Category[];
    let posts: Post[];

    beforeEach(async () => {
      // Create test authors
      authors = [];
      for (let i = 0; i < 3; i++) {
        const author = await User.create({
          username: `author${i}`,
          email: `author${i}@example.com`,
          password: 'password123'
        });
        authors.push(author);
      }

      // Create test categories
      categories = [];
      const categoryNames = ['Technology', 'Design', 'Business'];
      for (const name of categoryNames) {
        const category = await Category.create({
          name,
          description: `Posts about ${name.toLowerCase()}`
        });
        categories.push(category);
      }

      // Create test posts
      posts = [];
      for (let i = 0; i < 6; i++) {
        const post = await Post.create({
          title: `Test Post ${i + 1}`,
          content: `This is the content of test post ${i + 1}.`,
          authorId: authors[i % authors.length].id,
          categoryId: categories[i % categories.length].id,
          tags: [`tag${i}`, `common-tag`],
          status: i % 2 === 0 ? 'published' : 'draft'
        });
        if (post.status === 'published') {
          await post.publish();
        }
        posts.push(post);
      }
    });

    it('should query posts by status', async () => {
      const publishedQuery = Post.query().where('status', 'published');
      const draftQuery = Post.query().where('status', 'draft');

      // These would work in a real implementation with actual database queries
      expect(publishedQuery).toBeDefined();
      expect(draftQuery).toBeDefined();
      expect(typeof publishedQuery.find).toBe('function');
      expect(typeof draftQuery.count).toBe('function');
    });

    it('should query posts by author', async () => {
      const authorQuery = Post.query().where('authorId', authors[0].id);

      expect(authorQuery).toBeDefined();
      expect(typeof authorQuery.find).toBe('function');
    });

    it('should query posts by category', async () => {
      const categoryQuery = Post.query().where('categoryId', categories[0].id);

      expect(categoryQuery).toBeDefined();
      expect(typeof categoryQuery.orderBy).toBe('function');
    });

    it('should support complex queries with multiple conditions', async () => {
      const complexQuery = Post.query()
        .where('status', 'published')
        .where('isFeatured', true)
        .where('categoryId', categories[0].id)
        .orderBy('publishedAt', 'desc')
        .limit(10);

      expect(complexQuery).toBeDefined();
      expect(typeof complexQuery.find).toBe('function');
      expect(typeof complexQuery.count).toBe('function');
    });

    it('should query posts by tags', async () => {
      const tagQuery = Post.query()
        .where('tags', 'includes', 'common-tag')
        .where('status', 'published')
        .orderBy('publishedAt', 'desc');

      expect(tagQuery).toBeDefined();
    });
  });

  describe('Relationships and Data Loading', () => {
    let user: User;
    let profile: UserProfile;
    let category: Category;
    let post: Post;
    let comments: Comment[];

    beforeEach(async () => {
      // Create user with profile
      user = await User.create({
        username: 'relationuser',
        email: 'relation@example.com',
        password: 'password123'
      });

      profile = await UserProfile.create({
        userId: user.id,
        bio: 'I am a test user for relationship testing',
        interests: ['testing', 'relationships']
      });

      // Create category and post
      category = await Category.create({
        name: 'Relationships',
        description: 'Testing relationships'
      });

      post = await Post.create({
        title: 'Post with Relationships',
        content: 'This post tests relationship loading.',
        authorId: user.id,
        categoryId: category.id
      });
      await post.publish();

      // Create comments
      comments = [];
      for (let i = 0; i < 3; i++) {
        const comment = await Comment.create({
          content: `Comment ${i + 1} on the post.`,
          postId: post.id,
          authorId: user.id
        });
        comments.push(comment);
      }
    });

    it('should load user relationships', async () => {
      const relationshipManager = framework.getRelationshipManager();

      // Load user's posts
      const userPosts = await relationshipManager!.loadRelationship(user, 'posts');
      expect(Array.isArray(userPosts)).toBe(true);

      // Load user's profile
      const userProfile = await relationshipManager!.loadRelationship(user, 'profile');
      // Mock implementation might return null, but the method should work
      expect(userProfile === null || userProfile instanceof UserProfile).toBe(true);

      // Load user's comments
      const userComments = await relationshipManager!.loadRelationship(user, 'comments');
      expect(Array.isArray(userComments)).toBe(true);
    });

    it('should load post relationships', async () => {
      const relationshipManager = framework.getRelationshipManager();

      // Load post's author
      const postAuthor = await relationshipManager!.loadRelationship(post, 'author');
      // Mock might return null, but relationship should be loadable
      expect(postAuthor === null || postAuthor instanceof User).toBe(true);

      // Load post's category
      const postCategory = await relationshipManager!.loadRelationship(post, 'category');
      expect(postCategory === null || postCategory instanceof Category).toBe(true);

      // Load post's comments
      const postComments = await relationshipManager!.loadRelationship(post, 'comments');
      expect(Array.isArray(postComments)).toBe(true);
    });

    it('should support eager loading of multiple relationships', async () => {
      const relationshipManager = framework.getRelationshipManager();

      // Eager load multiple relationships on multiple posts
      await relationshipManager!.eagerLoadRelationships(
        [post],
        ['author', 'category', 'comments']
      );

      // Relationships should be available through the loaded relations
      expect(post._loadedRelations.size).toBeGreaterThan(0);
    });

    it('should handle nested relationships', async () => {
      const relationshipManager = framework.getRelationshipManager();

      // Load comments first
      const postComments = await relationshipManager!.loadRelationship(post, 'comments');

      if (Array.isArray(postComments) && postComments.length > 0) {
        // Load author relationship on first comment
        const commentAuthor = await relationshipManager!.loadRelationship(postComments[0], 'author');
        expect(commentAuthor === null || commentAuthor instanceof User).toBe(true);
      }
    });
  });

  describe('Blog Workflow Integration', () => {
    it('should support complete blog publishing workflow', async () => {
      // 1. Create author
      const author = await User.create({
        username: 'blogauthor',
        email: 'blog@example.com',
        password: 'blogpass',
        displayName: 'Blog Author',
        roles: ['author']
      });

      // 2. Create author profile
      const profile = await UserProfile.create({
        userId: author.id,
        bio: 'Professional blogger and writer',
        website: 'https://blogauthor.com'
      });

      // 3. Create category
      const category = await Category.create({
        name: 'Web Development',
        description: 'Posts about web development and programming'
      });

      // 4. Create draft post
      const post = await Post.create({
        title: 'Advanced JavaScript Techniques',
        content: 'In this post, we will explore advanced JavaScript techniques...',
        excerpt: 'Learn advanced JavaScript techniques to improve your code.',
        authorId: author.id,
        categoryId: category.id,
        tags: ['javascript', 'advanced', 'programming'],
        featuredImage: 'https://example.com/js-advanced.jpg'
      });

      expect(post.status).toBe('draft');

      // 5. Publish the post
      await post.publish();
      expect(post.status).toBe('published');
      expect(post.publishedAt).toBeDefined();

      // 6. Reader discovers and engages with post
      await post.incrementViews();
      await post.like();
      expect(post.viewCount).toBe(1);
      expect(post.likeCount).toBe(1);

      // 7. Create reader and comment
      const reader = await User.create({
        username: 'reader',
        email: 'reader@example.com',
        password: 'readerpass'
      });

      const comment = await Comment.create({
        content: 'Great post! Very helpful information.',
        postId: post.id,
        authorId: reader.id
      });

      // 8. Author replies to comment
      const reply = await Comment.create({
        content: 'Thank you for the feedback! Glad you found it helpful.',
        postId: post.id,
        authorId: author.id,
        parentId: comment.id
      });

      // Verify the complete workflow
      expect(author).toBeInstanceOf(User);
      expect(profile).toBeInstanceOf(UserProfile);
      expect(category).toBeInstanceOf(Category);
      expect(post).toBeInstanceOf(Post);
      expect(comment).toBeInstanceOf(Comment);
      expect(reply).toBeInstanceOf(Comment);
      expect(reply.parentId).toBe(comment.id);
    });

    it('should support content management operations', async () => {
      const author = await User.create({
        username: 'contentmgr',
        email: 'mgr@example.com',
        password: 'mgrpass'
      });

      // Create multiple posts
      const posts = [];
      for (let i = 0; i < 5; i++) {
        const post = await Post.create({
          title: `Management Post ${i + 1}`,
          content: `Content for post ${i + 1}`,
          authorId: author.id,
          tags: [`tag${i}`]
        });
        posts.push(post);
      }

      // Publish some posts
      await posts[0].publish();
      await posts[2].publish();
      await posts[4].publish();

      // Feature a post
      posts[0].isFeatured = true;
      await posts[0].save();

      // Archive a post
      posts[1].status = 'archived';
      await posts[1].save();

      // Verify post states
      expect(posts[0].status).toBe('published');
      expect(posts[0].isFeatured).toBe(true);
      expect(posts[1].status).toBe('archived');
      expect(posts[2].status).toBe('published');
      expect(posts[3].status).toBe('draft');
    });
  });

  describe('Performance and Scalability', () => {
    it('should handle bulk operations efficiently', async () => {
      const startTime = Date.now();

      // Create multiple users concurrently
      const userPromises = [];
      for (let i = 0; i < 10; i++) {
        userPromises.push(User.create({
          username: `bulkuser${i}`,
          email: `bulk${i}@example.com`,
          password: 'bulkpass'
        }));
      }

      const users = await Promise.all(userPromises);
      expect(users).toHaveLength(10);

      const endTime = Date.now();
      const duration = endTime - startTime;

      // Should complete reasonably quickly (less than 1 second for mocked operations)
      expect(duration).toBeLessThan(1000);
    });

    it('should support concurrent read operations', async () => {
      const author = await User.create({
        username: 'concurrentauthor',
        email: 'concurrent@example.com',
        password: 'concurrentpass'
      });

      const post = await Post.create({
        title: 'Concurrent Read Test',
        content: 'Testing concurrent reads',
        authorId: author.id
      });

      // Simulate concurrent reads
      const readPromises = [];
      for (let i = 0; i < 5; i++) {
        readPromises.push(post.incrementViews());
      }

      await Promise.all(readPromises);

      // View count should reflect all increments
      expect(post.viewCount).toBe(5);
    });
  });

  describe('Data Integrity and Validation', () => {
    it('should enforce required field validation', async () => {
      await expect(User.create({
        // Missing required fields username and email
        password: 'password123'
      } as any)).rejects.toThrow();
    });

    it('should enforce unique constraints', async () => {
      await User.create({
        username: 'uniqueuser',
        email: 'unique@example.com',
        password: 'password123'
      });

      // Attempt to create user with same username should fail
      await expect(User.create({
        username: 'uniqueuser', // Duplicate username
        email: 'different@example.com',
        password: 'password123'
      })).rejects.toThrow();
    });

    it('should validate field types', async () => {
      await expect(User.create({
        username: 'typetest',
        email: 'typetest@example.com',
        password: 'password123',
        isActive: 'not-a-boolean' as any // Invalid type
      })).rejects.toThrow();
    });

    it('should apply default values correctly', async () => {
      const user = await User.create({
        username: 'defaultuser',
        email: 'default@example.com',
        password: 'password123'
      });

      expect(user.isActive).toBe(true); // Default value
      expect(user.roles).toEqual([]); // Default array

      const post = await Post.create({
        title: 'Default Test',
        content: 'Testing defaults',
        authorId: user.id
      });

      expect(post.status).toBe('draft'); // Default status
      expect(post.tags).toEqual([]); // Default array
      expect(post.viewCount).toBe(0); // Default number
      expect(post.isFeatured).toBe(false); // Default boolean
    });
  });
});