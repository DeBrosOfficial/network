---
sidebar_position: 2
---

# Getting Started

This guide will help you set up DebrosFramework and create your first decentralized application in just a few minutes.

## Prerequisites

Before you begin, make sure you have:

- **Node.js** (version 18.0 or above)
- **npm** or **pnpm** package manager
- Basic knowledge of **TypeScript** and **decorators**
- Familiarity with **async/await** patterns

## Installation

### 1. Create a New Project

```bash
mkdir my-debros-app
cd my-debros-app
npm init -y
```

### 2. Install DebrosFramework

```bash
npm install debros-framework
npm install --save-dev typescript @types/node
```

### 3. Set Up TypeScript Configuration

Create a `tsconfig.json` file:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "lib": ["ES2020"],
    "experimentalDecorators": true,
    "emitDecoratorMetadata": true,
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "declaration": true,
    "outDir": "./dist",
    "rootDir": "./src"
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
```

### 4. Install OrbitDB and IPFS Dependencies

DebrosFramework requires OrbitDB and IPFS services:

```bash
npm install @orbitdb/core @helia/helia @helia/unixfs @libp2p/peer-id
```

## Your First Application

Let's create a simple social media application to demonstrate DebrosFramework's capabilities.

### 1. Create the Project Structure

```
src/
├── models/
│   ├── User.ts
│   ├── Post.ts
│   └── index.ts
├── services/
│   ├── orbitdb.ts
│   └── ipfs.ts
└── index.ts
```

### 2. Set Up IPFS Service

Create `src/services/ipfs.ts`:

```typescript
import { createHelia } from '@helia/helia';
import { createLibp2p } from 'libp2p';
// Add other necessary imports based on your setup

export class IPFSService {
  private helia: any;
  private libp2p: any;

  async init(): Promise<void> {
    // Initialize your IPFS/Helia instance
    // This is a simplified example - customize based on your needs
    this.libp2p = await createLibp2p({
      // Your libp2p configuration
    });

    this.helia = await createHelia({
      libp2p: this.libp2p,
    });
  }

  getHelia() {
    return this.helia;
  }

  getLibp2pInstance() {
    return this.libp2p;
  }

  async stop(): Promise<void> {
    if (this.helia) {
      await this.helia.stop();
    }
  }
}
```

### 3. Set Up OrbitDB Service

Create `src/services/orbitdb.ts`:

```typescript
import { createOrbitDB } from '@orbitdb/core';

export class OrbitDBService {
  private orbitdb: any;
  private ipfs: any;

  constructor(ipfsService: any) {
    this.ipfs = ipfsService;
  }

  async init(): Promise<void> {
    this.orbitdb = await createOrbitDB({
      ipfs: this.ipfs.getHelia(),
      // Add other OrbitDB configuration options
    });
  }

  async openDB(name: string, type: string): Promise<any> {
    return await this.orbitdb.open(name, { type });
  }

  getOrbitDB() {
    return this.orbitdb;
  }

  async stop(): Promise<void> {
    if (this.orbitdb) {
      await this.orbitdb.stop();
    }
  }
}
```

### 4. Define Your First Model

Create `src/models/User.ts`:

```typescript
import { BaseModel, Model, Field, HasMany } from 'debros-framework';
import { Post } from './Post';

@Model({
  scope: 'global', // Global model - shared across all users
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
    validate: (value: string) => value.length >= 3 && value.length <= 20,
  })
  username: string;

  @Field({
    type: 'string',
    required: true,
    unique: true,
    validate: (value: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value),
  })
  email: string;

  @Field({ type: 'string', required: false })
  bio?: string;

  @Field({ type: 'string', required: false })
  profilePicture?: string;

  @Field({ type: 'boolean', required: false, default: true })
  isActive: boolean;

  @Field({ type: 'number', required: false, default: () => Date.now() })
  registeredAt: number;

  // Relationship: One user has many posts
  @HasMany(() => Post, 'userId')
  posts: Post[];
}
```

### 5. Create a Post Model

Create `src/models/Post.ts`:

```typescript
import { BaseModel, Model, Field, BelongsTo } from 'debros-framework';
import { User } from './User';

