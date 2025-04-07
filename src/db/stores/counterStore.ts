import { createServiceLogger } from '../../utils/logger';
import { ErrorCode, StoreType, StoreOptions, CreateResult, UpdateResult, PaginatedResult, QueryOptions, ListOptions } from '../types';
import { DBError } from '../core/error';
import { BaseStore, openStore } from './baseStore';
import * as cache from '../cache/cacheService';
import * as events from '../events/eventService';
import { measurePerformance } from '../metrics/metricsService';

const logger = createServiceLogger('COUNTER_STORE');

/**
 * CounterStore implementation
 * Uses OrbitDB's counter store for simple numeric counters
 */
export class CounterStore implements BaseStore {
  /**
   * Create or set counter value
   */
  async create<T extends Record<string, any>>(
    collection: string, 
    id: string, 
    data: Omit<T, 'createdAt' | 'updatedAt'>, 
    options?: StoreOptions
  ): Promise<CreateResult> {
    return measurePerformance(async () => {
      try {
        const db = await openStore(collection, StoreType.COUNTER, options);
        
        // Extract value from data, default to 0
        const value = typeof data === 'object' && data !== null && 'value' in data ? 
          Number(data.value) : 0;
        
        // Set the counter value
        const hash = await db.set(value);
        
        // Construct document representation
        const document = {
          id,
          value,
          createdAt: Date.now(),
          updatedAt: Date.now()
        };
        
        // Add to cache
        const cacheKey = `${collection}:${id}`;
        cache.set(cacheKey, document);
        
        // Emit change event
        events.emit('document:created', { collection, id, document });
        
        logger.info(`Set counter in ${collection} to ${value}`);
        return { id, hash };
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error setting counter in ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to set counter in ${collection}`, error);
      }
    });
  }
  
  /**
   * Get counter value
   */
  async get<T extends Record<string, any>>(
    collection: string, 
    id: string, 
    options?: StoreOptions & { skipCache?: boolean }
  ): Promise<T | null> {
    return measurePerformance(async () => {
      try {
        // Note: for counters, id is not used in the underlying store (there's only one counter per db)
        // but we use it for consistency with the API
        
        // Check cache first if not skipped
        const cacheKey = `${collection}:${id}`;
        if (!options?.skipCache) {
          const cachedDocument = cache.get<T>(cacheKey);
          if (cachedDocument) {
            return cachedDocument;
          }
        }
        
        const db = await openStore(collection, StoreType.COUNTER, options);
        
        // Get the counter value
        const value = await db.value();
        
        // Construct document representation
        const document = {
          id,
          value,
          updatedAt: Date.now()
        } as unknown as T;
        
        // Update cache
        cache.set(cacheKey, document);
        
        return document;
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error getting counter from ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to get counter from ${collection}`, error);
      }
    });
  }
  
  /**
   * Update counter (increment/decrement)
   */
  async update<T extends Record<string, any>>(
    collection: string, 
    id: string, 
    data: Partial<Omit<T, 'createdAt' | 'updatedAt'>>, 
    options?: StoreOptions & { upsert?: boolean }
  ): Promise<UpdateResult> {
    return measurePerformance(async () => {
      try {
        const db = await openStore(collection, StoreType.COUNTER, options);
        
        // Get current value before update
        const currentValue = await db.value();
        
        // Extract value from data
        let value: number;
        let operation: 'increment' | 'decrement' | 'set' = 'set';
        
        // Check what kind of operation we're doing
        if (typeof data === 'object' && data !== null) {
          if ('increment' in data) {
            value = Number(data.increment);
            operation = 'increment';
          } else if ('decrement' in data) {
            value = Number(data.decrement);
            operation = 'decrement';
          } else if ('value' in data) {
            value = Number(data.value);
            operation = 'set';
          } else {
            value = 0;
            operation = 'set';
          }
        } else {
          value = 0;
          operation = 'set';
        }
        
        // Update the counter
        let hash;
        let newValue;
        
        switch (operation) {
          case 'increment':
            hash = await db.inc(value);
            newValue = currentValue + value;
            break;
          case 'decrement':
            hash = await db.inc(-value); // Counter store uses inc with negative value
            newValue = currentValue - value;
            break;
          case 'set':
            hash = await db.set(value);
            newValue = value;
            break;
        }
        
        // Construct document representation
        const document = {
          id,
          value: newValue,
          updatedAt: Date.now()
        };
        
        // Update cache
        const cacheKey = `${collection}:${id}`;
        cache.set(cacheKey, document);
        
        // Emit change event
        events.emit('document:updated', { 
          collection, 
          id, 
          document, 
          previous: { id, value: currentValue } 
        });
        
        logger.info(`Updated counter in ${collection} from ${currentValue} to ${newValue}`);
        return { id, hash };
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error updating counter in ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to update counter in ${collection}`, error);
      }
    });
  }
  
  /**
   * Delete/reset counter
   */
  async remove(
    collection: string, 
    id: string, 
    options?: StoreOptions
  ): Promise<boolean> {
    return measurePerformance(async () => {
      try {
        const db = await openStore(collection, StoreType.COUNTER, options);
        
        // Get the current value for the event
        const currentValue = await db.value();
        
        // Reset the counter to 0 (counters can't be truly deleted)
        await db.set(0);
        
        // Remove from cache
        const cacheKey = `${collection}:${id}`;
        cache.del(cacheKey);
        
        // Emit change event
        events.emit('document:deleted', { 
          collection, 
          id, 
          document: { id, value: currentValue } 
        });
        
        logger.info(`Reset counter in ${collection} from ${currentValue} to 0`);
        return true;
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error resetting counter in ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to reset counter in ${collection}`, error);
      }
    });
  }
  
  /**
   * List all counters (for counter stores, there's only one counter per db)
   */
  async list<T extends Record<string, any>>(
    collection: string, 
    options?: ListOptions
  ): Promise<PaginatedResult<T>> {
    return measurePerformance(async () => {
      try {
        const db = await openStore(collection, StoreType.COUNTER, options);
        const value = await db.value();
        
        // For counter stores, we just return one document with the counter value
        const document = {
          id: '0',  // Default ID since counters don't have IDs
          value,
          updatedAt: Date.now()
        } as unknown as T;
        
        return {
          documents: [document],
          total: 1,
          hasMore: false
        };
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error listing counter in ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to list counter in ${collection}`, error);
      }
    });
  }
  
  /**
   * Query is not applicable for counter stores, but we implement for API consistency
   */
  async query<T extends Record<string, any>>(
    collection: string, 
    filter: (doc: T) => boolean,
    options?: QueryOptions
  ): Promise<PaginatedResult<T>> {
    return measurePerformance(async () => {
      try {
        const db = await openStore(collection, StoreType.COUNTER, options);
        const value = await db.value();
        
        // Create document
        const document = {
          id: '0',  // Default ID since counters don't have IDs
          value,
          updatedAt: Date.now()
        } as unknown as T;
        
        // Apply filter
        const documents = filter(document) ? [document] : [];
        
        return {
          documents,
          total: documents.length,
          hasMore: false
        };
      } catch (error) {
        if (error instanceof DBError) {
          throw error;
        }
        
        logger.error(`Error querying counter in ${collection}:`, error);
        throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to query counter in ${collection}`, error);
      }
    });
  }
  
  /**
   * Create an index - not applicable for counter stores
   */
  async createIndex(
    collection: string,
    field: string,
    options?: StoreOptions
  ): Promise<boolean> {
    logger.warn(`Index creation not supported for counter collections, ignoring request for ${collection}`);
    return false;
  }
}