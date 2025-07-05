import { QueryBuilder } from './QueryBuilder';
import { BaseModel } from '../models/BaseModel';

export interface CacheEntry<T> {
  key: string;
  data: T[];
  timestamp: number;
  ttl: number;
  hitCount: number;
}

export interface CacheStats {
  totalRequests: number;
  cacheHits: number;
  cacheMisses: number;
  hitRate: number;
  size: number;
  maxSize: number;
}

export class QueryCache {
  private cache: Map<string, CacheEntry<any>> = new Map();
  private maxSize: number;
  private defaultTTL: number;
  private stats: CacheStats;

  constructor(maxSize: number = 1000, defaultTTL: number = 300000) {
    // 5 minutes default
    this.maxSize = maxSize;
    this.defaultTTL = defaultTTL;
    this.stats = {
      totalRequests: 0,
      cacheHits: 0,
      cacheMisses: 0,
      hitRate: 0,
      size: 0,
      maxSize,
    };
  }

  generateKey<T extends BaseModel>(query: QueryBuilder<T>): string {
    const model = query.getModel();
    const conditions = query.getConditions();
    const relations = query.getRelations();
    const sorting = query.getSorting();
    const limit = query.getLimit();
    const offset = query.getOffset();

    // Create a deterministic cache key
    const keyParts = [
      model.name,
      model.scope,
      JSON.stringify(conditions.sort((a, b) => a.field.localeCompare(b.field))),
      JSON.stringify(relations.sort()),
      JSON.stringify(sorting),
      limit?.toString() || 'no-limit',
      offset?.toString() || 'no-offset',
    ];

    // Create hash of the key parts
    return this.hashString(keyParts.join('|'));
  }

  async get<T extends BaseModel>(query: QueryBuilder<T>): Promise<T[] | null> {
    this.stats.totalRequests++;

    const key = this.generateKey(query);
    const entry = this.cache.get(key);

    if (!entry) {
      this.stats.cacheMisses++;
      this.updateHitRate();
      return null;
    }

    // Check if entry has expired
    if (Date.now() - entry.timestamp > entry.ttl) {
      this.cache.delete(key);
      this.stats.cacheMisses++;
      this.updateHitRate();
      return null;
    }

    // Update hit count and stats
    entry.hitCount++;
    this.stats.cacheHits++;
    this.updateHitRate();

    // Convert cached data back to model instances
    const modelClass = query.getModel() as any; // Type assertion for abstract class
    return entry.data.map((item) => new modelClass(item));
  }

  set<T extends BaseModel>(query: QueryBuilder<T>, data: T[], customTTL?: number): void {
    const key = this.generateKey(query);
    const ttl = customTTL || this.defaultTTL;

    // Serialize model instances to plain objects for caching
    const serializedData = data.map((item) => item.toJSON());

    const entry: CacheEntry<any> = {
      key,
      data: serializedData,
      timestamp: Date.now(),
      ttl,
      hitCount: 0,
    };

    // Check if we need to evict entries
    if (this.cache.size >= this.maxSize) {
      this.evictLeastUsed();
    }

    this.cache.set(key, entry);
    this.stats.size = this.cache.size;
  }

  invalidate<T extends BaseModel>(query: QueryBuilder<T>): boolean {
    const key = this.generateKey(query);
    const deleted = this.cache.delete(key);
    this.stats.size = this.cache.size;
    return deleted;
  }

  invalidateByModel(modelName: string): number {
    let deletedCount = 0;

    for (const [key, _entry] of this.cache.entries()) {
      if (key.startsWith(this.hashString(modelName))) {
        this.cache.delete(key);
        deletedCount++;
      }
    }

    this.stats.size = this.cache.size;
    return deletedCount;
  }

  invalidateByUser(userId: string): number {
    let deletedCount = 0;

    for (const [key, entry] of this.cache.entries()) {
      // Check if the cached entry contains user-specific data
      if (this.entryContainsUser(entry, userId)) {
        this.cache.delete(key);
        deletedCount++;
      }
    }

    this.stats.size = this.cache.size;
    return deletedCount;
  }

  clear(): void {
    this.cache.clear();
    this.stats.size = 0;
    this.stats.totalRequests = 0;
    this.stats.cacheHits = 0;
    this.stats.cacheMisses = 0;
    this.stats.hitRate = 0;
  }

