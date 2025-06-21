import { BaseModel } from '../../../../src/framework/models/BaseModel';
import { Model, Field, HasMany, BelongsTo, HasOne, BeforeCreate, AfterCreate } from '../../../../src/framework/models/decorators';

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
    if (!this.slug && this.name) {
      this.slug = this.name
        .toLowerCase()
        .replace(/\s+/g, '-')
        .replace(/[^a-z0-9-]/g, '');
    }
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

// Export all models
export { User, UserProfile, Category, Post, Comment };

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