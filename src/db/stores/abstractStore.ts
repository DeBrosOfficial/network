import { createServiceLogger } from '../../utils/logger';
import {
  ErrorCode,
  StoreType,
  StoreOptions,
  CreateResult,
  UpdateResult,
  PaginatedResult,
  QueryOptions,
  ListOptions,
  acquireLock,
  releaseLock,
  isLocked,
} from '../types';
import { DBError } from '../core/error';
import { BaseStore, openStore, prepareDocument } from './baseStore';
import * as events from '../events/eventService';

/**
 * Abstract store implementation with common CRUD operations
 * Specific store types extend this class and customize only what's different
 */
export abstract class AbstractStore implements BaseStore {
  protected logger = createServiceLogger(this.getLoggerName());
  protected storeType: StoreType;

  constructor(storeType: StoreType) {
    this.storeType = storeType;
  }

  /**
   * Must be implemented by subclasses to provide the logger name
   */
  protected abstract getLoggerName(): string;

  /**
   * Create a new document in the specified collection
   */
  async create<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Omit<T, 'createdAt' | 'updatedAt'>,
    options?: StoreOptions,
  ): Promise<CreateResult> {
    // Create a lock ID for this resource to prevent concurrent operations
    const lockId = `${collection}:${id}:create`;

    // Try to acquire a lock
    if (!acquireLock(lockId)) {
      this.logger.warn(
        `Concurrent operation detected on ${collection}/${id}, waiting for completion`,
      );
      // Wait until the lock is released (poll every 100ms for max 5 seconds)
      let attempts = 0;
      while (isLocked(lockId) && attempts < 50) {
        await new Promise((resolve) => setTimeout(resolve, 100));
        attempts++;
      }

      if (isLocked(lockId)) {
        throw new DBError(
          ErrorCode.OPERATION_FAILED,
          `Timed out waiting for lock on ${collection}/${id}`,
        );
      }

      // Try to acquire lock again
      if (!acquireLock(lockId)) {
        throw new DBError(
          ErrorCode.OPERATION_FAILED,
          `Failed to acquire lock on ${collection}/${id}`,
        );
      }
    }

    try {
      const db = await openStore(collection, this.storeType, options);

      // Prepare document for storage with validation
      const document = this.prepareCreateDocument<T>(collection, id, data);

      // Add to database - this will be overridden by specific implementations if needed
      const hash = await this.performCreate(db, id, document);

      // Emit change event
      events.emit('document:created', { collection, id, document });

      this.logger.info(`Created document in ${collection} with id ${id}`);
      return { id, hash };
    } catch (error: unknown) {
      if (error instanceof DBError) {
        throw error;
      }

      this.logger.error(`Error creating document in ${collection}:`, error);
      throw new DBError(
        ErrorCode.OPERATION_FAILED,
        `Failed to create document in ${collection}: ${error instanceof Error ? error.message : String(error)}`,
        error,
      );
    } finally {
      // Always release the lock when done
      releaseLock(lockId);
    }
  }

  /**
   * Prepare a document for creation - can be overridden by subclasses
   */
  protected prepareCreateDocument<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Omit<T, 'createdAt' | 'updatedAt'>,
  ): any {
    return prepareDocument<T>(collection, data);
  }

  /**
   * Perform the actual create operation - should be implemented by subclasses
   */
  protected abstract performCreate(db: any, id: string, document: any): Promise<string>;

  /**
   * Get a document by ID from a collection
   */
  async get<T extends Record<string, any>>(
    collection: string,
    id: string,
    options?: StoreOptions & { skipCache?: boolean },
  ): Promise<T | null> {
    try {
      const db = await openStore(collection, this.storeType, options);
      const document = await this.performGet<T>(db, id);

      return document;
    } catch (error: unknown) {
      if (error instanceof DBError) {
        throw error;
      }

      this.logger.error(`Error getting document ${id} from ${collection}:`, error);
      throw new DBError(
        ErrorCode.OPERATION_FAILED,
        `Failed to get document ${id} from ${collection}: ${error instanceof Error ? error.message : String(error)}`,
        error,
      );
    }
  }

  /**
   * Perform the actual get operation - should be implemented by subclasses
   */
  protected abstract performGet<T>(db: any, id: string): Promise<T | null>;

  /**
   * Update a document in a collection
   */
  async update<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Partial<Omit<T, 'createdAt' | 'updatedAt'>>,
    options?: StoreOptions & { upsert?: boolean },
  ): Promise<UpdateResult> {
    // Create a lock ID for this resource to prevent concurrent operations
    const lockId = `${collection}:${id}:update`;

    // Try to acquire a lock
    if (!acquireLock(lockId)) {
      this.logger.warn(
        `Concurrent operation detected on ${collection}/${id}, waiting for completion`,
      );
      // Wait until the lock is released (poll every 100ms for max 5 seconds)
      let attempts = 0;
      while (isLocked(lockId) && attempts < 50) {
        await new Promise((resolve) => setTimeout(resolve, 100));
        attempts++;
      }

      if (isLocked(lockId)) {
        throw new DBError(
          ErrorCode.OPERATION_FAILED,
          `Timed out waiting for lock on ${collection}/${id}`,
        );
      }

      // Try to acquire lock again
      if (!acquireLock(lockId)) {
        throw new DBError(
          ErrorCode.OPERATION_FAILED,
          `Failed to acquire lock on ${collection}/${id}`,
        );
      }
    }

    try {
      const db = await openStore(collection, this.storeType, options);
      const existing = await this.performGet<T>(db, id);

      if (!existing && !options?.upsert) {
        throw new DBError(
          ErrorCode.DOCUMENT_NOT_FOUND,
          `Document ${id} not found in ${collection}`,
          { collection, id },
        );
      }

      // Prepare document for update with validation
      const document = this.prepareUpdateDocument<T>(collection, id, data, existing || undefined);

      // Update in database
      const hash = await this.performUpdate(db, id, document);

      // Emit change event
      events.emit('document:updated', { collection, id, document, previous: existing });

      this.logger.info(`Updated document in ${collection} with id ${id}`);
      return { id, hash };
    } catch (error: unknown) {
      if (error instanceof DBError) {
        throw error;
      }

      this.logger.error(`Error updating document in ${collection}:`, error);
      throw new DBError(
        ErrorCode.OPERATION_FAILED,
        `Failed to update document in ${collection}: ${error instanceof Error ? error.message : String(error)}`,
        error,
      );
    } finally {
      // Always release the lock when done
      releaseLock(lockId);
    }
  }

  /**
   * Prepare a document for update - can be overridden by subclasses
   */
  protected prepareUpdateDocument<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Partial<Omit<T, 'createdAt' | 'updatedAt'>>,
    existing?: T,
  ): any {
    return prepareDocument<T>(
      collection,
      data as unknown as Omit<T, 'createdAt' | 'updatedAt'>,
      existing,
    );
  }

  /**
   * Perform the actual update operation - should be implemented by subclasses
   */
  protected abstract performUpdate(db: any, id: string, document: any): Promise<string>;

  /**
   * Delete a document from a collection
   */
  async remove(collection: string, id: string, options?: StoreOptions): Promise<boolean> {
    // Create a lock ID for this resource to prevent concurrent operations
    const lockId = `${collection}:${id}:remove`;

    // Try to acquire a lock
    if (!acquireLock(lockId)) {
      this.logger.warn(
        `Concurrent operation detected on ${collection}/${id}, waiting for completion`,
      );
      // Wait until the lock is released (poll every 100ms for max 5 seconds)
      let attempts = 0;
      while (isLocked(lockId) && attempts < 50) {
        await new Promise((resolve) => setTimeout(resolve, 100));
        attempts++;
      }

      if (isLocked(lockId)) {
        throw new DBError(
          ErrorCode.OPERATION_FAILED,
          `Timed out waiting for lock on ${collection}/${id}`,
        );
      }

      // Try to acquire lock again
      if (!acquireLock(lockId)) {
        throw new DBError(
          ErrorCode.OPERATION_FAILED,
          `Failed to acquire lock on ${collection}/${id}`,
        );
      }
    }

    try {
      const db = await openStore(collection, this.storeType, options);

      // Get the document before deleting for the event
      const document = await this.performGet(db, id);

      if (!document) {
        this.logger.warn(`Document ${id} not found in ${collection} for deletion`);
        return false;
      }

      // Delete from database
      await this.performRemove(db, id);

      // Emit change event
      events.emit('document:deleted', { collection, id, document });

      this.logger.info(`Deleted document in ${collection} with id ${id}`);
      return true;
    } catch (error: unknown) {
      if (error instanceof DBError) {
        throw error;
      }

      this.logger.error(`Error deleting document in ${collection}:`, error);
      throw new DBError(
        ErrorCode.OPERATION_FAILED,
        `Failed to delete document in ${collection}: ${error instanceof Error ? error.message : String(error)}`,
        error,
      );
    } finally {
      // Always release the lock when done
      releaseLock(lockId);
    }
  }

  /**
   * Perform the actual remove operation - should be implemented by subclasses
   */
  protected abstract performRemove(db: any, id: string): Promise<void>;

  /**
   * Apply sorting to a list of documents
   */
  protected applySorting<T extends Record<string, any>>(
    documents: T[],
    options?: ListOptions | QueryOptions,
  ): T[] {
    if (!options?.sort) {
      return documents;
    }

    const { field, order } = options.sort;

    return [...documents].sort((a, b) => {
      const valueA = a[field];
      const valueB = b[field];

      // Handle different data types for sorting
      if (typeof valueA === 'string' && typeof valueB === 'string') {
        return order === 'asc' ? valueA.localeCompare(valueB) : valueB.localeCompare(valueA);
      } else if (typeof valueA === 'number' && typeof valueB === 'number') {
        return order === 'asc' ? valueA - valueB : valueB - valueA;
      } else if (valueA instanceof Date && valueB instanceof Date) {
        return order === 'asc'
          ? valueA.getTime() - valueB.getTime()
          : valueB.getTime() - valueA.getTime();
      }

      // Default comparison for other types
      return order === 'asc'
        ? String(valueA).localeCompare(String(valueB))
        : String(valueB).localeCompare(String(valueA));
    });
  }

  /**
   * Apply pagination to a list of documents
   */
  protected applyPagination<T>(
    documents: T[],
    options?: ListOptions | QueryOptions,
  ): {
    documents: T[];
    total: number;
    hasMore: boolean;
  } {
    const total = documents.length;
    const offset = options?.offset || 0;
    const limit = options?.limit || total;

    const paginatedDocuments = documents.slice(offset, offset + limit);
    const hasMore = offset + limit < total;

    return {
      documents: paginatedDocuments,
      total,
      hasMore,
    };
  }

  /**
   * List all documents in a collection with pagination
   */
  abstract list<T extends Record<string, any>>(
    collection: string,
    options?: ListOptions,
  ): Promise<PaginatedResult<T>>;

  /**
   * Query documents in a collection with filtering and pagination
   */
  abstract query<T extends Record<string, any>>(
    collection: string,
    filter: (doc: T) => boolean,
    options?: QueryOptions,
  ): Promise<PaginatedResult<T>>;

  /**
   * Create an index for a collection to speed up queries
   */
  abstract createIndex(collection: string, field: string, options?: StoreOptions): Promise<boolean>;
}
