import { createServiceLogger } from '../../utils/logger';
import { openDB } from '../../orbit/orbitDBService';
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
  create<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Omit<T, 'createdAt' | 'updatedAt'>,
    options?: StoreOptions,
  ): Promise<CreateResult>;

  get<T extends Record<string, any>>(
    collection: string,
    id: string,
    options?: StoreOptions & { skipCache?: boolean },
  ): Promise<T | null>;

  update<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Partial<Omit<T, 'createdAt' | 'updatedAt'>>,
    options?: StoreOptions & { upsert?: boolean },
  ): Promise<UpdateResult>;

  remove(collection: string, id: string, options?: StoreOptions): Promise<boolean>;

  list<T extends Record<string, any>>(
    collection: string,
    options?: ListOptions,
  ): Promise<PaginatedResult<T>>;

  query<T extends Record<string, any>>(
    collection: string,
    filter: (doc: T) => boolean,
    options?: QueryOptions,
  ): Promise<PaginatedResult<T>>;

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
    // Log minimal connection info to avoid leaking sensitive data
    logger.info(
      `Opening ${storeType} store for collection: ${collection} (connection ID: ${options?.connectionId || 'default'})`,
    );

    return await openDB(collection, storeType).catch((err) => {
      throw new Error(`OrbitDB openDB failed: ${err.message}`);
    });
  } catch (error) {
    logger.error(`Error opening ${storeType} store for collection ${collection}:`, error);

    // Add more context to the error for improved debugging
    const errorMessage = error instanceof Error ? error.message : String(error);
    throw new DBError(
      ErrorCode.OPERATION_FAILED,
      `Failed to open ${storeType} store for collection ${collection}: ${errorMessage}`,
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

  // Prepare document for validation
  let docToValidate: T;

  // If it's an update to an existing document
  if (existingDoc) {
    docToValidate = {
      ...existingDoc,
      ...sanitizedData,
      updatedAt: timestamp,
    } as T;
  } else {
    // Otherwise it's a new document
    docToValidate = {
      ...sanitizedData,
      createdAt: timestamp,
      updatedAt: timestamp,
    } as unknown as T;
  }

  // Validate the document BEFORE processing
  validateDocument(collection, docToValidate);

  // Return the validated document
  return docToValidate;
}