  getStats(): CacheStats {
    return { ...this.stats };
  }

  // Cache warming - preload frequently used queries
  async warmup<T extends BaseModel>(queries: QueryBuilder<T>[]): Promise<void> {
    console.log(`🔥 Warming up cache with ${queries.length} queries...`);

    const promises = queries.map(async (query) => {
      try {
        const results = await query.exec();
        this.set(query, results);
        console.log(`✓ Cached query for ${query.getModel().name}`);
      } catch (error) {
        console.warn(`Failed to warm cache for ${query.getModel().name}:`, error);
      }
    });

    await Promise.all(promises);
    console.log(`✅ Cache warmup completed`);
  }

  // Get cache entries sorted by various criteria
  getPopularEntries(limit: number = 10): Array<{ key: string; hitCount: number; age: number }> {
    return Array.from(this.cache.entries())
      .map(([key, entry]) => ({
        key,
        hitCount: entry.hitCount,
        age: Date.now() - entry.timestamp,
      }))
      .sort((a, b) => b.hitCount - a.hitCount)
      .slice(0, limit);
  }

  getExpiredEntries(): string[] {
    const now = Date.now();
    const expired: string[] = [];

    for (const [key, entry] of this.cache.entries()) {
      if (now - entry.timestamp > entry.ttl) {
        expired.push(key);
      }
    }

    return expired;
  }

  // Cleanup expired entries
  cleanup(): number {
    const expired = this.getExpiredEntries();

    for (const key of expired) {
      this.cache.delete(key);
    }

    this.stats.size = this.cache.size;
    return expired.length;
  }

  // Configure cache behavior
  setMaxSize(size: number): void {
    this.maxSize = size;
    this.stats.maxSize = size;

    // Evict entries if current size exceeds new max
    while (this.cache.size > size) {
      this.evictLeastUsed();
    }
  }

  setDefaultTTL(ttl: number): void {
    this.defaultTTL = ttl;
  }

  // Cache analysis
  analyzeUsage(): {
    totalEntries: number;
    averageHitCount: number;
    averageAge: number;
    memoryUsage: number;
  } {
    const entries = Array.from(this.cache.values());
    const now = Date.now();

    const totalHits = entries.reduce((sum, entry) => sum + entry.hitCount, 0);
    const totalAge = entries.reduce((sum, entry) => sum + (now - entry.timestamp), 0);

    // Rough memory usage estimation
    const memoryUsage = entries.reduce((sum, entry) => {
      return sum + JSON.stringify(entry.data).length;
    }, 0);

    return {
      totalEntries: entries.length,
      averageHitCount: entries.length > 0 ? totalHits / entries.length : 0,
      averageAge: entries.length > 0 ? totalAge / entries.length : 0,
      memoryUsage,
    };
  }

  private evictLeastUsed(): void {
    if (this.cache.size === 0) return;

    // Find entry with lowest hit count and oldest timestamp
    let leastUsedKey: string | null = null;
    let leastUsedScore = Infinity;

    for (const [key, entry] of this.cache.entries()) {
      // Score based on hit count and age (lower is worse)
      const age = Date.now() - entry.timestamp;
      const score = entry.hitCount - age / 1000000; // Age penalty

      if (score < leastUsedScore) {
        leastUsedScore = score;
        leastUsedKey = key;
      }
    }

    if (leastUsedKey) {
      this.cache.delete(leastUsedKey);
      this.stats.size = this.cache.size;
    }
  }

  private entryContainsUser(entry: CacheEntry<any>, userId: string): boolean {
    // Check if the cached data contains user-specific information
    try {
      const dataStr = JSON.stringify(entry.data);
      return dataStr.includes(userId);
    } catch {
      return false;
    }
  }

  private updateHitRate(): void {
    if (this.stats.totalRequests > 0) {
      this.stats.hitRate = this.stats.cacheHits / this.stats.totalRequests;
    }
  }

  private hashString(str: string): string {
    let hash = 0;
    if (str.length === 0) return hash.toString();

    for (let i = 0; i < str.length; i++) {
      const char = str.charCodeAt(i);
      hash = (hash << 5) - hash + char;
      hash = hash & hash; // Convert to 32-bit integer
    }

    return Math.abs(hash).toString(36);
  }
}
