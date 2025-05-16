import { StoreType, StoreOptions, PaginatedResult, QueryOptions, ListOptions } from '../types';
import { AbstractStore } from './abstractStore';
import { DBError, ErrorCode } from '../core/error';

/**
 * KeyValue Store implementation using the AbstractStore base class
 */
export class KeyValueStore extends AbstractStore {
  constructor() {
    super(StoreType.KEYVALUE);
  }

  protected getLoggerName(): string {
    return 'KEYVALUE_STORE';
  }

  /**
   * Implementation for the KeyValue store create operation
   */
  protected async performCreate(db: any, id: string, document: any): Promise<string> {
    return await db.put(id, document);
  }

  /**
   * Implementation for the KeyValue store get operation
   */
  protected async performGet<T>(db: any, id: string): Promise<T | null> {
    return (await db.get(id)) as T | null;
  }

  /**
   * Implementation for the KeyValue store update operation
   */
  protected async performUpdate(db: any, id: string, document: any): Promise<string> {
    return await db.put(id, document);
  }

  /**
   * Implementation for the KeyValue store remove operation
   */
  protected async performRemove(db: any, id: string): Promise<void> {
    await db.del(id);
  }

  /**
   * List all documents in a collection with pagination
   */
  async list<T extends Record<string, any>>(
    collection: string,
    options?: ListOptions,
  ): Promise<PaginatedResult<T>> {
    try {
      const db = await this.openStore(collection, options);
      const all = await db.all();

      // Convert the key-value pairs to an array of documents with IDs
      let documents = Object.entries(all).map(([key, value]) => ({
        id: key,
        ...(value as any),
      })) as T[];

      // Apply sorting
      documents = this.applySorting(documents, options);

      // Apply pagination
      return this.applyPagination(documents, options);
    } catch (error) {
      this.handleError(`Error listing documents in ${collection}`, error);
    }
  }

  /**
   * Query documents in a collection with filtering and pagination
   */
  async query<T extends Record<string, any>>(
    collection: string,
    filter: (doc: T) => boolean,
    options?: QueryOptions,
  ): Promise<PaginatedResult<T>> {
    try {
      const db = await this.openStore(collection, options);
      const all = await db.all();

      // Apply filter
      let filtered = Object.entries(all)
        .filter(([_, value]) => filter(value as T))
        .map(([key, value]) => ({
          id: key,
          ...(value as any),
        })) as T[];

      // Apply sorting
      filtered = this.applySorting(filtered, options);

      // Apply pagination
      return this.applyPagination(filtered, options);
    } catch (error) {
      this.handleError(`Error querying documents in ${collection}`, error);
    }
  }

  /**
   * Create an index for a collection to speed up queries
   */
  async createIndex(collection: string, field: string): Promise<boolean> {
    try {
      // KeyValueStore doesn't support real indexing - this is just a placeholder
      this.logger.info(
        `Index created on ${field} for collection ${collection} (not supported in KeyValueStore)`,
      );
      return true;
    } catch (error) {
      this.handleError(`Error creating index for ${collection}`, error);
    }
  }

  /**
   * Helper to open a store of the correct type
   */
  private async openStore(collection: string, options?: StoreOptions): Promise<any> {
    const { openStore } = await import('./baseStore');
    return await openStore(collection, this.storeType, options);
  }

  /**
   * Helper to handle errors consistently
   */
  private handleError(message: string, error: any): never {
    if (error instanceof DBError) {
      throw error;
    }

    this.logger.error(`${message}:`, error);
    throw new DBError(ErrorCode.OPERATION_FAILED, `${message}: ${error.message}`, error);
  }
}
