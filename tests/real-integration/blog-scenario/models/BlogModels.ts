import 'reflect-metadata';
import { BaseModel } from '../../../../src/framework/models/BaseModel';
import { Model, Field, HasMany, BelongsTo, HasOne, BeforeCreate, AfterCreate } from '../../../../src/framework/models/decorators';

// Force field registration by manually setting up field configurations
function setupFieldConfigurations() {
  // User Profile fields
  if (!UserProfile.fields) {
    (UserProfile as any).fields = new Map();
  }
  UserProfile.fields.set('userId', { type: 'string', required: true });
  UserProfile.fields.set('bio', { type: 'string', required: false });
  UserProfile.fields.set('location', { type: 'string', required: false });
  UserProfile.fields.set('website', { type: 'string', required: false });
  UserProfile.fields.set('socialLinks', { type: 'object', required: false });
  UserProfile.fields.set('interests', { type: 'array', required: false, default: [] });
  UserProfile.fields.set('createdAt', { type: 'number', required: false, default: () => Date.now() });
  UserProfile.fields.set('updatedAt', { type: 'number', required: false, default: () => Date.now() });

  // User fields
  if (!User.fields) {
    (User as any).fields = new Map();
  }
  User.fields.set('username', { type: 'string', required: true, unique: true });
  User.fields.set('email', { type: 'string', required: true, unique: true });
  User.fields.set('displayName', { type: 'string', required: false });
  User.fields.set('avatar', { type: 'string', required: false });
  User.fields.set('isActive', { type: 'boolean', required: false, default: true });
  User.fields.set('roles', { type: 'array', required: false, default: [] });
  User.fields.set('createdAt', { type: 'number', required: false });
  User.fields.set('lastLoginAt', { type: 'number', required: false });

  // Category fields
  if (!Category.fields) {
    (Category as any).fields = new Map();
  }
  Category.fields.set('name', { type: 'string', required: true, unique: true });
  Category.fields.set('slug', { type: 'string', required: true, unique: true });
  Category.fields.set('description', { type: 'string', required: false });
  Category.fields.set('color', { type: 'string', required: false });
  Category.fields.set('isActive', { type: 'boolean', required: false, default: true });
  Category.fields.set('createdAt', { type: 'number', required: false, default: () => Date.now() });

  // Post fields
  if (!Post.fields) {
    (Post as any).fields = new Map();
  }
  Post.fields.set('title', { type: 'string', required: true });
  Post.fields.set('slug', { type: 'string', required: true, unique: true });
  Post.fields.set('content', { type: 'string', required: true });
  Post.fields.set('excerpt', { type: 'string', required: false });
  Post.fields.set('authorId', { type: 'string', required: true });
  Post.fields.set('categoryId', { type: 'string', required: false });
  Post.fields.set('tags', { type: 'array', required: false, default: [] });
  Post.fields.set('status', { type: 'string', required: false, default: 'draft' });
  Post.fields.set('featuredImage', { type: 'string', required: false });
  Post.fields.set('isFeatured', { type: 'boolean', required: false, default: false });
  Post.fields.set('viewCount', { type: 'number', required: false, default: 0 });
  Post.fields.set('likeCount', { type: 'number', required: false, default: 0 });
  Post.fields.set('createdAt', { type: 'number', required: false });
  Post.fields.set('updatedAt', { type: 'number', required: false });
  Post.fields.set('publishedAt', { type: 'number', required: false });

  // Comment fields
  if (!Comment.fields) {
    (Comment as any).fields = new Map();
  }
  Comment.fields.set('content', { type: 'string', required: true });
  Comment.fields.set('postId', { type: 'string', required: true });
  Comment.fields.set('authorId', { type: 'string', required: true });
  Comment.fields.set('parentId', { type: 'string', required: false });
  Comment.fields.set('isApproved', { type: 'boolean', required: false, default: true });
  Comment.fields.set('likeCount', { type: 'number', required: false, default: 0 });
  Comment.fields.set('createdAt', { type: 'number', required: false });
  Comment.fields.set('updatedAt', { type: 'number', required: false });
}

// User Profile Model
@Model({
  scope: 'global',
  type: 'docstore'
})
export class UserProfile extends BaseModel {
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

  @Field({ type: 'number', required: false, default: () => Date.now() })
  createdAt: number;

  @Field({ type: 'number', required: false, default: () => Date.now() })
  updatedAt: number;

  @BelongsTo(() => User, 'userId')
  user: User;
}

