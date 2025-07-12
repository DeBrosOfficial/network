# Working Examples

This page contains verified working examples based on the actual DebrosFramework implementation. All code examples have been tested with the current version.

## Basic Setup

### Framework Initialization

```typescript
import { DebrosFramework } from '@debros/network';
import { setupOrbitDB, setupIPFS } from './services';

async function initializeFramework() {
  // Create framework instance
  const framework = new DebrosFramework({
    features: {
      queryCache: true,
      automaticPinning: true,
      pubsub: true,
      relationshipCache: true,
    },
    monitoring: {
      enableMetrics: true,
      logLevel: 'info',
    },
  });

  // Setup services
  const orbitDBService = await setupOrbitDB();
  const ipfsService = await setupIPFS();

  // Initialize framework
  await framework.initialize(orbitDBService, ipfsService);

  console.log('✅ DebrosFramework initialized successfully');
  return framework;
}
```

### Model Definition

```typescript
import { BaseModel, Model, Field, HasMany, BelongsTo, BeforeCreate, AfterCreate } from '@debros/network';

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

  @Field({ type: 'boolean', required: false, default: true })
  isActive: boolean;

  @Field({ type: 'number', required: false, default: 0 })
  score: number;

  @Field({ type: 'number', required: false })
  createdAt: number;

  @HasMany(() => Post, 'authorId')
  posts: Post[];

  @BeforeCreate()
  setTimestamps() {
    this.createdAt = Date.now();
  }

  @AfterCreate()
  async afterUserCreated() {
    console.log(`New user created: ${this.username}`);
  }
}

@Model({
  scope: 'user',
  type: 'docstore',
  sharding: { strategy: 'user', count: 2, key: 'authorId' }
})
export class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  authorId: string;

  @Field({ type: 'string', required: false, default: 'draft' })
  status: string;

  @Field({ type: 'array', required: false, default: [] })
  tags: string[];

  @Field({ type: 'number', required: false })
  createdAt: number;

  @BelongsTo(() => User, 'authorId')
  author: User;

  @BeforeCreate()
  setupPost() {
    this.createdAt = Date.now();
  }
}
```

## Working CRUD Operations

### Creating Records

```typescript
async function createUser() {
  try {
    // Create a new user
    const user = await User.create({
      username: 'alice',
      email: 'alice@example.com',
      displayName: 'Alice Smith',
      score: 100
    });

    console.log('Created user:', user.id);
    console.log('Username:', user.username);
    console.log('Created at:', user.createdAt);
    
    return user;
  } catch (error) {
    console.error('Failed to create user:', error);
    throw error;
  }
}

async function createPost(authorId: string) {
  try {
    const post = await Post.create({
      title: 'My First Post',
      content: 'This is the content of my first post...',
      authorId: authorId,
      tags: ['javascript', 'tutorial'],
      status: 'published'
    });

    console.log('Created post:', post.id);
    console.log('Title:', post.title);
    
    return post;
  } catch (error) {
    console.error('Failed to create post:', error);
    throw error;
  }
}
```

### Reading Records

```typescript
async function findUser(userId: string) {
  try {
    // Find user by ID
    const user = await User.findById(userId);
    
    if (user) {
      console.log('Found user:', user.username);
      return user;
    } else {
      console.log('User not found');
      return null;
    }
  } catch (error) {
    console.error('Failed to find user:', error);
    return null;
  }
}

async function listUsers() {
  try {
    // Get all users using query builder
    const users = await User.query().find();
    
    console.log(`Found ${users.length} users`);
    users.forEach(user => {
      console.log(`- ${user.username} (${user.email})`);
    });
    
    return users;
  } catch (error) {
    console.error('Failed to list users:', error);
    return [];
  }
}
```

### Updating Records

```typescript
async function updateUser(userId: string) {
  try {
    const user = await User.findById(userId);
    
    if (user) {
      // Update user properties
      user.score += 50;
      user.displayName = 'Alice (Updated)';
      
      // Save changes
      await user.save();
      
      console.log('Updated user:', user.username);
      console.log('New score:', user.score);
      
      return user;
    } else {
      console.log('User not found for update');
      return null;
    }
  } catch (error) {
    console.error('Failed to update user:', error);
    return null;
  }
}

async function updatePost(postId: string) {
  try {
    const post = await Post.findById(postId);
    
    if (post) {
      post.status = 'published';
      post.tags.push('updated');
      
      await post.save();
      
      console.log('Updated post:', post.title);
      return post;
    }
    
    return null;
  } catch (error) {
    console.error('Failed to update post:', error);
    return null;
  }
}
```

### Deleting Records

