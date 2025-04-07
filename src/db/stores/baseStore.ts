import { createServiceLogger } from '../../utils/logger';
import { openDB } from '../../orbit/orbitDBService';
import { getConnection } from '../core/connection';
import { validateDocument } from '../schema/validator';
import {
  ErrorCode,
  StoreType,
  StoreOptions,
  CreateResult,
  UpdateResult,
  PaginatedResult,
  QueryOptions,
  ListOptions,
} from '../types';
import { DBError } from '../core/error';

const logger = createServiceLogger('DB_STORE');

/**
 * Base Store interface that all store implementations should extend
 */
export interface BaseStore {
  /**
   * Create a new document
   */
  create<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Omit<T, 'createdAt' | 'updatedAt'>,
    options?: StoreOptions,
  ): Promise<CreateResult>;

  /**
   * Get a document by ID
   */
  get<T extends Record<string, any>>(
    collection: string,
    id: string,
    options?: StoreOptions & { skipCache?: boolean },
  ): Promise<T | null>;

  /**
   * Update a document
   */
  update<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Partial<Omit<T, 'createdAt' | 'updatedAt'>>,
    options?: StoreOptions & { upsert?: boolean },
  ): Promise<UpdateResult>;

  /**
   * Delete a document
   */
  remove(collection: string, id: string, options?: StoreOptions): Promise<boolean>;

  /**
   * List all documents in a collection with pagination
   */
  list<T extends Record<string, any>>(
    collection: string,
    options?: ListOptions,
  ): Promise<PaginatedResult<T>>;

  /**
   * Query documents in a collection with filtering and pagination
   */
  query<T extends Record<string, any>>(
    collection: string,
    filter: (doc: T) => boolean,
    options?: QueryOptions,
  ): Promise<PaginatedResult<T>>;

  /**
   * Create an index for a collection to speed up queries
   */
  createIndex(collection: string, field: string, options?: StoreOptions): Promise<boolean>;
}

/**
 * Open a store of the specified type
 */
export async function openStore(
  collection: string,
  storeType: StoreType,
  options?: StoreOptions,
): Promise<any> {
  try {
    const connection = getConnection(options?.connectionId);
    logger.info(`Connection for ${collection}:`, connection);
    return await openDB(collection, storeType).catch((err) => {
      throw new Error(`OrbitDB openDB failed: ${err.message}`);
    });
  } catch (error) {
    logger.error(`Error opening ${storeType} store for collection ${collection}:`, error);
    throw new DBError(
      ErrorCode.OPERATION_FAILED,
      `Failed to open ${storeType} store for collection ${collection}`,
      error,
    );
  }
}

/**
 * Helper function to prepare a document for storage
 */
export function prepareDocument<T extends Record<string, any>>(
  collection: string,
  data: Omit<T, 'createdAt' | 'updatedAt'>,
  existingDoc?: T | null,
): T {
  const timestamp = Date.now();

  // Sanitize the input data by replacing undefined with null
  const sanitizedData = Object.fromEntries(
    Object.entries(data).map(([key, value]) => [key, value === undefined ? null : value]),
  ) as Omit<T, 'createdAt' | 'updatedAt'>;

  // If it's an update to an existing document
  if (existingDoc) {
    const doc = {
      ...existingDoc,
      ...sanitizedData,
      updatedAt: timestamp,
    } as T;

    // Validate the document against its schema
    validateDocument(collection, doc);
    return doc;
  }

  // Otherwise it's a new document
  const doc = {
    ...sanitizedData,
    createdAt: timestamp,
    updatedAt: timestamp,
  } as unknown as T;

  // Validate the document against its schema
  validateDocument(collection, doc);
  return doc;
}
