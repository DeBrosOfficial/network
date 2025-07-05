---
sidebar_position: 1
---

# Welcome to @debros/network

**@debros/network** is a powerful Node.js library that provides a simple, database-like API over OrbitDB and IPFS, making it easy to build decentralized applications with familiar database operations.

## What is @debros/network?

@debros/network simplifies decentralized application development by providing:

- **Simple Database API**: Familiar CRUD operations for decentralized data
- **Multiple Store Types**: KeyValue, Document, Feed, and Counter stores
- **Schema Validation**: Built-in validation for data integrity
- **Transaction Support**: Batch operations for consistency
- **File Storage**: Built-in file upload and retrieval with IPFS
- **Real-time Events**: Subscribe to data changes
- **TypeScript Support**: Full TypeScript support with type safety
- **Connection Management**: Handle multiple database connections

## Key Features

### 🗄️ Database-like Operations

Perform familiar database operations on decentralized data:

```typescript
import { initDB, create, get, query } from '@debros/network';

// Initialize the database
await initDB();

// Create documents
await create('users', 'user123', {
  username: 'alice',
  email: 'alice@example.com',
});

// Query with filtering
const activeUsers = await query('users', (user) => user.isActive === true, {
  limit: 10,
  sort: { field: 'createdAt', order: 'desc' },
});
```

### 📁 File Storage

Upload and manage files on IPFS:

```typescript
import { uploadFile, getFile } from '@debros/network';

// Upload a file
const fileData = Buffer.from('Hello World');
const result = await uploadFile(fileData, {
  filename: 'hello.txt',
  metadata: { type: 'text' },
});

// Retrieve file
const file = await getFile(result.cid);
```

### 🔄 Real-time Updates

Subscribe to data changes:

```typescript
import { subscribe } from '@debros/network';

const unsubscribe = subscribe('document:created', (data) => {
  console.log(`New document created: ${data.id}`);
});
```

### 📊 Schema Validation

Define schemas for data validation:

```typescript
import { defineSchema } from '@debros/network';

defineSchema('users', {
  properties: {
    username: { type: 'string', required: true, min: 3 },
    email: { type: 'string', pattern: '^[\\w-\\.]+@([\\w-]+\\.)+[\\w-]{2,4}$' },
  },
});
```

## Architecture Overview

@debros/network provides a clean abstraction layer over OrbitDB and IPFS:

```
┌─────────────────────────────────────────────────────────────┐
│                    Your Application                         │
├─────────────────────────────────────────────────────────────┤
│                  @debros/network API                        │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │   Database  │ │    File     │ │   Schema    │            │
│  │ Operations  │ │   Storage   │ │ Validation  │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │Transaction  │ │   Events    │ │ Connection  │            │
│  │   System    │ │   System    │ │ Management  │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
├─────────────────────────────────────────────────────────────┤
│                   OrbitDB Layer                             │
├─────────────────────────────────────────────────────────────┤
│                    IPFS Layer                               │
└─────────────────────────────────────────────────────────────┘
```

## Who Should Use @debros/network?

@debros/network is perfect for developers who want to:

- Build decentralized applications with familiar database patterns
- Store and query data in a distributed manner
- Handle file storage on IPFS seamlessly
- Create applications with real-time data synchronization
- Use TypeScript for type-safe decentralized development
- Avoid dealing with low-level OrbitDB and IPFS complexities

## Getting Started

Ready to build your first decentralized application? Check out our [Getting Started Guide](./getting-started) to set up your development environment and start building.

## Community and Support

- 📖 [Documentation](./getting-started) - Comprehensive guides and examples
- 💻 [GitHub Repository](https://github.com/debros/network) - Source code and issue tracking
- 💬 [Discord Community](#) - Chat with other developers
- 📧 [Support Email](#) - Get help from the core team

---

_@debros/network makes decentralized application development as simple as traditional database operations, while providing the benefits of distributed systems._
