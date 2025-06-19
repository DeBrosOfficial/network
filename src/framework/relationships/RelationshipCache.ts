import { BaseModel } from '../models/BaseModel';

export interface RelationshipCacheEntry {
  key: string;
  data: any;
  timestamp: number;
  ttl: number;
  modelType: string;
  relationshipType: string;
}

export interface RelationshipCacheStats {
  totalEntries: number;
  hitCount: number;
  missCount: number;
  hitRate: number;
  memoryUsage: number;
}

export class RelationshipCache {
  private cache: Map<string, RelationshipCacheEntry> = new Map();
  private maxSize: number;
  private defaultTTL: number;
  private stats: RelationshipCacheStats;

  constructor(maxSize: number = 1000, defaultTTL: number = 600000) {
    // 10 minutes default
    this.maxSize = maxSize;
    this.defaultTTL = defaultTTL;
    this.stats = {
      totalEntries: 0,
      hitCount: 0,
      missCount: 0,
      hitRate: 0,
      memoryUsage: 0,
    };
  }

  generateKey(instance: BaseModel, relationshipName: string, extraData?: any): string {
    const baseKey = `${instance.constructor.name}:${instance.id}:${relationshipName}`;

    if (extraData) {
      try {
        const extraStr = JSON.stringify(extraData);
        if (extraStr) {
          return `${baseKey}:${this.hashString(extraStr)}`;
        }
      } catch (_e) {
        // If JSON.stringify fails (e.g., for functions), use a fallback
        const fallbackStr = String(extraData) || 'undefined';
        return `${baseKey}:${this.hashString(fallbackStr)}`;
      }
    }

    return baseKey;
  }

  get(key: string): any | null {
    const entry = this.cache.get(key);

    if (!entry) {
      this.stats.missCount++;
      this.updateHitRate();
      return null;
    }

    // Check if entry has expired
    if (Date.now() - entry.timestamp > entry.ttl) {
      this.cache.delete(key);
      this.stats.missCount++;
      this.updateHitRate();
      return null;
    }

    this.stats.hitCount++;
    this.updateHitRate();

    return this.deserializeData(entry.data, entry.modelType);
  }

  set(
    key: string,
    data: any,
    modelType: string,
    relationshipType: string,
    customTTL?: number,
  ): void {
    const ttl = customTTL || this.defaultTTL;

    // Check if we need to evict entries
    if (this.cache.size >= this.maxSize) {
      this.evictOldest();
    }

    const entry: RelationshipCacheEntry = {
      key,
      data: this.serializeData(data),
      timestamp: Date.now(),
      ttl,
      modelType,
      relationshipType,
    };

    this.cache.set(key, entry);
    this.stats.totalEntries = this.cache.size;
    this.updateMemoryUsage();
  }

  invalidate(key: string): boolean {
    const deleted = this.cache.delete(key);
    this.stats.totalEntries = this.cache.size;
    this.updateMemoryUsage();
    return deleted;
  }

  invalidateByInstance(instance: BaseModel): number {
    const prefix = `${instance.constructor.name}:${instance.id}:`;
    let deletedCount = 0;

    for (const [key] of this.cache.entries()) {
      if (key.startsWith(prefix)) {
        this.cache.delete(key);
        deletedCount++;
      }
    }

    this.stats.totalEntries = this.cache.size;
    this.updateMemoryUsage();
    return deletedCount;
  }

  invalidateByModel(modelName: string): number {
    let deletedCount = 0;

    for (const [key, entry] of this.cache.entries()) {
      if (key.startsWith(`${modelName}:`) || entry.modelType === modelName) {
        this.cache.delete(key);
        deletedCount++;
      }
    }

    this.stats.totalEntries = this.cache.size;
    this.updateMemoryUsage();
    return deletedCount;
  }

  invalidateByRelationship(relationshipType: string): number {
    let deletedCount = 0;

    for (const [key, entry] of this.cache.entries()) {
      if (entry.relationshipType === relationshipType) {
        this.cache.delete(key);
        deletedCount++;
      }
    }

    this.stats.totalEntries = this.cache.size;
    this.updateMemoryUsage();
    return deletedCount;
  }

  clear(): void {
    this.cache.clear();
    this.stats = {
      totalEntries: 0,
      hitCount: 0,
      missCount: 0,
      hitRate: 0,
      memoryUsage: 0,
    };
  }

  getStats(): RelationshipCacheStats {
    return { ...this.stats };
  }