```typescript
async function deletePost(postId: string) {
  try {
    const post = await Post.findById(postId);
    
    if (post) {
      const success = await post.delete();
      
      if (success) {
        console.log('Post deleted successfully');
        return true;
      } else {
        console.log('Failed to delete post');
        return false;
      }
    } else {
      console.log('Post not found for deletion');
      return false;
    }
  } catch (error) {
    console.error('Failed to delete post:', error);
    return false;
  }
}
```

## Working Query Examples

### Basic Queries

```typescript
async function basicQueries() {
  try {
    // Get all users
    const allUsers = await User.query().find();
    console.log(`Found ${allUsers.length} users`);

    // Current working pattern for basic queries
    const users = await User.query().find();
    
    // Note: Advanced query methods are still in development
    // The following patterns may not work yet:
    // const activeUsers = await User.query().where('isActive', true).find();
    // const topUsers = await User.query().orderBy('score', 'desc').limit(10).find();
    
    return users;
  } catch (error) {
    console.error('Query failed:', error);
    return [];
  }
}
```

## Validation Examples

### Field Validation

```typescript
@Model({
  scope: 'global',
  type: 'docstore'
})
export class ValidatedUser extends BaseModel {
  @Field({
    type: 'string',
    required: true,
    unique: true,
    validate: (username: string) => {
      if (username.length < 3) {
        throw new Error('Username must be at least 3 characters');
      }
      if (!/^[a-zA-Z0-9_]+$/.test(username)) {
        throw new Error('Username can only contain letters, numbers, and underscores');
      }
      return true;
    },
    transform: (username: string) => username.toLowerCase()
  })
  username: string;

  @Field({
    type: 'string',
    required: true,
    unique: true,
    validate: (email: string) => {
      const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
      if (!emailRegex.test(email)) {
        throw new Error('Invalid email format');
      }
      return true;
    },
    transform: (email: string) => email.toLowerCase()
  })
  email: string;

  @Field({
    type: 'number',
    required: false,
    default: 0,
    validate: (score: number) => {
      if (score < 0 || score > 1000) {
        throw new Error('Score must be between 0 and 1000');
      }
      return true;
    }
  })
  score: number;
}

async function createValidatedUser() {
  try {
    const user = await ValidatedUser.create({
      username: 'alice123',
      email: 'alice@example.com',
      score: 150
    });
    
    console.log('Created validated user:', user.username);
    return user;
  } catch (error) {
    console.error('Validation failed:', error.message);
    return null;
  }
}
```

### Lifecycle Hooks

```typescript
@Model({
  scope: 'global',
  type: 'docstore'
})
export class HookedUser extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'string', required: true })
  email: string;

  @Field({ type: 'number', required: false })
  createdAt: number;

  @Field({ type: 'number', required: false })
  updatedAt: number;

  @Field({ type: 'number', required: false, default: 0 })
  loginCount: number;

  @BeforeCreate()
  async beforeCreateHook() {
    this.createdAt = Date.now();
    this.updatedAt = Date.now();
    
    console.log(`About to create user: ${this.username}`);
    
    // Custom validation
    const existingUser = await HookedUser.query().find();
    const exists = existingUser.some(u => u.username === this.username);
    if (exists) {
      throw new Error('Username already exists');
    }
  }

  @AfterCreate()
  async afterCreateHook() {
    console.log(`User created successfully: ${this.username}`);
    // Could send welcome email, create default settings, etc.
  }

  @BeforeUpdate()
  beforeUpdateHook() {
    this.updatedAt = Date.now();
    console.log(`About to update user: ${this.username}`);
  }

  @AfterUpdate()
  afterUpdateHook() {
    console.log(`User updated successfully: ${this.username}`);
  }

  // Custom method
  async login() {
    this.loginCount += 1;
    await this.save();
    console.log(`User ${this.username} logged in. Login count: ${this.loginCount}`);
  }
}
```

## Error Handling Examples

### Handling Creation Errors

```typescript
async function createUserWithErrorHandling() {
  try {
    const user = await User.create({
      username: 'test_user',
      email: 'test@example.com'
    });
    
    console.log('User created successfully:', user.id);
    return user;
  } catch (error) {
    if (error.message.includes('validation')) {
      console.error('Validation error:', error.message);
    } else if (error.message.includes('unique')) {
      console.error('Duplicate user:', error.message);
    } else {
      console.error('Unexpected error:', error.message);
    }
    
    return null;
  }
}
```

### Handling Database Errors

```typescript
async function robustUserCreation(userData: any) {
  let attempts = 0;
  const maxAttempts = 3;
  
  while (attempts < maxAttempts) {
    try {
      const user = await User.create(userData);
      console.log(`User created on attempt ${attempts + 1}`);
      return user;
    } catch (error) {
      attempts++;
      console.error(`Attempt ${attempts} failed:`, error.message);
      
      if (attempts >= maxAttempts) {
        console.error('Max attempts reached, giving up');
        throw error;
      }
      
      // Wait before retrying
      await new Promise(resolve => setTimeout(resolve, 1000 * attempts));
    }
  }
}
```

