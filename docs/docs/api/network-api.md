---
sidebar_position: 1
---

# API Reference Overview

The @debros/network API provides a comprehensive set of functions for building decentralized applications with familiar database operations built on OrbitDB and IPFS.

## Core Database Functions

### Primary Operations

| Function                                    | Description                    | Parameters                                                                  | Returns                       |
| ------------------------------------------- | ------------------------------ | --------------------------------------------------------------------------- | ----------------------------- |
| `initDB(connectionId?)`                     | Initialize database connection | `connectionId?: string`                                                     | `Promise<string>`             |
| `create<T>(collection, id, data, options?)` | Create a new document          | `collection: string, id: string, data: T, options?: CreateOptions`          | `Promise<CreateResult>`       |
| `get<T>(collection, id, options?)`          | Get document by ID             | `collection: string, id: string, options?: GetOptions`                      | `Promise<T \| null>`          |
| `update<T>(collection, id, data, options?)` | Update existing document       | `collection: string, id: string, data: Partial<T>, options?: UpdateOptions` | `Promise<UpdateResult>`       |
| `remove(collection, id, options?)`          | Delete document                | `collection: string, id: string, options?: RemoveOptions`                   | `Promise<boolean>`            |
| `list<T>(collection, options?)`             | List documents with pagination | `collection: string, options?: ListOptions`                                 | `Promise<PaginatedResult<T>>` |
| `query<T>(collection, filter, options?)`    | Query documents with filtering | `collection: string, filter: FilterFunction<T>, options?: QueryOptions`     | `Promise<PaginatedResult<T>>` |
| `stopDB()`                                  | Stop database service          | None                                                                        | `Promise<void>`               |

## File Operations

| Function                     | Description             | Parameters                                            | Returns                     |
| ---------------------------- | ----------------------- | ----------------------------------------------------- | --------------------------- |
| `uploadFile(data, options?)` | Upload file to IPFS     | `data: Buffer \| Uint8Array, options?: UploadOptions` | `Promise<FileUploadResult>` |
| `getFile(cid, options?)`     | Retrieve file from IPFS | `cid: string, options?: FileGetOptions`               | `Promise<FileResult>`       |
| `deleteFile(cid, options?)`  | Delete file from IPFS   | `cid: string, options?: FileDeleteOptions`            | `Promise<boolean>`          |

## Schema and Validation

| Function                           | Description              | Parameters                                     | Returns |
| ---------------------------------- | ------------------------ | ---------------------------------------------- | ------- |
| `defineSchema(collection, schema)` | Define validation schema | `collection: string, schema: SchemaDefinition` | `void`  |

## Transaction System

| Function                           | Description            | Parameters                 | Returns                      |
| ---------------------------------- | ---------------------- | -------------------------- | ---------------------------- |
| `createTransaction(connectionId?)` | Create new transaction | `connectionId?: string`    | `Transaction`                |
| `commitTransaction(transaction)`   | Execute transaction    | `transaction: Transaction` | `Promise<TransactionResult>` |

## Event System

| Function                     | Description         | Parameters                                  | Returns               |
| ---------------------------- | ------------------- | ------------------------------------------- | --------------------- |
| `subscribe(event, callback)` | Subscribe to events | `event: EventType, callback: EventCallback` | `UnsubscribeFunction` |

## Connection Management

| Function                        | Description               | Parameters             | Returns            |
| ------------------------------- | ------------------------- | ---------------------- | ------------------ |
| `closeConnection(connectionId)` | Close specific connection | `connectionId: string` | `Promise<boolean>` |

## Performance and Indexing

| Function                                   | Description           | Parameters                                                  | Returns            |
| ------------------------------------------ | --------------------- | ----------------------------------------------------------- | ------------------ |
| `createIndex(collection, field, options?)` | Create database index | `collection: string, field: string, options?: IndexOptions` | `Promise<boolean>` |

## Type Definitions

### Core Types

```typescript
interface CreateResult {
  id: string;
  success: boolean;
  timestamp: number;
}

interface UpdateResult {
  id: string;
  success: boolean;
  modified: boolean;
  timestamp: number;
}

interface PaginatedResult<T> {
  documents: T[];
  total: number;
  hasMore: boolean;
  offset: number;
  limit: number;
}

interface FileUploadResult {
  cid: string;
  size: number;
  filename?: string;
  metadata?: Record<string, any>;
}

interface FileResult {
  data: Buffer;
  metadata?: Record<string, any>;
  size: number;
}
```

### Options Types

```typescript
interface CreateOptions {
  connectionId?: string;
  validate?: boolean;
  overwrite?: boolean;
}

interface GetOptions {
  connectionId?: string;
  includeMetadata?: boolean;
}

interface UpdateOptions {
  connectionId?: string;
  validate?: boolean;
  upsert?: boolean;
}

interface ListOptions {
  connectionId?: string;
  limit?: number;
  offset?: number;
  sort?: {
    field: string;
    order: 'asc' | 'desc';
  };
}

interface QueryOptions extends ListOptions {
  includeScore?: boolean;
}

interface UploadOptions {
  filename?: string;
  metadata?: Record<string, any>;
  connectionId?: string;
}
```

### Store Types

```typescript
enum StoreType {
  KEYVALUE = 'keyvalue',
  DOCSTORE = 'docstore',
  FEED = 'feed',
  EVENTLOG = 'eventlog',
  COUNTER = 'counter',
}
```

### Event Types

```typescript
type EventType =
  | 'document:created'
  | 'document:updated'
  | 'document:deleted'
  | 'connection:established'
  | 'connection:lost';

type EventCallback = (data: EventData) => void;

interface EventData {
  collection: string;
  id: string;
  document?: any;
  timestamp: number;
}
```

## Configuration

### Environment Configuration

```typescript
import { config } from '@debros/network';

// Available configuration options
config.env.fingerprint = 'my-app-id';
config.env.port = 9000;
config.ipfs.blockstorePath = './blockstore';
config.orbitdb.directory = './orbitdb';
```

## Error Handling

### Common Error Types

```typescript
// Network errors
class NetworkError extends Error {
  code: 'NETWORK_ERROR';
  details: string;
}

// Validation errors
class ValidationError extends Error {
  code: 'VALIDATION_ERROR';
  field: string;
  value: any;
}

// Not found errors
class NotFoundError extends Error {
  code: 'NOT_FOUND';
  collection: string;
  id: string;
}
```

## Usage Examples

### Basic CRUD Operations

```typescript
import { initDB, create, get, update, remove } from '@debros/network';

// Initialize
await initDB();

// Create
const user = await create('users', 'user123', {
  username: 'alice',
  email: 'alice@example.com',
});

// Read
const retrieved = await get('users', 'user123');

// Update
await update('users', 'user123', { email: 'newemail@example.com' });

// Delete
await remove('users', 'user123');
```

### File Operations

```typescript
import { uploadFile, getFile } from '@debros/network';

// Upload file
const fileData = Buffer.from('Hello World');
const result = await uploadFile(fileData, { filename: 'hello.txt' });

// Get file
const file = await getFile(result.cid);
console.log(file.data.toString()); // "Hello World"
```

### Transaction Example

```typescript
import { createTransaction, commitTransaction } from '@debros/network';

const tx = createTransaction();
tx.create('users', 'user1', { name: 'Alice' })
  .create('posts', 'post1', { title: 'Hello', authorId: 'user1' })
  .update('users', 'user1', { postCount: 1 });

const result = await commitTransaction(tx);
```

This API reference covers all the actual functionality available in the @debros/network package. For detailed examples and guides, see the other documentation sections.
