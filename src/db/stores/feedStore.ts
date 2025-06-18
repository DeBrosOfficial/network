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
} from '../types';
import { DBError } from '../core/error';
import { BaseStore, openStore, prepareDocument } from './baseStore';
import * as events from '../events/eventService';

const logger = createServiceLogger('FEED_STORE');

/**
 * FeedStore/EventLog implementation
 * Uses OrbitDB's feed/eventlog store which is an append-only log
 */
export class FeedStore implements BaseStore {
  /**
   * Create a new document in the specified collection
   * For feeds, this appends a new entry
   */
  async create<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Omit<T, 'createdAt' | 'updatedAt'>,
    options?: StoreOptions,
  ): Promise<CreateResult> {
    try {
      const db = await openStore(collection, StoreType.FEED, options);

      // Prepare document for storage with ID
      const document = {
        id,
        ...prepareDocument<T>(collection, data),
      };

      // Add to database
      const hash = await db.add(document);

      // Emit change event
      events.emit('document:created', { collection, id, document, hash });

      logger.info(`Created entry in feed ${collection} with id ${id} and hash ${hash}`);
      return { id, hash };
    } catch (error: unknown) {
      if (error instanceof DBError) {
        throw error;
      }

      logger.error(`Error creating entry in feed ${collection}:`, error);
      throw new DBError(
        ErrorCode.OPERATION_FAILED,
        `Failed to create entry in feed ${collection}`,
        error,
      );
    }
  }

  /**
   * Get a specific entry in a feed - note this works differently than other stores
   * as feeds are append-only logs identified by hash
   */
  async get<T extends Record<string, any>>(
    collection: string,
    hash: string,
    options?: StoreOptions & { skipCache?: boolean },
  ): Promise<T | null> {
    try {
      const db = await openStore(collection, StoreType.FEED, options);

      // Get the specific entry by hash
      const entry = await db.get(hash);
      if (!entry) {
        return null;
      }

      const document = entry.payload.value as T;

      return document;
    } catch (error: unknown) {
      if (error instanceof DBError) {
        throw error;
      }

      logger.error(`Error getting entry ${hash} from feed ${collection}:`, error);
      throw new DBError(
        ErrorCode.OPERATION_FAILED,
        `Failed to get entry ${hash} from feed ${collection}`,
        error,
      );
    }
  }

  /**
   * Update an entry in a feed
   * Note: Feeds are append-only, so we can't actually update existing entries
   * Instead, we append a new entry with the updated data and link it to the original
   */
  async update<T extends Record<string, any>>(
    collection: string,
    id: string,
    data: Partial<Omit<T, 'createdAt' | 'updatedAt'>>,
    options?: StoreOptions & { upsert?: boolean },
  ): Promise<UpdateResult> {
    try {
      const db = await openStore(collection, StoreType.FEED, options);

      // Get all entries using proper iterator API
      const entries = [];
      for await (const entry of db.iterator({ limit: -1 })) {
        entries.push(entry);
      }

      const existingEntryIndex = entries.findIndex((e: any) => {
        const value = e.payload.value;
        return value && value.id === id;
      });

      if (existingEntryIndex === -1 && !options?.upsert) {
        throw new DBError(
          ErrorCode.DOCUMENT_NOT_FOUND,
          `Entry with id ${id} not found in feed ${collection}`,
          { collection, id },
        );
      }

      const existingEntry =
        existingEntryIndex !== -1 ? entries[existingEntryIndex].payload.value : null;

      // Prepare document with update
      const document = {
        id,
        ...prepareDocument<T>(
          collection,
          data as unknown as Omit<T, 'createdAt' | 'updatedAt'>,
          existingEntry,
        ),
        // Add reference to the previous entry if it exists
        previousEntryHash: existingEntryIndex !== -1 ? entries[existingEntryIndex].hash : undefined,
      };

      // Add to feed (append new entry)
      const hash = await db.add(document);

      // Emit change event
      events.emit('document:updated', { collection, id, document, previous: existingEntry });

      logger.info(`Updated entry in feed ${collection} with id ${id} (new hash: ${hash})`);
      return { id, hash };
    } catch (error: unknown) {
      if (error instanceof DBError) {
        throw error;
      }

      logger.error(`Error updating entry in feed ${collection}:`, error);
      throw new DBError(
        ErrorCode.OPERATION_FAILED,
        `Failed to update entry in feed ${collection}`,
        error,
      );
    }
  }

  /**
   * Delete is not supported in feed/eventlog stores since they're append-only
   * Instead, we add a "tombstone" entry that marks the entry as deleted
   */
  async remove(collection: string, id: string, options?: StoreOptions): Promise<boolean> {
    try {
      const db = await openStore(collection, StoreType.FEED, options);

      // Find the entry with the given id using proper iterator API
      const entries = [];
      for await (const entry of db.iterator({ limit: -1 })) {
        entries.push(entry);
      }

      const existingEntryIndex = entries.findIndex((e: any) => {
        const value = e.payload.value;
        return value && value.id === id;
      });

      if (existingEntryIndex === -1) {
        throw new DBError(
          ErrorCode.DOCUMENT_NOT_FOUND,
          `Entry with id ${id} not found in feed ${collection}`,
          { collection, id },
        );
      }

      const existingEntry = entries[existingEntryIndex].payload.value;
      const existingHash = entries[existingEntryIndex].hash;

      // Add a "tombstone" entry that marks this as deleted
      const tombstone = {
        id,
        deleted: true,
        deletedAt: Date.now(),
        previousEntryHash: existingHash,
      };

      await db.add(tombstone);

      // Emit change event
      events.emit('document:deleted', { collection, id, document: existingEntry });

      logger.info(`Marked entry as deleted in feed ${collection} with id ${id}`);
      return true;
    } catch (error: unknown) {
      if (error instanceof DBError) {
        throw error;
      }

      logger.error(`Error marking entry as deleted in feed ${collection}:`, error);
      throw new DBError(
        ErrorCode.OPERATION_FAILED,
        `Failed to mark entry as deleted in feed ${collection}`,
        error,
      );
    }
  }

  /**
   * List all entries in a feed with pagination
   * Note: This will only return the latest entry for each unique ID
   */
  async list<T extends Record<string, any>>(
    collection: string,
    options?: ListOptions,
  ): Promise<PaginatedResult<T>> {
    try {
      const db = await openStore(collection, StoreType.FEED, options);

      // Use proper pagination instead of loading everything
      const requestedLimit = options?.limit || 50;
      const requestedOffset = options?.offset || 0;

      // For feeds, we need to get more entries than requested since we'll filter duplicates
      // Use a reasonable multiplier but cap it to prevent memory issues
      const fetchLimit = requestedLimit === -1 ? -1 : Math.min(requestedLimit * 3, 1000);

      // Get entries using proper iterator API with pagination
      const entries = [];
      let count = 0;
      let skipped = 0;

      for await (const entry of db.iterator({ limit: fetchLimit })) {
        // Skip entries for offset
        if (requestedOffset > 0 && skipped < requestedOffset) {
          skipped++;
          continue;
        }

        entries.push(entry);
        count++;

        // Break if we have enough entries and not requesting all
        if (requestedLimit !== -1 && count >= fetchLimit) {
          break;
        }
      }

      // Group by ID and keep only the latest entry for each ID
      // Also filter out tombstone entries
      const latestEntries = new Map<string, any>();
      for (const entry of entries) {
        // Handle different possible entry structures
        let value;
        if (entry && entry.payload && entry.payload.value) {
          value = entry.payload.value;
        } else if (entry && entry.value) {
          value = entry.value;
        } else if (entry && typeof entry === 'object') {
          value = entry;
        } else {
          continue;
        }

        if (!value || value.deleted) continue;

        const id = value.id;
        if (!id) continue;

        // If we already have an entry with this ID, check which is newer
        if (latestEntries.has(id)) {
          const existing = latestEntries.get(id);
          const existingTime = existing.value.updatedAt || existing.value.timestamp || 0;
          const currentTime = value.updatedAt || value.timestamp || 0;
          if (currentTime > existingTime) {
            latestEntries.set(id, { hash: entry.hash, value });
          }
        } else {
          latestEntries.set(id, { hash: entry.hash, value });
        }
      }

      // Convert to array of documents
      let documents = Array.from(latestEntries.values()).map((entry) => ({
        ...entry.value,
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

      // Apply final pagination to the processed results
      const total = documents.length;
      const finalLimit = requestedLimit === -1 ? total : requestedLimit;
      const paginatedDocuments = documents.slice(0, finalLimit);
      const hasMore = documents.length > finalLimit;

      return {
        documents: paginatedDocuments,
        total,
        hasMore,
      };
    } catch (error: unknown) {
      if (error instanceof DBError) {
        throw error;
      }

      logger.error(`Error listing entries in feed ${collection}:`, error);
      throw new DBError(
        ErrorCode.OPERATION_FAILED,
        `Failed to list entries in feed ${collection}`,
        error,
      );
    }
  }

  /**
   * Query entries in a feed with filtering and pagination
   * Note: This queries the latest entry for each unique ID
   */
  async query<T extends Record<string, any>>(
    collection: string,
    filter: (doc: T) => boolean,
    options?: QueryOptions,
  ): Promise<PaginatedResult<T>> {
    try {
      const db = await openStore(collection, StoreType.FEED, options);

      // Get all entries using proper iterator API
      const entries = [];
      for await (const entry of db.iterator({ limit: -1 })) {
        entries.push(entry);
      }

      // Group by ID and keep only the latest entry for each ID
      // Also filter out tombstone entries
      const latestEntries = new Map<string, any>();
      for (const entry of entries) {
        // Handle different possible entry structures
        let value;
        if (entry && entry.payload && entry.payload.value) {
          value = entry.payload.value;
        } else if (entry && entry.value) {
          value = entry.value;
        } else if (entry && typeof entry === 'object') {
          value = entry;
        } else {
          continue;
        }

        if (!value || value.deleted) continue;

        const id = value.id;
        if (!id) continue;

        // If we already have an entry with this ID, check which is newer
        if (latestEntries.has(id)) {
          const existing = latestEntries.get(id);
          if (value.updatedAt > existing.value.updatedAt) {
            latestEntries.set(id, { hash: entry.hash, value });
          }
        } else {
          latestEntries.set(id, { hash: entry.hash, value });
        }
      }

      // Convert to array of documents and apply filter
      let filtered = Array.from(latestEntries.values())
        .filter((entry) => filter(entry.value as T))
        .map((entry) => ({
          ...entry.value,
        })) as T[];

      // Sort if requested
      if (options?.sort) {
        const { field, order } = options.sort;
        filtered.sort((a, b) => {
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

      // Apply pagination
      const total = filtered.length;
      const offset = options?.offset || 0;
      const limit = options?.limit || total;

      const paginatedDocuments = filtered.slice(offset, offset + limit);
      const hasMore = offset + limit < total;

      return {
        documents: paginatedDocuments,
        total,
        hasMore,
      };
    } catch (error: unknown) {
      if (error instanceof DBError) {
        throw error;
      }

      logger.error(`Error querying entries in feed ${collection}:`, error);
      throw new DBError(
        ErrorCode.OPERATION_FAILED,
        `Failed to query entries in feed ${collection}`,
        error,
      );
    }
  }

  /**
   * Create an index for a collection - not supported for feeds
   */
  async createIndex(collection: string, _field: string, _options?: StoreOptions): Promise<boolean> {
    logger.warn(
      `Index creation not supported for feed collections, ignoring request for ${collection}`,
    );
    return false;
  }
}
