# @debros/network

Core networking functionality for the Debros decentralized network. This package provides a powerful database interface with advanced features built on IPFS and OrbitDB for decentralized applications.

## Features

- Rich database-like API with TypeScript support
- Multiple database store types (KeyValue, Document, Feed, Counter)
- Document operations with schema validation
- Advanced querying with pagination, sorting and filtering
- Transaction support for batch operations
- Built-in file storage with metadata
- Real-time subscriptions for data changes
- Memory caching for performance
- Connection pooling for managing multiple database instances
- Index creation for faster queries
- Comprehensive error handling with error codes
- Performance metrics and monitoring

## Installation

```bash
npm install @debros/network
```

## Basic Usage

```typescript
import { initDB, create, get, query, uploadFile, logger } from "@debros/network";

// Initialize the database service
async function startApp() {
  try {
    // Initialize with default configuration
    await initDB();
    logger.info("Database initialized successfully");
    
    // Create a new user document
    const userId = 'user123';
    const user = {
      username: 'johndoe',
      walletAddress: '0x1234567890',
      avatar: null
    };
    
    const result = await create('users', userId, user);
    logger.info(`Created user with ID: ${result.id}`);
    
    // Get a user by ID
    const retrievedUser = await get('users', userId);
    logger.info('User:', retrievedUser);
    
    // Query users with filtering
    const activeUsers = await query('users', 
      user => user.isActive === true,
      { limit: 10, sort: { field: 'createdAt', order: 'desc' } }
    );
    logger.info(`Found ${activeUsers.total} active users`);
    
    // Upload a file
    const fileData = Buffer.from('File content');
    const fileUpload = await uploadFile(fileData, { filename: 'document.txt' });
    logger.info(`Uploaded file with CID: ${fileUpload.cid}`);
    
    return true;
  } catch (error) {
    logger.error("Failed to start app:", error);
    throw error;
  }
}

startApp();
```

## Database Store Types

The library supports multiple OrbitDB store types, each optimized for different use cases:

```typescript
import { create, get, update, StoreType } from "@debros/network";

// Default KeyValue store (for general use)
await create('users', 'user1', { name: 'Alice' });

// Document store (better for complex documents with indexing)
await create('posts', 'post1', { title: 'Hello', content: '...' }, 
  { storeType: StoreType.DOCSTORE }
);

// Feed/EventLog store (append-only, good for immutable logs)
await create('events', 'evt1', { type: 'login', user: 'alice' }, 
  { storeType: StoreType.FEED }
);

// Counter store (for numeric counters)
await create('stats', 'visits', { value: 0 }, 
  { storeType: StoreType.COUNTER }
);

// Increment a counter
await update('stats', 'visits', { increment: 1 }, 
  { storeType: StoreType.COUNTER }
);

// Get counter value
const stats = await get('stats', 'visits', { storeType: StoreType.COUNTER });
console.log(`Visit count: ${stats.value}`);
```

## Advanced Features

### Schema Validation

```typescript
import { defineSchema, create } from "@debros/network";

// Define a schema
defineSchema('users', {
  properties: {
    username: {
      type: 'string',
      required: true,
      min: 3,
      max: 20
    },
    email: {
      type: 'string',
      pattern: '^[\w-\.]+@([\w-]+\.)+[\w-]{2,4}$'
    },
    age: {
      type: 'number',
      min: 18
    }
  },
  required: ['username']
});

// Document creation will be validated against the schema
await create('users', 'user1', {
  username: 'alice',
  email: 'alice@example.com',
  age: 25
});
```

### Transactions

```typescript
import { createTransaction, commitTransaction } from "@debros/network";

// Create a transaction
const transaction = createTransaction();

// Add multiple operations
transaction
  .create('posts', 'post1', { title: 'Hello World', content: '...' })
  .update('users', 'user1', { postCount: 1 })
  .delete('drafts', 'draft1');

// Commit all operations
const result = await commitTransaction(transaction);
console.log(`Transaction completed with ${result.results.length} operations`);
```

### Subscriptions

```typescript
import { subscribe } from "@debros/network";

// Subscribe to document changes
const unsubscribe = subscribe('document:created', (data) => {
  console.log(`New document created in ${data.collection}:`, data.id);
});

// Later, unsubscribe
unsubscribe();
```

