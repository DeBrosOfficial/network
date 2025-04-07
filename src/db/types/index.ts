// Common types for database operations
import { EventEmitter } from 'events';
import { Transaction } from '../transactions/transactionService';

export type { Transaction };

// Database Types
export enum StoreType {
  KEYVALUE = 'keyvalue',
  DOCSTORE = 'docstore',
  FEED = 'feed',
  EVENTLOG = 'eventlog',
  COUNTER = 'counter'
}

// Common result types
export interface CreateResult {
  id: string;
  hash: string;
}

export interface UpdateResult {
  id: string;
  hash: string;
}

export interface FileUploadResult {
  cid: string;
}

export interface FileMetadata {
  filename?: string;
  size: number;
  uploadedAt: number;
  [key: string]: any;
}

export interface FileResult {
  data: Buffer;
  metadata: FileMetadata | null;
}

export interface PaginatedResult<T> {
  documents: T[];
  total: number;
  hasMore: boolean;
}

// Define error codes
export enum ErrorCode {
  NOT_INITIALIZED = 'ERR_NOT_INITIALIZED',
  INITIALIZATION_FAILED = 'ERR_INIT_FAILED',
  DOCUMENT_NOT_FOUND = 'ERR_DOC_NOT_FOUND',
  INVALID_SCHEMA = 'ERR_INVALID_SCHEMA',
  OPERATION_FAILED = 'ERR_OPERATION_FAILED',
  TRANSACTION_FAILED = 'ERR_TRANSACTION_FAILED',
  FILE_NOT_FOUND = 'ERR_FILE_NOT_FOUND',
  INVALID_PARAMETERS = 'ERR_INVALID_PARAMS',
  CONNECTION_ERROR = 'ERR_CONNECTION',
  STORE_TYPE_ERROR = 'ERR_STORE_TYPE',
}

// Connection pool interface
export interface DBConnection {
  ipfs: any;
  orbitdb: any;
  timestamp: number;
  isActive: boolean;
}

// Schema validation
export interface SchemaDefinition {
  type: string;
  required?: boolean;
  pattern?: string;
  min?: number;
  max?: number;
  enum?: any[];
  items?: SchemaDefinition; // For arrays
  properties?: Record<string, SchemaDefinition>; // For objects
}

export interface CollectionSchema {
  properties: Record<string, SchemaDefinition>;
  required?: string[];
}

// Metrics tracking
export interface Metrics {
  operations: {
    creates: number;
    reads: number;
    updates: number;
    deletes: number;
    queries: number;
    fileUploads: number;
    fileDownloads: number;
  };
  performance: {
    totalOperationTime: number;
    operationCount: number;
    averageOperationTime?: number;
  };
  errors: {
    count: number;
    byCode: Record<string, number>;
  };
  cacheStats: {
    hits: number;
    misses: number;
  };
  startTime: number;
}

// Store options
export interface ListOptions {
  limit?: number;
  offset?: number;
  connectionId?: string;
  sort?: { field: string; order: 'asc' | 'desc' };
}

export interface QueryOptions extends ListOptions {
  indexBy?: string;
}

export interface StoreOptions {
  connectionId?: string;
}

// Event bus for database events
export const dbEvents = new EventEmitter();