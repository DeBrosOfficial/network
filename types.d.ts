// Type definitions for @debros/network
// Project: https://github.com/debros/anchat-relay
// Definitions by: Debros Team

declare module "@debros/network" {
  import { Request, Response, NextFunction } from "express";

  // Config types
  export interface DebrosConfig {
    env: {
      fingerprint: string;
      port: number;
    };
    ipfs: {
      swarm: {
        port: number;
        announceAddresses: string[];
        listenAddresses: string[];
        connectAddresses: string[];
      };
      blockstorePath: string;
      bootstrap: string[];
      privateKey?: string;
      serviceDiscovery?: {
        topic: string;
        heartbeatInterval: number;
      };
    };
    orbitdb: {
      directory: string;
    };
    logger: {
      level: string;
      file?: string;
    };
  }

  export interface ValidationResult {
    valid: boolean;
    errors?: string[];
  }

  // Core configuration
  export const config: DebrosConfig;
  export const defaultConfig: DebrosConfig;
  export function validateConfig(config: Partial<DebrosConfig>): ValidationResult;

  // Store types
  export enum StoreType {
    KEYVALUE = 'keyvalue',
    DOCSTORE = 'docstore',
    FEED = 'feed',
    EVENTLOG = 'eventlog',
    COUNTER = 'counter'
  }

  // Error handling
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
    STORE_TYPE_ERROR = 'ERR_STORE_TYPE'
  }

  export class DBError extends Error {
    code: ErrorCode;
    details?: any;
    constructor(code: ErrorCode, message: string, details?: any);
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

  // Database types
  export interface DocumentMetadata {
    createdAt: number;
    updatedAt: number;
  }

  export interface Document extends DocumentMetadata {
    [key: string]: any;
  }

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

  export interface ListOptions {
    limit?: number;
    offset?: number;
    sort?: { field: string; order: 'asc' | 'desc' };
    connectionId?: string;
    storeType?: StoreType;
  }

  export interface QueryOptions extends ListOptions {
    indexBy?: string;
  }

  export interface PaginatedResult<T> {
    documents: T[];
    total: number;
    hasMore: boolean;
  }

  // Transaction API
  export class Transaction {
    create<T>(collection: string, id: string, data: T): Transaction;
    update<T>(collection: string, id: string, data: Partial<T>): Transaction;
    delete(collection: string, id: string): Transaction;
    commit(): Promise<{ success: boolean; results: any[] }>;
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
      averageOperationTime: number;
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

  // Database Operations
  export function initDB(connectionId?: string): Promise<string>;
  export function create<T extends Record<string, any>>(collection: string, id: string, data: Omit<T, 'createdAt' | 'updatedAt'>, options?: { connectionId?: string, storeType?: StoreType }): Promise<CreateResult>;
  export function get<T extends Record<string, any>>(collection: string, id: string, options?: { connectionId?: string; skipCache?: boolean, storeType?: StoreType }): Promise<T | null>;
  export function update<T extends Record<string, any>>(collection: string, id: string, data: Partial<Omit<T, 'createdAt' | 'updatedAt'>>, options?: { connectionId?: string; upsert?: boolean, storeType?: StoreType }): Promise<UpdateResult>;
  export function remove(collection: string, id: string, options?: { connectionId?: string, storeType?: StoreType }): Promise<boolean>;
  export function list<T extends Record<string, any>>(collection: string, options?: ListOptions): Promise<PaginatedResult<T>>;
  export function query<T extends Record<string, any>>(collection: string, filter: (doc: T) => boolean, options?: QueryOptions): Promise<PaginatedResult<T>>;
  
  // Schema operations
  export function defineSchema(collection: string, schema: CollectionSchema): void;
  
  // Transaction operations
  export function createTransaction(connectionId?: string): Transaction;
  export function commitTransaction(transaction: Transaction): Promise<{ success: boolean; results: any[] }>;
  
  // Index operations
  export function createIndex(collection: string, field: string, options?: { connectionId?: string, storeType?: StoreType }): Promise<boolean>;
  
  // Subscription API
  export function subscribe(event: 'document:created' | 'document:updated' | 'document:deleted', callback: (data: any) => void): () => void;
  
  // File operations
  export function uploadFile(fileData: Buffer, options?: { filename?: string; connectionId?: string; metadata?: Record<string, any>; }): Promise<FileUploadResult>;
  export function getFile(cid: string, options?: { connectionId?: string }): Promise<FileResult>;
  export function deleteFile(cid: string, options?: { connectionId?: string }): Promise<boolean>;
  
  // Connection management
  export function closeConnection(connectionId: string): Promise<boolean>;
  
  // Metrics
  export function getMetrics(): Metrics;
  export function resetMetrics(): void;
  
  // Stop
  export function stopDB(): Promise<void>;

  // Logger
  export interface LoggerOptions {
    level?: string;
    file?: string;
    service?: string;
  }
  export const logger: any;
  export function createServiceLogger(name: string, options?: LoggerOptions): any;
  export function createDebrosLogger(options?: LoggerOptions): any;

  // Default export
  const defaultExport: {
    config: DebrosConfig;
    validateConfig: typeof validateConfig;
    db: {
      init: typeof initDB;
      create: typeof create;
      get: typeof get;
      update: typeof update;
      remove: typeof remove;
      list: typeof list;
      query: typeof query;
      createIndex: typeof createIndex;
      createTransaction: typeof createTransaction;
      commitTransaction: typeof commitTransaction;
      subscribe: typeof subscribe;
      uploadFile: typeof uploadFile;
      getFile: typeof getFile;
      deleteFile: typeof deleteFile;
      defineSchema: typeof defineSchema;
      getMetrics: typeof getMetrics;
      resetMetrics: typeof resetMetrics;
      closeConnection: typeof closeConnection;
      stop: typeof stopDB;
      ErrorCode: typeof ErrorCode;
      StoreType: typeof StoreType;
    };
    logger: any;
    createServiceLogger: typeof createServiceLogger;
  };
  export default defaultExport;
}