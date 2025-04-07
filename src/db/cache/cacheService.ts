import NodeCache from 'node-cache';
import { createServiceLogger } from '../../utils/logger';

const logger = createServiceLogger('DB_CACHE');

// Cache for frequently accessed documents
const cache = new NodeCache({
  stdTTL: 300, // 5 minutes default TTL
  checkperiod: 60, // Check for expired items every 60 seconds
  useClones: false, // Don't clone objects (for performance)
});

// Cache statistics
export const cacheStats = {
  hits: 0,
  misses: 0,
};

/**
 * Get an item from cache
 */
export const get = <T>(key: string): T | undefined => {
  const value = cache.get<T>(key);
  if (value !== undefined) {
    cacheStats.hits++;
    return value;
  }
  cacheStats.misses++;
  return undefined;
};

/**
 * Set an item in cache
 */
export const set = <T>(key: string, value: T, ttl?: number): boolean => {
  if (ttl === undefined) {
    return cache.set(key, value);
  }
  return cache.set(key, value, ttl);
};

/**
 * Delete an item from cache
 */
export const del = (key: string | string[]): number => {
  return cache.del(key);
};

/**
 * Flush the entire cache
 */
export const flushAll = (): void => {
  cache.flushAll();
};

/**
 * Reset cache statistics
 */
export const resetStats = (): void => {
  cacheStats.hits = 0;
  cacheStats.misses = 0;
};