@Model({
  scope: 'user', // User-scoped model - each user has their own database
  type: 'docstore',
  sharding: {
    strategy: 'user',
    count: 2,
    key: 'userId',
  },
})
export class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({
    type: 'string',
    required: true,
    validate: (value: string) => value.length <= 5000,
  })
  content: string;

  @Field({ type: 'string', required: true })
  userId: string;

  @Field({ type: 'array', required: false, default: [] })
  tags: string[];

  @Field({ type: 'boolean', required: false, default: true })
  isPublic: boolean;

  @Field({ type: 'number', required: false, default: 0 })
  likeCount: number;

  @Field({ type: 'number', required: false, default: () => Date.now() })
  createdAt: number;

  @Field({ type: 'number', required: false, default: () => Date.now() })
  updatedAt: number;

  // Relationship: Post belongs to a user
  @BelongsTo(() => User, 'userId')
  user: User;
}
```

### 6. Export Your Models

Create `src/models/index.ts`:

```typescript
export { User } from './User';
export { Post } from './Post';
```

### 7. Create the Main Application

Create `src/index.ts`:

```typescript
import { DebrosFramework } from 'debros-framework';
import { IPFSService } from './services/ipfs';
import { OrbitDBService } from './services/orbitdb';
import { User, Post } from './models';

async function main() {
  // Initialize services
  const ipfsService = new IPFSService();
  await ipfsService.init();

  const orbitDBService = new OrbitDBService(ipfsService);
  await orbitDBService.init();

  // Initialize DebrosFramework
  const framework = new DebrosFramework();
  await framework.initialize(orbitDBService, ipfsService);

  console.log('🚀 DebrosFramework initialized successfully!');

  // Create a user
  const user = await User.create({
    username: 'alice',
    email: 'alice@example.com',
    bio: 'Hello, I am Alice!',
    isActive: true,
  });

  console.log('✅ Created user:', user.id);

  // Create a post for the user
  const post = await Post.create({
    title: 'My First Post',
    content: 'This is my first post using DebrosFramework!',
    userId: user.id,
    tags: ['introduction', 'debros'],
    isPublic: true,
  });

  console.log('✅ Created post:', post.id);

  // Query users with their posts
  const usersWithPosts = await User.query().where('isActive', true).with(['posts']).find();

  console.log('📊 Users with posts:');
  usersWithPosts.forEach((user) => {
    console.log(`- ${user.username}: ${user.posts.length} posts`);
  });

  // Find posts by tags
  const taggedPosts = await Post.query()
    .where('tags', 'includes', 'debros')
    .with(['user'])
    .orderBy('createdAt', 'desc')
    .find();

  console.log('🏷️ Posts tagged with "debros":');
  taggedPosts.forEach((post) => {
    console.log(`- "${post.title}" by ${post.user.username}`);
  });

  // Clean up
  await framework.stop();
  console.log('👋 Framework stopped successfully');
}

main().catch(console.error);
```

### 8. Add Package.json Scripts

Update your `package.json`:

```json
{
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts"
  }
}
```

### 9. Install Additional Development Dependencies

```bash
npm install --save-dev ts-node
```

## Running Your Application

### 1. Build the Project

```bash
npm run build
```

### 2. Run the Application

```bash
npm start
```

Or for development with hot reloading:

```bash
npm run dev
```

You should see output similar to:

```
🚀 DebrosFramework initialized successfully!
✅ Created user: user_abc123
✅ Created post: post_def456
📊 Users with posts:
- alice: 1 posts
🏷️ Posts tagged with "debros":
- "My First Post" by alice
👋 Framework stopped successfully
```

## What's Next?

Congratulations! You've successfully created your first DebrosFramework application. Here's what you can explore next:

### Learn Core Concepts

- [Architecture Overview](./core-concepts/architecture) - Understand how DebrosFramework works
- [Models and Decorators](./core-concepts/models) - Deep dive into model definition
- [Database Management](./core-concepts/database-management) - Learn about user-scoped vs global databases

### Explore Advanced Features

- [Query System](./query-system/query-builder) - Build complex queries
- [Relationships](./query-system/relationships) - Work with model relationships
- [Automatic Pinning](./advanced/automatic-pinning) - Optimize data availability
- [Migrations](./advanced/migrations) - Evolve your schema over time

### Check Out Examples

- [Social Platform Example](./examples/social-platform) - Complete social media application
- [Complex Queries](./examples/complex-queries) - Advanced query patterns
- [Migration Examples](./examples/migrations) - Schema evolution patterns

## Common Issues

### TypeScript Decorator Errors

Make sure you have `"experimentalDecorators": true` and `"emitDecoratorMetadata": true` in your `tsconfig.json`.

### IPFS/OrbitDB Connection Issues

Ensure your IPFS and OrbitDB services are properly configured. Check the console for connection errors and verify your network configuration.

### Model Registration Issues

Models are automatically registered when imported. Make sure you're importing your models before using DebrosFramework.

## Need Help?

- 📖 Check our [comprehensive documentation](./core-concepts/architecture)
- 💻 Browse [example code](https://github.com/debros/network/tree/main/examples)
- 💬 Join our [Discord community](#)
- 📧 Contact [support](#)

Happy coding with DebrosFramework! 🎉