  // Preload relationships for multiple instances
  async warmup(
    instances: BaseModel[],
    relationships: string[],
    loadFunction: (instance: BaseModel, relationshipName: string) => Promise<any>,
  ): Promise<void> {
    console.log(`🔥 Warming relationship cache for ${instances.length} instances...`);

    const promises: Promise<void>[] = [];

    for (const instance of instances) {
      for (const relationshipName of relationships) {
        promises.push(
          loadFunction(instance, relationshipName)
            .then((data) => {
              const key = this.generateKey(instance, relationshipName);
              const modelType = data?.constructor?.name || 'unknown';
              this.set(key, data, modelType, relationshipName);
            })
            .catch((error) => {
              console.warn(
                `Failed to warm cache for ${instance.constructor.name}:${instance.id}:${relationshipName}:`,
                error,
              );
            }),
        );
      }
    }

    await Promise.allSettled(promises);
    console.log(`✅ Relationship cache warmed with ${promises.length} entries`);
  }

  // Get cache entries by relationship type
  getEntriesByRelationship(relationshipType: string): RelationshipCacheEntry[] {
    return Array.from(this.cache.values()).filter(
      (entry) => entry.relationshipType === relationshipType,
    );
  }

  // Get expired entries
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

    this.stats.totalEntries = this.cache.size;
    this.updateMemoryUsage();
    return expired.length;
  }

  // Performance analysis
  analyzePerformance(): {
    averageAge: number;
    oldestEntry: number;
    newestEntry: number;
    relationshipTypes: Map<string, number>;
  } {
    const now = Date.now();
    let totalAge = 0;
    let oldestAge = 0;
    let newestAge = Infinity;
    const relationshipTypes = new Map<string, number>();

    for (const entry of this.cache.values()) {
      const age = now - entry.timestamp;
      totalAge += age;

      if (age > oldestAge) oldestAge = age;
      if (age < newestAge) newestAge = age;

      const count = relationshipTypes.get(entry.relationshipType) || 0;
      relationshipTypes.set(entry.relationshipType, count + 1);
    }

    return {
      averageAge: this.cache.size > 0 ? totalAge / this.cache.size : 0,
      oldestEntry: oldestAge,
      newestEntry: newestAge === Infinity ? 0 : newestAge,
      relationshipTypes,
    };
  }

  private serializeData(data: any): any {
    if (Array.isArray(data)) {
      return data.map((item) => this.serializeItem(item));
    } else {
      return this.serializeItem(data);
    }
  }

  private serializeItem(item: any): any {
    if (item && typeof item.toJSON === 'function') {
      return {
        __type: item.constructor.name,
        __data: item.toJSON(),
      };
    }
    return item;
  }

  private deserializeData(data: any, expectedType: string): any {
    if (Array.isArray(data)) {
      return data.map((item) => this.deserializeItem(item, expectedType));
    } else {
      return this.deserializeItem(data, expectedType);
    }
  }

  private deserializeItem(item: any, _expectedType: string): any {
    if (item && item.__type && item.__data) {
      // For now, return the raw data
      // In a full implementation, we would reconstruct the model instance
      return item.__data;
    }
    return item;
  }

  private evictOldest(): void {
    if (this.cache.size === 0) return;

    let oldestKey: string | null = null;
    let oldestTime = Infinity;

    for (const [key, entry] of this.cache.entries()) {
      if (entry.timestamp < oldestTime) {
        oldestTime = entry.timestamp;
        oldestKey = key;
      }
    }

    if (oldestKey) {
      this.cache.delete(oldestKey);
    }
  }

  private updateHitRate(): void {
    const total = this.stats.hitCount + this.stats.missCount;
    this.stats.hitRate = total > 0 ? this.stats.hitCount / total : 0;
  }

  private updateMemoryUsage(): void {
    // Rough estimation of memory usage
    let size = 0;
    for (const entry of this.cache.values()) {
      size += JSON.stringify(entry.data).length;
    }
    this.stats.memoryUsage = size;
  }

  private hashString(str: string): string {
    if (!str || typeof str !== 'string') {
      return 'empty';
    }
    
    let hash = 0;
    if (str.length === 0) return hash.toString();

    for (let i = 0; i < str.length; i++) {
      const char = str.charCodeAt(i);
      hash = (hash << 5) - hash + char;
      hash = hash & hash;
    }

    return Math.abs(hash).toString(36);
  }
}