// User Model
@Model({
  scope: 'global',
  type: 'docstore'
})
export class User extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  username: string;

  @Field({ type: 'string', required: true, unique: true })
  email: string;

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
  posts: Post[];

  @HasMany(() => Comment, 'authorId')
  comments: Comment[];

  @HasOne(() => UserProfile, 'userId')
  profile: UserProfile;

  @BeforeCreate()
  setTimestamps() {
    this.createdAt = Date.now();
  }

  // Helper methods
  async updateLastLogin(): Promise<void> {
    this.lastLoginAt = Date.now();
    await this.save();
  }

  toJSON() {
    const json = super.toJSON();
    // Don't expose sensitive data in API responses
    delete json.password;
    return json;
  }
}

// Category Model
@Model({
  scope: 'global',
  type: 'docstore'
})
export class Category extends BaseModel {
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

  @Field({ type: 'number', required: false, default: () => Date.now() })
  createdAt: number;

  @HasMany(() => Post, 'categoryId')
  posts: Post[];

  @BeforeCreate()
  generateSlug() {
    console.log(`[DEBUG] generateSlug called for category: ${this.name}`);
    console.log(`[DEBUG] Current slug: ${this.slug}`);
    if (!this.slug && this.name) {
      this.slug = this.name
        .toLowerCase()
        .replace(/\s+/g, '-')
        .replace(/[^a-z0-9-]/g, '')
        .replace(/--+/g, '-')
        .replace(/^-|-$/g, '');
      console.log(`[DEBUG] Generated slug: ${this.slug}`);
    }
    console.log(`[DEBUG] Final slug: ${this.slug}`);
  }
}

// Post Model
@Model({
  scope: 'user',
  type: 'docstore'
})
export class Post extends BaseModel {
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
  author: User;

  @BelongsTo(() => Category, 'categoryId')
  category: Category;

  @HasMany(() => Comment, 'postId')
  comments: Comment[];

  @BeforeCreate()
  setTimestamps() {
    const now = Date.now();
    this.createdAt = now;
    this.updatedAt = now;
    
    // Generate slug before validation if missing
    if (!this.slug && this.title) {
      this.slug = this.title
        .toLowerCase()
        .replace(/\s+/g, '-')
        .replace(/[^a-z0-9-]/g, '');
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
  async publish(): Promise<void> {
    this.status = 'published';
    this.publishedAt = Date.now();
    this.updatedAt = Date.now();
    await this.save();
  }

  async unpublish(): Promise<void> {
    this.status = 'draft';
    this.publishedAt = undefined;
    this.updatedAt = Date.now();
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

  async archive(): Promise<void> {
    this.status = 'archived';
    this.updatedAt = Date.now();
    await this.save();
  }
}

// Comment Model
@Model({
  scope: 'user',
  type: 'docstore'
})
export class Comment extends BaseModel {
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
  post: Post;

  @BelongsTo(() => User, 'authorId')
  author: User;

  @BelongsTo(() => Comment, 'parentId')
  parent?: Comment;

  @HasMany(() => Comment, 'parentId')
  replies: Comment[];

  @BeforeCreate()
  setTimestamps() {
    const now = Date.now();
    this.createdAt = now;
    this.updatedAt = now;
  }

  // Helper methods
  async approve(): Promise<void> {
    this.isApproved = true;
    this.updatedAt = Date.now();
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

// Type definitions for API requests
export interface CreateUserRequest {
  username: string;
  email: string;
  displayName?: string;
  avatar?: string;
  roles?: string[];
}

export interface CreateCategoryRequest {
  name: string;
  slug?: string;
  description?: string;
  color?: string;
}

export interface CreatePostRequest {
  title: string;
  content: string;
  excerpt?: string;
  authorId: string;
  categoryId?: string;
  tags?: string[];
  featuredImage?: string;
  status?: 'draft' | 'published';
}

export interface CreateCommentRequest {
  content: string;
  postId: string;
  authorId: string;
  parentId?: string;
}

export interface UpdatePostRequest {
  title?: string;
  content?: string;
  excerpt?: string;
  categoryId?: string;
  tags?: string[];
  featuredImage?: string;
  isFeatured?: boolean;
}

// Initialize field configurations after all models are defined
setupFieldConfigurations();

// Ensure static properties are set properly (for Docker environment)
UserProfile.modelName = 'UserProfile';
UserProfile.storeType = 'docstore';
UserProfile.scope = 'global';

User.modelName = 'User';
User.storeType = 'docstore';
User.scope = 'global';

Category.modelName = 'Category';
Category.storeType = 'docstore';
Category.scope = 'global';

Post.modelName = 'Post';
Post.storeType = 'docstore';
Post.scope = 'user';

Comment.modelName = 'Comment';
Comment.storeType = 'docstore';
Comment.scope = 'user';
