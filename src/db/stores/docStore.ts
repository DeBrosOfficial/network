import { StoreType, StoreOptions, PaginatedResult, QueryOptions, ListOptions } from '../types';
import { AbstractStore } from './abstractStore';
import { prepareDocument } from './baseStore';

/**
 * DocStore implementation
 * Uses OrbitDB's document store which allows for more complex document storage with indices
 */
export class DocStore extends AbstractStore {
  constructor() {
    super(StoreType.DOCSTORE);
  }

  protected getLoggerName(): string {
    return 'DOCSTORE';
  }

  /**
   * Prepare a document for creation - override to add _id which is required for docstore
   */
  protected prepareCreateDocument<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Omit<T, 'createdAt' | 'updatedAt'>,
  ): any {
    return {
      _id: id,
      ...prepareDocument<T>(collection, data),
    };
  }

  /**
   * Prepare a document for update - override to add _id which is required for docstore
   */
  protected prepareUpdateDocument<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Partial<Omit<T, 'createdAt' | 'updatedAt'>>,
    existing?: T,
  ): any {
    return {
      _id: id,
      ...prepareDocument<T>(
        collection,
        data as unknown as Omit<T, 'createdAt' | 'updatedAt'>,
        existing,
      ),
    };
  }

  /**
   * Implementation for the DocStore create operation
   */
  protected async performCreate(db: any, id: string, document: any): Promise<string> {
    return await db.put(document);
  }

  /**
   * Implementation for the DocStore get operation
   */
  protected async performGet<T>(db: any, id: string): Promise<T | null> {
    return (await db.get(id)) as T | null;
  }

  /**
   * Implementation for the DocStore update operation
   */
  protected async performUpdate(db: any, id: string, document: any): Promise<string> {
    return await db.put(document);
  }

  /**
   * Implementation for the DocStore remove operation
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
      const allDocs = await db.query((_doc: any) => true);

      // Map the documents to include id
      let documents = allDocs.map((doc: any) => ({
        id: doc._id,
        ...doc,
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

      // Apply filter using docstore's query capability
      const filtered = await db.query((doc: any) => filter(doc as T));

      // Map the documents to include id
      let documents = filtered.map((doc: any) => ({
        id: doc._id,
        ...doc,
      })) as T[];

      // Apply sorting
      documents = this.applySorting(documents, options);

      // Apply pagination
      return this.applyPagination(documents, options);
    } catch (error) {
      this.handleError(`Error querying documents in ${collection}`, error);
    }
  }

  /**
   * Create an index for a collection to speed up queries
   * DocStore has built-in indexing capabilities
   */
  async createIndex(collection: string, field: string, options?: StoreOptions): Promise<boolean> {
    try {
      const db = await this.openStore(collection, options);

      // DocStore supports indexing, so we create the index
      if (typeof db.createIndex === 'function') {
        await db.createIndex(field);
        this.logger.info(`Index created on ${field} for collection ${collection}`);
        return true;
      }

      this.logger.info(
        `Index creation not supported for this DB instance, but DocStore has built-in indices`,
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
    const { DBError, ErrorCode } = require('../core/error');

    if (error instanceof DBError) {
      throw error;
    }

    this.logger.error(`${message}:`, error);
    throw new DBError(ErrorCode.OPERATION_FAILED, `${message}: ${error.message}`, error);
  }
}
