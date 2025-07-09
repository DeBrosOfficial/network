---
sidebar_position: 1
---

# Social Platform Example

This example demonstrates how to build a simple social media platform using DebrosFramework.

## Overview

In this example, you'll build a social media application featuring users, posts, and comments.

## Project Structure

```plaintext
src/
├── models/
│   ├── User.ts
│   ├── Post.ts
│   ├── Comment.ts
│   └── index.ts
├── services/
│   ├── ipfs.ts
│   ├── orbitdb.ts
│   └── index.ts
└── index.ts
```

## Setting Up the Environment

### Install Dependencies

```bash
npm install @debros/network
npm install --save-dev typescript @types/node
```

### Configure TypeScript

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

## Building the Application

### 1. User Model

Create `src/models/User.ts`:

```typescript
import { BaseModel, Model, Field, HasMany } from 'debros-framework';
import { Post } from './Post';
import { Comment } from './Comment';

@Model({
  scope: 'global', // Global model
  type: 'docstore'
})
export class User extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  username: string;

  @Field({ type: 'string', required: true, unique: true })
  email: string;

  @HasMany(() => Post, 'userId')
  posts: Post[];

  @HasMany(() => Comment, 'userId')
  comments: Comment[];
}
```

### 2. Post Model

Create `src/models/Post.ts`:

```typescript
import { BaseModel, Model, Field, BelongsTo, HasMany } from 'debros-framework';
import { User } from './User';
import { Comment } from './Comment';

@Model({
  scope: 'user', // User-scoped model
  type: 'docstore'
})
export class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  userId: string;

  @BelongsTo(() => User, 'userId')
  user: User;

  @HasMany(() => Comment, 'postId')
  comments: Comment[];
}
```

### 3. Comment Model

Create `src/models/Comment.ts`:

```typescript
import { BaseModel, Model, Field, BelongsTo } from 'debros-framework';
import { User } from './User';
import { Post } from './Post';

@Model({
  scope: 'user', // User-scoped model
  type: 'docstore'
})
export class Comment extends BaseModel {
  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  userId: string;

  @Field({ type: 'string', required: true })
  postId: string;

  @BelongsTo(() => User, 'userId')
  user: User;

  @BelongsTo(() => Post, 'postId')
  post: Post;
}
```

### 4. IPFS and OrbitDB Service

Create `src/services/ipfs.ts` and `src/services/orbitdb.ts` following similar patterns to the setup in earlier examples.

### 5. Main Application

Create `src/index.ts`:

```typescript
import { DebrosFramework } from 'debros-framework';
import { IPFSService } from './services/ipfs';
import { OrbitDBService } from './services/orbitdb';
import { User, Post, Comment } from './models';

async function main() {
  // Initialize services
  const ipfsService = new IPFSService();
  await ipfsService.init();

  const orbitDBService = new OrbitDBService(ipfsService);
  await orbitDBService.init();

  // Initialize DebrosFramework
  const framework = new DebrosFramework();
  await framework.initialize(orbitDBService, ipfsService);

  console.log('🚀 Social Platform initialized successfully!');

  // Example: Create a user and post
  const user = await User.create({
    username: 'alice',
    email: 'alice@example.com'
  });

  const post = await Post.create({
    title: 'Hello, World!',
    content: 'This is my first post.',
    userId: user.id
  });

  console.log('✅ User and Post created successfully!');

  // Clean up
  await framework.stop();
  console.log('👋 Framework stopped successfully');
}

main().catch(console.error);
```

## Running the Application

### Build and Run

```bash
npm run build
npm start
```

### Development

```bash
npm run dev
```

## Conclusion

In this example, you have seen how to leverage DebrosFramework to build a basic social media application. You can enhance it by adding more features, such as user authentication, friend networks, or media uploads.
