import { createServiceLogger } from '../../utils/logger';
import { ErrorCode, StoreType, StoreOptions, CreateResult, UpdateResult, PaginatedResult, QueryOptions, ListOptions } from '../types';
import { DBError } from '../core/error';
import { BaseStore, openStore, prepareDocument } from './baseStore';
import * as cache from '../cache/cacheService';
import * as events from '../events/eventService';
import { measurePerformance } from '../metrics/metricsService';

const logger = createServiceLogger('DOCSTORE');

/**
 * DocStore implementation
 * Uses OrbitDB's document store which allows for more complex document storage with indices
 */
export class DocStore implements BaseStore {
  /**
   * Create a new document in the specified collection
   */
  async create<T extends Record<string, any>>(
    collection: string, 
    id: string, 
    data: Omit<T, 'createdAt' | 'updatedAt'>, 
    options?: StoreOptions
  ): Promise<CreateResult> {
    return measurePerformance(async () => {
      try {
        const db = await openStore(collection, StoreType.DOCSTORE, options);
        
        // Prepare document for storage (including _id which is required for docstore)
        const document = {
          _id: id,
          ...prepareDocument<T>(collection, data)
        };
        
        // Add to database
        const hash = await db.put(document);
        
        // Add to cache
        const cacheKey = `${collection}:${id}`;
        cache.set(cacheKey, document);
        
        // Emit change event
        events.emit('document:created', { collection, id, document });
        
        logger.info(`Created document in ${collection} with id ${id}`);
        return { id, hash };
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error creating document in ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to create document in ${collection}`, error);
      }
    });
  }
  
  /**
   * Get a document by ID from a collection
   */
  async get<T extends Record<string, any>>(
    collection: string, 
    id: string, 
    options?: StoreOptions & { skipCache?: boolean }
  ): Promise<T | null> {
    return measurePerformance(async () => {
      try {
        // Check cache first if not skipped
        const cacheKey = `${collection}:${id}`;
        if (!options?.skipCache) {
          const cachedDocument = cache.get<T>(cacheKey);
          if (cachedDocument) {
            return cachedDocument;
          }
        }
        
        const db = await openStore(collection, StoreType.DOCSTORE, options);
        const document = await db.get(id) as T | null;
        
        // Update cache if document exists
        if (document) {
          cache.set(cacheKey, document);
        }
        
        return document;
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error getting document ${id} from ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to get document ${id} from ${collection}`, error);
      }
    });
  }
  
  /**
   * Update a document in a collection
   */
  async update<T extends Record<string, any>>(
    collection: string, 
    id: string, 
    data: Partial<Omit<T, 'createdAt' | 'updatedAt'>>, 
    options?: StoreOptions & { upsert?: boolean }
  ): Promise<UpdateResult> {
    return measurePerformance(async () => {
      try {
        const db = await openStore(collection, StoreType.DOCSTORE, options);
        const existing = await db.get(id) as T | null;
        
        if (!existing && !options?.upsert) {
          throw new DBError(
            ErrorCode.DOCUMENT_NOT_FOUND,
            `Document ${id} not found in ${collection}`,
            { collection, id }
          );
        }
        
        // Prepare document for update
        const document = {
          _id: id,
          ...prepareDocument<T>(collection, data as unknown as Omit<T, "createdAt" | "updatedAt">, existing || undefined)
        };
        
        // Update in database
        const hash = await db.put(document);
        
        // Update cache
        const cacheKey = `${collection}:${id}`;
        cache.set(cacheKey, document);
        
        // Emit change event
        events.emit('document:updated', { collection, id, document, previous: existing });
        
        logger.info(`Updated document in ${collection} with id ${id}`);
        return { id, hash };
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error updating document in ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to update document in ${collection}`, error);
      }
    });
  }
  
  /**
   * Delete a document from a collection
   */
  async remove(
    collection: string, 
    id: string, 
    options?: StoreOptions
  ): Promise<boolean> {
    return measurePerformance(async () => {
      try {
        const db = await openStore(collection, StoreType.DOCSTORE, options);
        
        // Get the document before deleting for the event
        const document = await db.get(id);
        
        // Delete from database
        await db.del(id);
        
        // Remove from cache
        const cacheKey = `${collection}:${id}`;
        cache.del(cacheKey);
        
        // Emit change event
        events.emit('document:deleted', { collection, id, document });
        
        logger.info(`Deleted document in ${collection} with id ${id}`);
        return true;
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error deleting document in ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to delete document in ${collection}`, error);
      }
    });
  }
  
  /**
   * List all documents in a collection with pagination
   */
  async list<T extends Record<string, any>>(
    collection: string, 
    options?: ListOptions
  ): Promise<PaginatedResult<T>> {
    return measurePerformance(async () => {
      try {
        const db = await openStore(collection, StoreType.DOCSTORE, options);
        const allDocs = await db.query((doc: any) => true);
        
        let documents = allDocs.map((doc: any) => ({
          id: doc._id,
          ...doc
        })) as T[];
        
        // Sort if requested
        if (options?.sort) {
          const { field, order } = options.sort;
          documents.sort((a, b) => {
            const valueA = a[field];
            const valueB = b[field];
            
            // Handle different data types for sorting
            if (typeof valueA === 'string' && typeof valueB === 'string') {
              return order === 'asc' ? valueA.localeCompare(valueB) : valueB.localeCompare(valueA);
            } else if (typeof valueA === 'number' && typeof valueB === 'number') {
              return order === 'asc' ? valueA - valueB : valueB - valueA;
            } else if (valueA instanceof Date && valueB instanceof Date) {
              return order === 'asc' ? valueA.getTime() - valueB.getTime() : valueB.getTime() - valueA.getTime();
            }
            
            // Default comparison for other types
            return order === 'asc' ? 
              String(valueA).localeCompare(String(valueB)) : 
              String(valueB).localeCompare(String(valueA));
          });
        }
        
        const total = documents.length;
        
        // Apply pagination
        const offset = options?.offset || 0;
        const limit = options?.limit || total;
        
        const paginatedDocuments = documents.slice(offset, offset + limit);
        const hasMore = offset + limit < total;
        
        return {
          documents: paginatedDocuments,
          total,
          hasMore
        };
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error listing documents in ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to list documents in ${collection}`, error);
      }
    });
  }
  
  /**
   * Query documents in a collection with filtering and pagination
   */
  async query<T extends Record<string, any>>(
    collection: string, 
    filter: (doc: T) => boolean,
    options?: QueryOptions
  ): Promise<PaginatedResult<T>> {
    return measurePerformance(async () => {
      try {
        const db = await openStore(collection, StoreType.DOCSTORE, options);
        
        // Apply filter using docstore's query capability
        const filtered = await db.query((doc: any) => filter(doc as T));
        
        // Map the documents to include id
        let documents = filtered.map((doc: any) => ({
          id: doc._id,
          ...doc
        })) as T[];
        
        // Sort if requested
        if (options?.sort) {
          const { field, order } = options.sort;
          documents.sort((a, b) => {
            const valueA = a[field];
            const valueB = b[field];
            
            // Handle different data types for sorting
            if (typeof valueA === 'string' && typeof valueB === 'string') {
              return order === 'asc' ? valueA.localeCompare(valueB) : valueB.localeCompare(valueA);
            } else if (typeof valueA === 'number' && typeof valueB === 'number') {
              return order === 'asc' ? valueA - valueB : valueB - valueA;
            } else if (valueA instanceof Date && valueB instanceof Date) {
              return order === 'asc' ? valueA.getTime() - valueB.getTime() : valueB.getTime() - valueA.getTime();
            }
            
            // Default comparison for other types
            return order === 'asc' ? 
              String(valueA).localeCompare(String(valueB)) : 
              String(valueB).localeCompare(String(valueA));
          });
        }
        
        const total = documents.length;
        
        // Apply pagination
        const offset = options?.offset || 0;
        const limit = options?.limit || total;
        
        const paginatedDocuments = documents.slice(offset, offset + limit);
        const hasMore = offset + limit < total;
        
        return {
          documents: paginatedDocuments,
          total,
          hasMore
        };
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error querying documents in ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to query documents in ${collection}`, error);
      }
    });
  }
  
  /**
   * Create an index for a collection to speed up queries
   * DocStore has built-in indexing capabilities
   */
  async createIndex(
    collection: string,
    field: string,
    options?: StoreOptions
  ): Promise<boolean> {
    try {
      const db = await openStore(collection, StoreType.DOCSTORE, options);
      
      // DocStore supports indexing, so we create the index
      if (typeof db.createIndex === 'function') {
        await db.createIndex(field);
        logger.info(`Index created on ${field} for collection ${collection}`);
        return true;
      }
      
      logger.info(`Index creation not supported for this DB instance, but DocStore has built-in indices`);
      return true;
    } catch (error) {
      if (error instanceof DBError) {
        throw error;
      }
      
      logger.error(`Error creating index for ${collection}:`, error);
      throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to create index for ${collection}`, error);
    }
  }
}