### Pagination and Sorting

```typescript
import { list, query } from "@debros/network";

// List with pagination and sorting
const page1 = await list('users', {
  limit: 10,
  offset: 0,
  sort: { field: 'createdAt', order: 'desc' }
});

// Query with pagination
const results = await query('users', 
  (user) => user.age > 21, 
  { limit: 10, offset: 20 }
);

console.log(`Found ${results.total} matches, showing ${results.documents.length}`);
console.log(`Has more pages: ${results.hasMore}`);
```

### TypeScript Support

```typescript
import { get, update, query } from "@debros/network";

interface User {
  username: string;
  email: string;
  age: number;
  createdAt: number;
  updatedAt: number;
}

// Type-safe operations
const user = await get<User>('users', 'user1');

await update<User>('users', 'user1', { age: 26 });

const results = await query<User>('users', 
  (user) => user.age > 21
);
```

### Connection Management

```typescript
import { initDB, closeConnection } from "@debros/network";

// Create multiple connections
const conn1 = await initDB('connection1');
const conn2 = await initDB('connection2');

// Use specific connection
await create('users', 'user1', { name: 'Alice' }, { connectionId: conn1 });

// Close a specific connection
await closeConnection(conn1);
```

### Performance Metrics

```typescript
import { getMetrics, resetMetrics } from "@debros/network";

// Get performance metrics
const metrics = getMetrics();
console.log('Operations:', metrics.operations);
console.log('Avg operation time:', metrics.performance.averageOperationTime, 'ms');
console.log('Cache hits/misses:', metrics.cacheStats);

// Reset metrics (e.g., after deployment)
resetMetrics();
```

## API Reference

### Core Database Operations

- `initDB(connectionId?: string): Promise<string>` - Initialize the database
- `create<T>(collection, id, data, options?): Promise<CreateResult>` - Create a document
- `get<T>(collection, id, options?): Promise<T | null>` - Get a document by ID
- `update<T>(collection, id, data, options?): Promise<UpdateResult>` - Update a document
- `remove(collection, id, options?): Promise<boolean>` - Delete a document
- `list<T>(collection, options?): Promise<PaginatedResult<T>>` - List documents with pagination
- `query<T>(collection, filter, options?): Promise<PaginatedResult<T>>` - Query documents
- `stopDB(): Promise<void>` - Stop the database service

### Store Types

- `StoreType.KEYVALUE` - Key-value pair storage (default)
- `StoreType.DOCSTORE` - Document storage with indexing
- `StoreType.FEED` - Append-only log
- `StoreType.EVENTLOG` - Alias for FEED
- `StoreType.COUNTER` - Numeric counter

### Schema Validation

- `defineSchema(collection, schema): void` - Define a schema for a collection

### Transactions

- `createTransaction(connectionId?): Transaction` - Create a new transaction
- `commitTransaction(transaction): Promise<{success, results}>` - Execute the transaction
- `Transaction.create<T>(collection, id, data): Transaction` - Add a create operation
- `Transaction.update<T>(collection, id, data): Transaction` - Add an update operation
- `Transaction.delete(collection, id): Transaction` - Add a delete operation

### Subscriptions

- `subscribe(event, callback): () => void` - Subscribe to events, returns unsubscribe function

### File Operations

- `uploadFile(fileData, options?): Promise<FileUploadResult>` - Upload a file
- `getFile(cid, options?): Promise<FileResult>` - Get a file by CID
- `deleteFile(cid, options?): Promise<boolean>` - Delete a file

### Connection Management

- `closeConnection(connectionId): Promise<boolean>` - Close a specific connection

### Indexes and Performance

- `createIndex(collection, field, options?): Promise<boolean>` - Create an index
- `getMetrics(): Metrics` - Get performance metrics
- `resetMetrics(): void` - Reset performance metrics

## Configuration

```typescript
import { config, initDB } from "@debros/network";

// Configure (optional)
config.env.fingerprint = "my-unique-app-id";
config.env.port = 9000;
config.ipfs.blockstorePath = "./custom-path/blockstore";
config.orbitdb.directory = "./custom-path/orbitdb";

// Initialize with configuration
await initDB();
```