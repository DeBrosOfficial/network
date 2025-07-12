#!/usr/bin/env node

// Import reflect-metadata first for decorator support
import 'reflect-metadata';

// Polyfill CustomEvent for Node.js environment
if (typeof globalThis.CustomEvent === 'undefined') {
  globalThis.CustomEvent = class CustomEvent<T = any> extends Event {
    detail: T;
    
    constructor(type: string, eventInitDict?: CustomEventInit<T>) {
      super(type, eventInitDict);
      this.detail = eventInitDict?.detail;
    }
  } as any;
}

import express from 'express';
import { DebrosFramework } from '../../../../src/framework/DebrosFramework';
import { User, UserProfile, Category, Post, Comment } from '../models/BlogModels';
import { BlogValidation, ValidationError } from '../models/BlogValidation';

class BlogAPIServer {
  private app: express.Application;
  private framework: DebrosFramework;
  private nodeId: string;

  constructor() {
    this.app = express();
    this.nodeId = process.env.NODE_ID || 'blog-node';
    this.setupMiddleware();
    this.setupRoutes();
  }

  private setupMiddleware() {
    this.app.use(express.json({ limit: '10mb' }));
    this.app.use(express.urlencoded({ extended: true }));

    // CORS
    this.app.use((req, res, next) => {
      res.header('Access-Control-Allow-Origin', '*');
      res.header('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
      res.header('Access-Control-Allow-Headers', 'Content-Type, Authorization');
      if (req.method === 'OPTIONS') {
        res.sendStatus(200);
      } else {
        next();
      }
    });

    // Logging
    this.app.use((req, res, next) => {
      console.log(`[${this.nodeId}] ${new Date().toISOString()} ${req.method} ${req.path}`);
      if (req.method === 'POST' && req.body) {
        console.log(`[${this.nodeId}] Request body:`, JSON.stringify(req.body, null, 2));
      }
      next();
    });
  }

  private setupRoutes() {
    // Health check
    this.app.get('/health', async (req, res) => {
      try {
        const peers = await this.getConnectedPeerCount();
        res.json({
          status: 'healthy',
          nodeId: this.nodeId,
          peers,
          timestamp: Date.now(),
        });
      } catch (error) {
        res.status(500).json({
          status: 'unhealthy',
          nodeId: this.nodeId,
          error: error.message,
        });
      }
    });

    // API routes
    this.setupUserRoutes();
    this.setupCategoryRoutes();
    this.setupPostRoutes();
    this.setupCommentRoutes();
    this.setupMetricsRoutes();
    
    // Error handling middleware must be defined after all routes
    this.app.use(
      (error: any, req: express.Request, res: express.Response, next: express.NextFunction) => {
        console.error(`[${this.nodeId}] Error:`, error);

        if (error instanceof ValidationError) {
          return res.status(400).json({
            error: error.message,
            field: error.field,
            nodeId: this.nodeId,
          });
        }

        res.status(500).json({
          error: 'Internal server error',
          nodeId: this.nodeId,
        });
      },
    );
  }

  private setupUserRoutes() {
    // Create user
    this.app.post('/api/users', async (req, res, next) => {
      try {
        console.log(`[${this.nodeId}] Received user creation request:`, JSON.stringify(req.body, null, 2));
        
        const sanitizedData = BlogValidation.sanitizeUserInput(req.body);
        console.log(`[${this.nodeId}] Sanitized user data:`, JSON.stringify(sanitizedData, null, 2));
        
        BlogValidation.validateUser(sanitizedData);
        console.log(`[${this.nodeId}] User validation passed`);

        const user = await User.create(sanitizedData);
        console.log(`[${this.nodeId}] User created successfully:`, JSON.stringify(user, null, 2));

        console.log(`[${this.nodeId}] Created user: ${user.username} (${user.id})`);
        res.status(201).json(user);
      } catch (error) {
        console.error(`[${this.nodeId}] Error creating user:`, error);
        next(error);
      }
    });

    // Get user by ID
    this.app.get('/api/users/:id', async (req, res, next) => {
      try {
        const user = await User.findById(req.params.id);
        if (!user) {
          return res.status(404).json({
            error: 'User not found',
            nodeId: this.nodeId,
          });
        }
        res.json(user.toJSON());
      } catch (error) {
        next(error);
      }
    });

    // Get all users
    this.app.get('/api/users', async (req, res, next) => {
      try {
        const page = parseInt(req.query.page as string) || 1;
        const limit = Math.min(parseInt(req.query.limit as string) || 20, 100);
        const search = req.query.search as string;

        let query = User.query();

        if (search) {
          query = query
            .where('username', 'like', `%${search}%`)
            .orWhere('displayName', 'like', `%${search}%`);
        }

const users = await query
  .orderBy('createdAt', 'desc')
  .limit(limit)
  .offset((page - 1) * limit)
  .find();

const userList = users ? users.map((u) => u.toJSON()) : [];

res.json({
  users: userList,
  page,
  limit,
  nodeId: this.nodeId,
});
      } catch (error) {
        next(error);
      }
    });

    // Update user
    this.app.put('/api/users/:id', async (req, res, next) => {
      try {
        const user = await User.findById(req.params.id);
        if (!user) {
          return res.status(404).json({
            error: 'User not found',
            nodeId: this.nodeId,
          });
        }

        // Only allow updating certain fields
        const allowedFields = ['displayName', 'avatar', 'roles'];
        const updateData: any = {};

        allowedFields.forEach((field) => {
          if (req.body[field] !== undefined) {
            updateData[field] = req.body[field];
          }
        });

        Object.assign(user, updateData);
        await user.save();

        console.log(`[${this.nodeId}] Updated user: ${user.username}`);
        res.json(user.toJSON());
      } catch (error) {
        next(error);
      }
    });

    // User login (update last login)
    this.app.post('/api/users/:id/login', async (req, res, next) => {
      try {
        const user = await User.findById(req.params.id);
        if (!user) {
          return res.status(404).json({
            error: 'User not found',
            nodeId: this.nodeId,
          });
        }

        await user.updateLastLogin();
        res.json({ message: 'Login recorded', lastLoginAt: user.lastLoginAt });
      } catch (error) {
        next(error);
      }
    });
  }

  private setupCategoryRoutes() {
    // Create category
    this.app.post('/api/categories', async (req, res, next) => {
      try {
        console.log(`[${this.nodeId}] Received category creation request:`, JSON.stringify(req.body, null, 2));
        
        const sanitizedData = BlogValidation.sanitizeCategoryInput(req.body);
        console.log(`[${this.nodeId}] Sanitized category data:`, JSON.stringify(sanitizedData, null, 2));
        
        // Generate slug if not provided
        if (!sanitizedData.slug && sanitizedData.name) {
          sanitizedData.slug = sanitizedData.name
            .toLowerCase()
            .replace(/\s+/g, '-')
            .replace(/[^a-z0-9-]/g, '')
            .replace(/--+/g, '-')
            .replace(/^-|-$/g, '');
          console.log(`[${this.nodeId}] Generated slug: ${sanitizedData.slug}`);
        }
        
        BlogValidation.validateCategory(sanitizedData);
        console.log(`[${this.nodeId}] Category validation passed`);

        const category = await Category.create(sanitizedData);
        console.log(`[${this.nodeId}] Category created successfully:`, JSON.stringify(category, null, 2));

        console.log(`[${this.nodeId}] Created category: ${category.name} (${category.id})`);
        res.status(201).json(category);
      } catch (error) {
        console.error(`[${this.nodeId}] Error creating category:`, error);
        next(error);
      }
    });

    // Get all categories
    this.app.get('/api/categories', async (req, res, next) => {
      try {
const categories = await Category.query()
  .where('isActive', true)
  .orderBy('name', 'asc')
  .find();

const categoryList = categories || [];

res.json({
  categories: categoryList,
  nodeId: this.nodeId,
});
      } catch (error) {
        next(error);
      }
    });

    // Get category by ID
    this.app.get('/api/categories/:id', async (req, res, next) => {
      try {
        const category = await Category.findById(req.params.id);
        if (!category) {
          return res.status(404).json({
            error: 'Category not found',
            nodeId: this.nodeId,
          });
        }
        res.json(category);
      } catch (error) {
        next(error);
      }
    });
  }

  private setupPostRoutes() {
    // Create post
    this.app.post('/api/posts', async (req, res, next) => {
      try {
        const sanitizedData = BlogValidation.sanitizePostInput(req.body);
        
        // Generate slug if not provided
        if (!sanitizedData.slug && sanitizedData.title) {
          sanitizedData.slug = sanitizedData.title
            .toLowerCase()
            .replace(/\s+/g, '-')
            .replace(/[^a-z0-9-]/g, '')
            .replace(/--+/g, '-')
            .replace(/^-|-$/g, '');
          console.log(`[${this.nodeId}] Generated slug: ${sanitizedData.slug}`);
        }
        
        BlogValidation.validatePost(sanitizedData);

        const post = await Post.create(sanitizedData);

        console.log(`[${this.nodeId}] Created post: ${post.title} (${post.id})`);
        res.status(201).json(post);
      } catch (error) {
        next(error);
      }
    });

    // Get post by ID with relationships
    this.app.get('/api/posts/:id', async (req, res, next) => {
      try {
        const post = await Post.query()
          .where('id', req.params.id)
          .with(['author', 'category', 'comments'])
          .first();

        if (!post) {
          return res.status(404).json({
            error: 'Post not found',
            nodeId: this.nodeId,
          });
        }

        res.json(post);
      } catch (error) {
        next(error);
      }
    });

    // Get all posts with pagination and filters
    this.app.get('/api/posts', async (req, res, next) => {
      try {
        const page = parseInt(req.query.page as string) || 1;
        const limit = Math.min(parseInt(req.query.limit as string) || 10, 50);
        const status = req.query.status as string;
        const authorId = req.query.authorId as string;
        const categoryId = req.query.categoryId as string;
        const tag = req.query.tag as string;

        let query = Post.query().with(['author', 'category']);

        if (status) {
          query = query.where('status', status);
        }

        if (authorId) {
          query = query.where('authorId', authorId);
        }

        if (categoryId) {
          query = query.where('categoryId', categoryId);
        }

        if (tag) {
          query = query.where('tags', 'includes', tag);
        }

        const posts = await query
          .orderBy('createdAt', 'desc')
          .limit(limit)
          .offset((page - 1) * limit)
          .find();

        const postList = posts || [];

        res.json({
          posts: postList,
          page,
          limit,
          nodeId: this.nodeId,
        });
      } catch (error) {
        next(error);
      }
    });

    // Update post
    this.app.put('/api/posts/:id', async (req, res, next) => {
      try {
        const post = await Post.query()
          .where('id', req.params.id)
          .first();
        if (!post) {
          return res.status(404).json({
            error: 'Post not found',
            nodeId: this.nodeId,
          });
        }

        BlogValidation.validatePostUpdate(req.body);

        Object.assign(post, req.body);
        post.updatedAt = Date.now();
        await post.save();

        console.log(`[${this.nodeId}] Updated post: ${post.title}`);
        res.json(post);
      } catch (error) {
        next(error);
      }
    });

    // Publish post
    this.app.post('/api/posts/:id/publish', async (req, res, next) => {
      try {
        const post = await Post.query()
          .where('id', req.params.id)
          .first();
        if (!post) {
          return res.status(404).json({
            error: 'Post not found',
            nodeId: this.nodeId,
          });
        }

        await post.publish();
        console.log(`[${this.nodeId}] Published post: ${post.title}`);
        res.json(post);
      } catch (error) {
        next(error);
      }
    });

    // Unpublish post
    this.app.post('/api/posts/:id/unpublish', async (req, res, next) => {
      try {
        const post = await Post.query()
          .where('id', req.params.id)
          .first();
        if (!post) {
          return res.status(404).json({
            error: 'Post not found',
            nodeId: this.nodeId,
          });
        }

        await post.unpublish();
        console.log(`[${this.nodeId}] Unpublished post: ${post.title}`);
        res.json(post);
      } catch (error) {
        next(error);
      }
    });

    // Like post
    this.app.post('/api/posts/:id/like', async (req, res, next) => {
      try {
        const post = await Post.query()
          .where('id', req.params.id)
          .first();
        if (!post) {
          return res.status(404).json({
            error: 'Post not found',
            nodeId: this.nodeId,
          });
        }

        await post.like();
        res.json({ likeCount: post.likeCount });
      } catch (error) {
        next(error);
      }
    });

    // View post (increment view count)
    this.app.post('/api/posts/:id/view', async (req, res, next) => {
      try {
        const post = await Post.query()
          .where('id', req.params.id)
          .first();
        if (!post) {
          return res.status(404).json({
            error: 'Post not found',
            nodeId: this.nodeId,
          });
        }

        await post.incrementViews();
        res.json({ viewCount: post.viewCount });
      } catch (error) {
        next(error);
      }
    });
  }

  private setupCommentRoutes() {
    // Create comment
    this.app.post('/api/comments', async (req, res, next) => {
      try {
        const sanitizedData = BlogValidation.sanitizeCommentInput(req.body);
        BlogValidation.validateComment(sanitizedData);

        const comment = await Comment.create(sanitizedData);

        console.log(
          `[${this.nodeId}] Created comment on post ${comment.postId} by ${comment.authorId}`,
        );
        res.status(201).json(comment);
      } catch (error) {
        next(error);
      }
    });

    // Get comments for a post
    this.app.get('/api/posts/:postId/comments', async (req, res, next) => {
      try {
        const comments = await Comment.query()
          .where('postId', req.params.postId)
          .where('isApproved', true)
          .with(['author'])
          .orderBy('createdAt', 'asc')
          .find();

        const commentList = comments || [];

        res.json({
          comments: commentList,
          nodeId: this.nodeId,
        });
      } catch (error) {
        next(error);
      }
    });

    // Approve comment
    this.app.post('/api/comments/:id/approve', async (req, res, next) => {
      try {
        const comment = await Comment.query()
          .where('id', req.params.id)
          .first();
        if (!comment) {
          return res.status(404).json({
            error: 'Comment not found',
            nodeId: this.nodeId,
          });
        }

        await comment.approve();
        console.log(`[${this.nodeId}] Approved comment ${comment.id}`);
        res.json(comment);
      } catch (error) {
        next(error);
      }
    });

    // Like comment
    this.app.post('/api/comments/:id/like', async (req, res, next) => {
      try {
        const comment = await Comment.query()
          .where('id', req.params.id)
          .first();
        if (!comment) {
          return res.status(404).json({
            error: 'Comment not found',
            nodeId: this.nodeId,
          });
        }

        await comment.like();
        res.json({ likeCount: comment.likeCount });
      } catch (error) {
        next(error);
      }
    });
  }

  private setupMetricsRoutes() {
    // Network metrics
    this.app.get('/api/metrics/network', async (req, res, next) => {
      try {
        const peers = await this.getConnectedPeerCount();
        res.json({
          nodeId: this.nodeId,
          peers,
          timestamp: Date.now(),
        });
      } catch (error) {
        next(error);
      }
    });

    // Data metrics
    this.app.get('/api/metrics/data', async (req, res, next) => {
      try {
        const [userCount, postCount, commentCount, categoryCount] = await Promise.all([
          User.count(),
          Post.count(),
          Comment.count(),
          Category.count(),
        ]);

        res.json({
          nodeId: this.nodeId,
          counts: {
            users: userCount,
            posts: postCount,
            comments: commentCount,
            categories: categoryCount,
          },
          timestamp: Date.now(),
        });
      } catch (error) {
        next(error);
      }
    });

    // Framework metrics
    this.app.get('/api/metrics/framework', async (req, res, next) => {
      try {
        const metrics = this.framework ? this.framework.getMetrics() : null;
        const defaultMetrics = {
          services: 'unknown',
          environment: 'unknown',
          features: 'unknown'
        };
        
        res.json({
          nodeId: this.nodeId,
          ...(metrics || defaultMetrics),
          timestamp: Date.now(),
        });
      } catch (error) {
        next(error);
      }
    });
  }

  private async getConnectedPeerCount(): Promise<number> {
    try {
      if (this.framework) {
        const ipfsService = this.framework.getIPFSService();
        if (ipfsService && ipfsService.getConnectedPeers) {
          const peers = await ipfsService.getConnectedPeers();
          return peers.size;
        }
      }
      return 0;
    } catch (error) {
      console.warn(`[${this.nodeId}] Failed to get peer count:`, error.message);
      return 0;
    }
  }

  async start() {
    try {
      console.log(`[${this.nodeId}] Starting Blog API Server...`);

      // Wait for dependencies
      await this.waitForDependencies();

      // Initialize framework
      await this.initializeFramework();

      // Start HTTP server
      const port = process.env.NODE_PORT || 3000;
      this.app.listen(port, () => {
        console.log(`[${this.nodeId}] Blog API server listening on port ${port}`);
        console.log(`[${this.nodeId}] Health check: http://localhost:${port}/health`);
      });
    } catch (error) {
      console.error(`[${this.nodeId}] Failed to start:`, error);
      process.exit(1);
    }
  }

  private async waitForDependencies(): Promise<void> {
    // In a real deployment, you might wait for database connections, etc.
    console.log(`[${this.nodeId}] Dependencies ready`);
  }

  private async initializeFramework(): Promise<void> {
    // Import services
    const { IPFSService } = await import('../../../../src/framework/services/IPFSService');
    const { OrbitDBService } = await import(
      '../../../../src/framework/services/RealOrbitDBService'
    );

    // Initialize IPFS service
    const ipfsService = new IPFSService({
      swarmKeyFile: process.env.SWARM_KEY_FILE,
      bootstrap: process.env.BOOTSTRAP_PEER ? [`/ip4/${process.env.BOOTSTRAP_PEER}/tcp/4001`] : [],
      ports: {
        swarm: parseInt(process.env.IPFS_PORT) || 4001,
      },
    });

    await ipfsService.init();
    console.log(`[${this.nodeId}] IPFS service initialized`);

    // Initialize OrbitDB service
    const orbitDBService = new OrbitDBService(ipfsService);
    await orbitDBService.init();
    console.log(`[${this.nodeId}] OrbitDB service initialized`);

    // Debug: Check OrbitDB service methods
    console.log(`[${this.nodeId}] OrbitDB service methods:`, Object.getOwnPropertyNames(Object.getPrototypeOf(orbitDBService)));
    console.log(`[${this.nodeId}] Has openDB method:`, typeof orbitDBService.openDB === 'function');
    
    // Initialize framework
    this.framework = new DebrosFramework({
      appName: 'blog-app', // Unique app name for this blog application
      environment: 'test',
      features: {
        autoMigration: true,
        automaticPinning: true,
        pubsub: true,
        queryCache: true,
        relationshipCache: true,
      },
      performance: {
        queryTimeout: 10000,
        maxConcurrentOperations: 20,
        batchSize: 50,
      },
    });

    // Pass raw services to framework - it will wrap them itself
    await this.framework.initialize(orbitDBService, ipfsService);
    console.log(`[${this.nodeId}] DebrosFramework initialized successfully`);

    // Register models with framework
    this.framework.registerModel(User, {
      scope: 'global',
      type: 'docstore'
    });
    this.framework.registerModel(UserProfile, {
      scope: 'global',
      type: 'docstore'
    });
    this.framework.registerModel(Category, {
      scope: 'global',
      type: 'docstore'
    });
    this.framework.registerModel(Post, {
      scope: 'user',
      type: 'docstore'
    });
    this.framework.registerModel(Comment, {
      scope: 'user',
      type: 'docstore'
    });
    console.log(`[${this.nodeId}] Models registered with framework`);
  }
}

// Handle graceful shutdown
process.on('SIGTERM', () => {
  console.log('Received SIGTERM, shutting down gracefully...');
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('Received SIGINT, shutting down gracefully...');
  process.exit(0);
});

// Start the server
const server = new BlogAPIServer();
server.start();
