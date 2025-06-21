import { CreateUserRequest, CreateCategoryRequest, CreatePostRequest, CreateCommentRequest, UpdatePostRequest } from './BlogModels';

export class ValidationError extends Error {
  constructor(message: string, public field?: string) {
    super(message);
    this.name = 'ValidationError';
  }
}

export class BlogValidation {
  static validateUser(data: CreateUserRequest): void {
    if (!data.username || data.username.length < 3 || data.username.length > 30) {
      throw new ValidationError('Username must be between 3 and 30 characters', 'username');
    }

    if (!/^[a-zA-Z0-9_]+$/.test(data.username)) {
      throw new ValidationError('Username can only contain letters, numbers, and underscores', 'username');
    }

    if (!data.email || !this.isValidEmail(data.email)) {
      throw new ValidationError('Valid email is required', 'email');
    }

    if (data.displayName && data.displayName.length > 100) {
      throw new ValidationError('Display name cannot exceed 100 characters', 'displayName');
    }

    if (data.avatar && !this.isValidUrl(data.avatar)) {
      throw new ValidationError('Avatar must be a valid URL', 'avatar');
    }

    if (data.roles && !Array.isArray(data.roles)) {
      throw new ValidationError('Roles must be an array', 'roles');
    }
  }

  static validateCategory(data: CreateCategoryRequest): void {
    if (!data.name || data.name.length < 2 || data.name.length > 50) {
      throw new ValidationError('Category name must be between 2 and 50 characters', 'name');
    }

    if (data.description && data.description.length > 500) {
      throw new ValidationError('Description cannot exceed 500 characters', 'description');
    }

    if (data.color && !/^#[0-9A-Fa-f]{6}$/.test(data.color)) {
      throw new ValidationError('Color must be a valid hex color code', 'color');
    }
  }

  static validatePost(data: CreatePostRequest): void {
    if (!data.title || data.title.length < 3 || data.title.length > 200) {
      throw new ValidationError('Title must be between 3 and 200 characters', 'title');
    }

    if (!data.content || data.content.length < 10) {
      throw new ValidationError('Content must be at least 10 characters long', 'content');
    }

    if (data.content.length > 50000) {
      throw new ValidationError('Content cannot exceed 50,000 characters', 'content');
    }

    if (!data.authorId) {
      throw new ValidationError('Author ID is required', 'authorId');
    }

    if (data.excerpt && data.excerpt.length > 300) {
      throw new ValidationError('Excerpt cannot exceed 300 characters', 'excerpt');
    }

    if (data.tags && !Array.isArray(data.tags)) {
      throw new ValidationError('Tags must be an array', 'tags');
    }

    if (data.tags && data.tags.length > 10) {
      throw new ValidationError('Cannot have more than 10 tags', 'tags');
    }

    if (data.tags) {
      for (const tag of data.tags) {
        if (typeof tag !== 'string' || tag.length > 30) {
          throw new ValidationError('Each tag must be a string with max 30 characters', 'tags');
        }
      }
    }

    if (data.featuredImage && !this.isValidUrl(data.featuredImage)) {
      throw new ValidationError('Featured image must be a valid URL', 'featuredImage');
    }

    if (data.status && !['draft', 'published'].includes(data.status)) {
      throw new ValidationError('Status must be either "draft" or "published"', 'status');
    }
  }

  static validatePostUpdate(data: UpdatePostRequest): void {
    if (data.title && (data.title.length < 3 || data.title.length > 200)) {
      throw new ValidationError('Title must be between 3 and 200 characters', 'title');
    }

    if (data.content && (data.content.length < 10 || data.content.length > 50000)) {
      throw new ValidationError('Content must be between 10 and 50,000 characters', 'content');
    }

    if (data.excerpt && data.excerpt.length > 300) {
      throw new ValidationError('Excerpt cannot exceed 300 characters', 'excerpt');
    }

    if (data.tags && !Array.isArray(data.tags)) {
      throw new ValidationError('Tags must be an array', 'tags');
    }

    if (data.tags && data.tags.length > 10) {
      throw new ValidationError('Cannot have more than 10 tags', 'tags');
    }

    if (data.tags) {
      for (const tag of data.tags) {
        if (typeof tag !== 'string' || tag.length > 30) {
          throw new ValidationError('Each tag must be a string with max 30 characters', 'tags');
        }
      }
    }

    if (data.featuredImage && !this.isValidUrl(data.featuredImage)) {
      throw new ValidationError('Featured image must be a valid URL', 'featuredImage');
    }

    if (data.isFeatured !== undefined && typeof data.isFeatured !== 'boolean') {
      throw new ValidationError('isFeatured must be a boolean', 'isFeatured');
    }
  }

  static validateComment(data: CreateCommentRequest): void {
    if (!data.content || data.content.length < 1 || data.content.length > 2000) {
      throw new ValidationError('Comment must be between 1 and 2000 characters', 'content');
    }

    if (!data.postId) {
      throw new ValidationError('Post ID is required', 'postId');
    }

    if (!data.authorId) {
      throw new ValidationError('Author ID is required', 'authorId');
    }

    // parentId is optional, but if provided should be a string
    if (data.parentId !== undefined && typeof data.parentId !== 'string') {
      throw new ValidationError('Parent ID must be a string', 'parentId');
    }
  }

  private static isValidEmail(email: string): boolean {
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return emailRegex.test(email);
  }

  private static isValidUrl(url: string): boolean {
    try {
      new URL(url);
      return true;
    } catch {
      return false;
    }
  }

  // Sanitization helpers
  static sanitizeString(input: string): string {
    return input.trim().replace(/[<>]/g, '');
  }

  static sanitizeArray(input: string[]): string[] {
    return input.map(item => this.sanitizeString(item)).filter(item => item.length > 0);
  }

  static sanitizeUserInput(data: CreateUserRequest): CreateUserRequest {
    return {
      username: this.sanitizeString(data.username),
      email: this.sanitizeString(data.email.toLowerCase()),
      displayName: data.displayName ? this.sanitizeString(data.displayName) : undefined,
      avatar: data.avatar ? this.sanitizeString(data.avatar) : undefined,
      roles: data.roles ? this.sanitizeArray(data.roles) : undefined
    };
  }

  static sanitizeCategoryInput(data: CreateCategoryRequest): CreateCategoryRequest {
    return {
      name: this.sanitizeString(data.name),
      description: data.description ? this.sanitizeString(data.description) : undefined,
      color: data.color ? this.sanitizeString(data.color) : undefined
    };
  }

  static sanitizePostInput(data: CreatePostRequest): CreatePostRequest {
    return {
      title: this.sanitizeString(data.title),
      content: data.content.trim(), // Don't sanitize content too aggressively
      excerpt: data.excerpt ? this.sanitizeString(data.excerpt) : undefined,
      authorId: this.sanitizeString(data.authorId),
      categoryId: data.categoryId ? this.sanitizeString(data.categoryId) : undefined,
      tags: data.tags ? this.sanitizeArray(data.tags) : undefined,
      featuredImage: data.featuredImage ? this.sanitizeString(data.featuredImage) : undefined,
      status: data.status
    };
  }

  static sanitizeCommentInput(data: CreateCommentRequest): CreateCommentRequest {
    return {
      content: data.content.trim(),
      postId: this.sanitizeString(data.postId),
      authorId: this.sanitizeString(data.authorId),
      parentId: data.parentId ? this.sanitizeString(data.parentId) : undefined
    };
  }
}