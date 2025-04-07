import { createServiceLogger } from '../utils/logger';
import { init, getConnection, closeConnection, stop } from './core/connection';
import { defineSchema, validateDocument } from './schema/validator';
import * as cache from './cache/cacheService';
import * as events from './events/eventService';
import { getMetrics, resetMetrics } from './metrics/metricsService';
import { Transaction } from './transactions/transactionService';
import { StoreType, CreateResult, UpdateResult, PaginatedResult, QueryOptions, ListOptions, ErrorCode } from './types';
import { DBError } from './core/error';
import { getStore } from './stores/storeFactory';
import { uploadFile, getFile, deleteFile } from './stores/fileStore';

// Re-export imported functions
export { init, closeConnection, stop, defineSchema, getMetrics, resetMetrics, uploadFile, getFile, deleteFile };

const logger = createServiceLogger('DB_SERVICE');

/**
 * Create a new transaction for batching operations
 */
export const createTransaction = (connectionId?: string): Transaction => {
  return new Transaction(connectionId);
};

/**
 * Execute all operations in a transaction
 */
export const commitTransaction = async (transaction: Transaction): Promise<{ success: boolean; results: any[] }> => {
  try {
    // Validate that we have operations
    const operations = transaction.getOperations();
    if (operations.length === 0) {
      return { success: true, results: [] };
    }
    
    const connectionId = transaction.getConnectionId();
    const results = [];
    
    // Execute all operations
    for (const operation of operations) {
      let result;
      
      switch (operation.type) {
        case 'create':
          result = await create(
            operation.collection, 
            operation.id, 
            operation.data, 
            { connectionId }
          );
          break;
          
        case 'update':
          result = await update(
            operation.collection, 
            operation.id, 
            operation.data, 
            { connectionId }
          );
          break;
          
        case 'delete':
          result = await remove(
            operation.collection, 
            operation.id, 
            { connectionId }
          );
          break;
      }
      
      results.push(result);
    }
    
    return { success: true, results };
  } catch (error) {
    logger.error('Transaction failed:', error);
    throw new DBError(ErrorCode.TRANSACTION_FAILED, 'Failed to commit transaction', error);
  }
};

/**
 * Create a new document in the specified collection using the appropriate store
 */
export const create = async <T extends Record<string, any>>(
  collection: string, 
  id: string, 
  data: Omit<T, 'createdAt' | 'updatedAt'>, 
  options?: { connectionId?: string, storeType?: StoreType }
): Promise<CreateResult> => {
  const storeType = options?.storeType || StoreType.KEYVALUE;
  const store = getStore(storeType);
  return store.create(collection, id, data, { connectionId: options?.connectionId });
};

/**
 * Get a document by ID from a collection
 */
export const get = async <T extends Record<string, any>>(
  collection: string, 
  id: string, 
  options?: { connectionId?: string; skipCache?: boolean, storeType?: StoreType }
): Promise<T | null> => {
  const storeType = options?.storeType || StoreType.KEYVALUE;
  const store = getStore(storeType);
  return store.get(collection, id, options);
};

/**
 * Update a document in a collection
 */
export const update = async <T extends Record<string, any>>(
  collection: string, 
  id: string, 
  data: Partial<Omit<T, 'createdAt' | 'updatedAt'>>, 
  options?: { connectionId?: string; upsert?: boolean, storeType?: StoreType }
): Promise<UpdateResult> => {
  const storeType = options?.storeType || StoreType.KEYVALUE;
  const store = getStore(storeType);
  return store.update(collection, id, data, options);
};

/**
 * Delete a document from a collection
 */
export const remove = async (
  collection: string, 
  id: string, 
  options?: { connectionId?: string, storeType?: StoreType }
): Promise<boolean> => {
  const storeType = options?.storeType || StoreType.KEYVALUE;
  const store = getStore(storeType);
  return store.remove(collection, id, options);
};

/**
 * List all documents in a collection with pagination
 */
export const list = async <T extends Record<string, any>>(
  collection: string, 
  options?: ListOptions & { storeType?: StoreType }
): Promise<PaginatedResult<T>> => {
  const storeType = options?.storeType || StoreType.KEYVALUE;
  const store = getStore(storeType);
  
  // Remove storeType from options
  const { storeType: _, ...storeOptions } = options || {};
  return store.list(collection, storeOptions);
};

/**
 * Query documents in a collection with filtering and pagination
 */
export const query = async <T extends Record<string, any>>(
  collection: string, 
  filter: (doc: T) => boolean,
  options?: QueryOptions & { storeType?: StoreType }
): Promise<PaginatedResult<T>> => {
  const storeType = options?.storeType || StoreType.KEYVALUE;
  const store = getStore(storeType);
  
  // Remove storeType from options
  const { storeType: _, ...storeOptions } = options || {};
  return store.query(collection, filter, storeOptions);
};

/**
 * Create an index for a collection to speed up queries
 */
export const createIndex = async (
  collection: string,
  field: string,
  options?: { connectionId?: string, storeType?: StoreType }
): Promise<boolean> => {
  const storeType = options?.storeType || StoreType.KEYVALUE;
  const store = getStore(storeType);
  return store.createIndex(collection, field, { connectionId: options?.connectionId });
};

/**
 * Subscribe to database events
 */
export const subscribe = events.subscribe;

// Re-export error types and codes
export { DBError } from './core/error';
export { ErrorCode } from './types';

// Export store types
export { StoreType } from './types';

export default {
  init,
  create,
  get,
  update,
  remove,
  list,
  query,
  createIndex,
  createTransaction,
  commitTransaction,
  subscribe,
  uploadFile,
  getFile,
  deleteFile,
  defineSchema,
  getMetrics,
  resetMetrics,
  closeConnection,
  stop,
  StoreType
};