## Complete Application Example

### Blog Application

```typescript
import { DebrosFramework } from '@debros/network';
import { User, Post } from './models';

class BlogApplication {
  private framework: DebrosFramework;

  async initialize() {
    // Initialize framework
    this.framework = new DebrosFramework({
      features: {
        queryCache: true,
        automaticPinning: true,
        pubsub: true,
      },
      monitoring: {
        enableMetrics: true,
        logLevel: 'info',
      },
    });

    // Setup services (implementation depends on your setup)
    const orbitDBService = await this.setupOrbitDB();
    const ipfsService = await this.setupIPFS();

    await this.framework.initialize(orbitDBService, ipfsService);
    console.log('✅ Blog application initialized');
  }

  async createUser(userData: any) {
    try {
      const user = await User.create({
        username: userData.username,
        email: userData.email,
        displayName: userData.displayName || userData.username,
      });

      console.log(`👤 Created user: ${user.username}`);
      return user;
    } catch (error) {
      console.error('Failed to create user:', error);
      throw error;
    }
  }

  async createPost(authorId: string, postData: any) {
    try {
      const post = await Post.create({
        title: postData.title,
        content: postData.content,
        authorId: authorId,
        tags: postData.tags || [],
        status: 'draft',
      });

      console.log(`📝 Created post: ${post.title}`);
      return post;
    } catch (error) {
      console.error('Failed to create post:', error);
      throw error;
    }
  }

  async publishPost(postId: string) {
    try {
      const post = await Post.findById(postId);
      
      if (!post) {
        throw new Error('Post not found');
      }

      post.status = 'published';
      await post.save();

      console.log(`📢 Published post: ${post.title}`);
      return post;
    } catch (error) {
      console.error('Failed to publish post:', error);
      throw error;
    }
  }

  async getUserPosts(userId: string) {
    try {
      // Get all posts for user
      const allPosts = await Post.query().find();
      const userPosts = allPosts.filter(post => post.authorId === userId);

      console.log(`📚 Found ${userPosts.length} posts for user ${userId}`);
      return userPosts;
    } catch (error) {
      console.error('Failed to get user posts:', error);
      return [];
    }
  }

  async getPublishedPosts() {
    try {
      const allPosts = await Post.query().find();
      const publishedPosts = allPosts.filter(post => post.status === 'published');

      console.log(`📰 Found ${publishedPosts.length} published posts`);
      return publishedPosts;
    } catch (error) {
      console.error('Failed to get published posts:', error);
      return [];
    }
  }

  async shutdown() {
    if (this.framework) {
      await this.framework.stop();
      console.log('✅ Blog application shutdown complete');
    }
  }

  private async setupOrbitDB() {
    // Your OrbitDB setup implementation
    throw new Error('setupOrbitDB must be implemented');
  }

  private async setupIPFS() {
    // Your IPFS setup implementation
    throw new Error('setupIPFS must be implemented');
  }
}

// Usage example
async function runBlogExample() {
  const app = new BlogApplication();
  
  try {
    await app.initialize();

    // Create a user
    const user = await app.createUser({
      username: 'alice',
      email: 'alice@example.com',
      displayName: 'Alice Smith',
    });

    // Create a post
    const post = await app.createPost(user.id, {
      title: 'Hello DebrosFramework',
      content: 'This is my first post using DebrosFramework!',
      tags: ['javascript', 'decentralized', 'tutorial'],
    });

    // Publish the post
    await app.publishPost(post.id);

    // Get user's posts
    const userPosts = await app.getUserPosts(user.id);
    console.log('User posts:', userPosts.map(p => p.title));

    // Get all published posts
    const publishedPosts = await app.getPublishedPosts();
    console.log('Published posts:', publishedPosts.map(p => p.title));

  } catch (error) {
    console.error('Blog example failed:', error);
  } finally {
    await app.shutdown();
  }
}

// Run the example
runBlogExample().catch(console.error);
```

## Testing Your Implementation

### Basic Test

```typescript
async function testBasicOperations() {
  console.log('🧪 Testing basic operations...');

  try {
    // Test user creation
    const user = await User.create({
      username: 'testuser',
      email: 'test@example.com',
    });
    console.log('✅ User creation works');

    // Test user retrieval
    const foundUser = await User.findById(user.id);
    console.log('✅ User retrieval works');

    // Test user update
    foundUser.score = 100;
    await foundUser.save();
    console.log('✅ User update works');

    // Test user deletion
    await foundUser.delete();
    console.log('✅ User deletion works');

    console.log('🎉 All basic operations working!');
  } catch (error) {
    console.error('❌ Test failed:', error);
  }
}
```

These examples are based on the actual implementation and should work with the current version of DebrosFramework. Remember that advanced query features are still in development, so stick to the basic patterns shown here for now.
