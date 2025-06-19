/**
 * PinningManager - Automatic IPFS Pinning with Smart Strategies
 *
 * This class implements intelligent pinning strategies for IPFS content:
 * - Fixed: Pin a fixed number of most important items
 * - Popularity: Pin based on access frequency and recency
 * - Size-based: Pin smaller items preferentially
 * - Custom: User-defined pinning logic
 * - Automatic cleanup of unpinned content
 */

import { PinningStrategy, PinningStats } from '../types/framework';

// Node.js types for compatibility
declare global {
  namespace NodeJS {
    interface Timeout {}
  }
}

export interface PinningRule {
  modelName: string;
  strategy?: PinningStrategy;
  factor?: number;
  maxPins?: number;
  minAccessCount?: number;
  maxAge?: number; // in milliseconds
  customLogic?: (item: any, stats: any) => number; // returns priority score
}

export interface PinnedItem {
  hash: string;
  modelName: string;
  itemId: string;
  pinnedAt: number;
  lastAccessed: number;
  accessCount: number;
  size: number;
  priority: number;
  metadata?: any;
}

export interface PinningMetrics {
  totalPinned: number;
  totalSize: number;
  averageSize: number;
  oldestPin: number;
  newestPin: number;
  mostAccessed: PinnedItem | null;
  leastAccessed: PinnedItem | null;
  strategyBreakdown: Map<PinningStrategy, number>;
}

export class PinningManager {
  private ipfsService: any;
  private pinnedItems: Map<string, PinnedItem> = new Map();
  private pinningRules: Map<string, PinningRule> = new Map();
  private accessLog: Map<string, { count: number; lastAccess: number }> = new Map();
  private cleanupInterval: NodeJS.Timeout | null = null;
  private maxTotalPins: number = 10000;
  private maxTotalSize: number = 10 * 1024 * 1024 * 1024; // 10GB
  private cleanupIntervalMs: number = 60000; // 1 minute

  constructor(
    ipfsService: any,
    options: {
      maxTotalPins?: number;
      maxTotalSize?: number;
      cleanupIntervalMs?: number;
    } = {},
  ) {
    this.ipfsService = ipfsService;
    this.maxTotalPins = options.maxTotalPins || this.maxTotalPins;
    this.maxTotalSize = options.maxTotalSize || this.maxTotalSize;
    this.cleanupIntervalMs = options.cleanupIntervalMs || this.cleanupIntervalMs;

    // Start automatic cleanup
    this.startAutoCleanup();
  }

  // Configure pinning rules for models
  setPinningRule(modelName: string, rule: Partial<PinningRule>): void {
    const existingRule = this.pinningRules.get(modelName);
    const newRule: PinningRule = {
      modelName,
      strategy: 'popularity' as const,
      factor: 1,
      ...existingRule,
      ...rule,
    };

    this.pinningRules.set(modelName, newRule);
    console.log(
      `📌 Set pinning rule for ${modelName}: ${newRule.strategy} (factor: ${newRule.factor})`,
    );
  }

  // Pin content based on configured strategy
  async pinContent(
    hash: string,
    modelName: string,
    itemId: string,
    metadata: any = {},
  ): Promise<boolean> {
    try {
      // Check if already pinned
      if (this.pinnedItems.has(hash)) {
        await this.recordAccess(hash);
        return true;
      }

      const rule = this.pinningRules.get(modelName);
      if (!rule) {
        console.warn(`No pinning rule found for model ${modelName}, skipping pin`);
        return false;
      }

      // Get content size
      const size = await this.getContentSize(hash);

      // Calculate priority based on strategy
      const priority = this.calculatePinningPriority(rule, metadata, size);

      // Check if we should pin based on priority and limits
      const shouldPin = await this.shouldPinContent(rule, priority, size);

      if (!shouldPin) {
        console.log(
          `⏭️  Skipping pin for ${hash} (${modelName}): priority too low or limits exceeded`,
        );
        return false;
      }

      // Perform the actual pinning
      await this.ipfsService.pin(hash);

      // Record the pinned item
      const pinnedItem: PinnedItem = {
        hash,
        modelName,
        itemId,
        pinnedAt: Date.now(),
        lastAccessed: Date.now(),
        accessCount: 1,
        size,
        priority,
        metadata,
      };

      this.pinnedItems.set(hash, pinnedItem);
      this.recordAccess(hash);

      console.log(
        `📌 Pinned ${hash} (${modelName}:${itemId}) with priority ${priority.toFixed(2)}`,
      );

      // Cleanup if we've exceeded limits
      await this.enforceGlobalLimits();

      return true;
    } catch (error) {
      console.error(`Failed to pin ${hash}:`, error);
      return false;
    }
  }

  // Unpin content
  async unpinContent(hash: string, force: boolean = false): Promise<boolean> {
    try {
      const pinnedItem = this.pinnedItems.get(hash);
      if (!pinnedItem) {
        console.warn(`Hash ${hash} is not tracked as pinned`);
        return false;
      }

      // Check if content should be protected from unpinning
      if (!force && (await this.isProtectedFromUnpinning(pinnedItem))) {
        console.log(`🔒 Content ${hash} is protected from unpinning`);
        return false;
      }

      await this.ipfsService.unpin(hash);
      this.pinnedItems.delete(hash);
      this.accessLog.delete(hash);

      console.log(`📌❌ Unpinned ${hash} (${pinnedItem.modelName}:${pinnedItem.itemId})`);
      return true;
    } catch (error) {
      console.error(`Failed to unpin ${hash}:`, error);
      return false;
    }
  }

  // Record access to pinned content
  async recordAccess(hash: string): Promise<void> {
    const pinnedItem = this.pinnedItems.get(hash);
    if (pinnedItem) {
      pinnedItem.lastAccessed = Date.now();
      pinnedItem.accessCount++;
    }

    // Update access log
    const accessInfo = this.accessLog.get(hash) || { count: 0, lastAccess: 0 };
    accessInfo.count++;
    accessInfo.lastAccess = Date.now();
    this.accessLog.set(hash, accessInfo);
  }

  // Calculate pinning priority based on strategy
  private calculatePinningPriority(rule: PinningRule, metadata: any, size: number): number {
    const now = Date.now();
    let priority = 0;

    switch (rule.strategy || 'popularity') {
      case 'fixed':
        // Fixed strategy: all items have equal priority
        priority = rule.factor || 1;
        break;

      case 'popularity':
        // Popularity-based: recent access + total access count
        const accessInfo = this.accessLog.get(metadata.hash) || { count: 0, lastAccess: 0 };
        const recencyScore = Math.max(0, 1 - (now - accessInfo.lastAccess) / (24 * 60 * 60 * 1000)); // 24h decay
        const accessScore = Math.min(1, accessInfo.count / 100); // Cap at 100 accesses
        priority = (recencyScore * 0.6 + accessScore * 0.4) * (rule.factor || 1);
        break;

      case 'size':
        // Size-based: prefer smaller content (inverse relationship)
        const maxSize = 100 * 1024 * 1024; // 100MB
        const sizeScore = Math.max(0.1, 1 - size / maxSize);
        priority = sizeScore * (rule.factor || 1);
        break;

      case 'age':
        // Age-based: prefer newer content
        const maxAge = 30 * 24 * 60 * 60 * 1000; // 30 days
        const age = now - (metadata.createdAt || now);
        const ageScore = Math.max(0.1, 1 - age / maxAge);
        priority = ageScore * (rule.factor || 1);
        break;

      case 'custom':
        // Custom logic provided by user
        if (rule.customLogic) {
          priority =
            rule.customLogic(metadata, {
              size,
              accessInfo: this.accessLog.get(metadata.hash),
              now,
            }) * (rule.factor || 1);
        } else {
          priority = rule.factor || 1;
        }
        break;

      default:
        priority = rule.factor || 1;
    }

    return Math.max(0, priority);
  }

  // Determine if content should be pinned
  private async shouldPinContent(
    rule: PinningRule,
    priority: number,
    size: number,
  ): Promise<boolean> {
    // Check rule-specific limits
    if (rule.maxPins) {
      const currentPinsForModel = Array.from(this.pinnedItems.values()).filter(
        (item) => item.modelName === rule.modelName,
      ).length;

      if (currentPinsForModel >= rule.maxPins) {
        // Find lowest priority item for this model to potentially replace
        const lowestPriorityItem = Array.from(this.pinnedItems.values())
          .filter((item) => item.modelName === rule.modelName)
          .sort((a, b) => a.priority - b.priority)[0];

        if (!lowestPriorityItem || priority <= lowestPriorityItem.priority) {
          return false;
        }

        // Unpin the lowest priority item to make room
        await this.unpinContent(lowestPriorityItem.hash, true);
      }
    }

    // Check global limits
    const metrics = this.getMetrics();

    if (metrics.totalPinned >= this.maxTotalPins) {
      // Find globally lowest priority item to replace
      const lowestPriorityItem = Array.from(this.pinnedItems.values()).sort(
        (a, b) => a.priority - b.priority,
      )[0];

      if (!lowestPriorityItem || priority <= lowestPriorityItem.priority) {
        return false;
      }

      await this.unpinContent(lowestPriorityItem.hash, true);
    }

    if (metrics.totalSize + size > this.maxTotalSize) {
      // Need to free up space
      const spaceNeeded = metrics.totalSize + size - this.maxTotalSize;
      await this.freeUpSpace(spaceNeeded);
    }

    return true;
  }

  // Check if content is protected from unpinning
  private async isProtectedFromUnpinning(pinnedItem: PinnedItem): Promise<boolean> {
    const rule = this.pinningRules.get(pinnedItem.modelName);
    if (!rule) return false;

    // Recently accessed content is protected
    const timeSinceAccess = Date.now() - pinnedItem.lastAccessed;
    if (timeSinceAccess < 60 * 60 * 1000) {
      // 1 hour
      return true;
    }

    // High-priority content is protected
    if (pinnedItem.priority > 0.8) {
      return true;
    }

    // Content with high access count is protected
    if (pinnedItem.accessCount > 50) {
      return true;
    }

    return false;
  }

  // Free up space by unpinning least important content
  private async freeUpSpace(spaceNeeded: number): Promise<void> {
    let freedSpace = 0;

    // Sort by priority (lowest first)
    const sortedItems = Array.from(this.pinnedItems.values())
      .filter((item) => !this.isProtectedFromUnpinning(item))
      .sort((a, b) => a.priority - b.priority);

    for (const item of sortedItems) {
      if (freedSpace >= spaceNeeded) break;

      await this.unpinContent(item.hash, true);
      freedSpace += item.size;
    }

    console.log(`🧹 Freed up ${(freedSpace / 1024 / 1024).toFixed(2)} MB of space`);
  }

  // Enforce global pinning limits
  private async enforceGlobalLimits(): Promise<void> {
    const metrics = this.getMetrics();

    // Check total pins limit
    if (metrics.totalPinned > this.maxTotalPins) {
      const excess = metrics.totalPinned - this.maxTotalPins;
      const itemsToUnpin = Array.from(this.pinnedItems.values())
        .sort((a, b) => a.priority - b.priority)
        .slice(0, excess);

      for (const item of itemsToUnpin) {
        await this.unpinContent(item.hash, true);
      }
    }

    // Check total size limit
    if (metrics.totalSize > this.maxTotalSize) {
      const excessSize = metrics.totalSize - this.maxTotalSize;
      await this.freeUpSpace(excessSize);
    }
  }

  // Automatic cleanup of old/unused pins
  private async performCleanup(): Promise<void> {
    const now = Date.now();
    const itemsToCleanup: PinnedItem[] = [];

    for (const item of this.pinnedItems.values()) {
      const rule = this.pinningRules.get(item.modelName);
      if (!rule) continue;

      let shouldCleanup = false;

      // Age-based cleanup
      if (rule.maxAge) {
        const age = now - item.pinnedAt;
        if (age > rule.maxAge) {
          shouldCleanup = true;
        }
      }

      // Access-based cleanup
      if (rule.minAccessCount) {
        if (item.accessCount < rule.minAccessCount) {
          shouldCleanup = true;
        }
      }

      // Inactivity-based cleanup (not accessed for 7 days)
      const inactivityThreshold = 7 * 24 * 60 * 60 * 1000;
      if (now - item.lastAccessed > inactivityThreshold && item.priority < 0.3) {
        shouldCleanup = true;
      }

      if (shouldCleanup && !(await this.isProtectedFromUnpinning(item))) {
        itemsToCleanup.push(item);
      }
    }

    // Unpin items marked for cleanup
    for (const item of itemsToCleanup) {
      await this.unpinContent(item.hash, true);
    }

    if (itemsToCleanup.length > 0) {
      console.log(`🧹 Cleaned up ${itemsToCleanup.length} old/unused pins`);
    }
  }

  // Start automatic cleanup
  private startAutoCleanup(): void {
    this.cleanupInterval = setInterval(() => {
      this.performCleanup().catch((error) => {
        console.error('Cleanup failed:', error);
      });
    }, this.cleanupIntervalMs);
  }

  // Stop automatic cleanup
  stopAutoCleanup(): void {
    if (this.cleanupInterval) {
      clearInterval(this.cleanupInterval as any);
      this.cleanupInterval = null;
    }
  }

  // Get content size from IPFS
  private async getContentSize(hash: string): Promise<number> {
    try {
      const stats = await this.ipfsService.object.stat(hash);
      return stats.CumulativeSize || stats.BlockSize || 0;
    } catch (error) {
      console.warn(`Could not get size for ${hash}:`, error);
      return 1024; // Default size
    }
  }

  // Get comprehensive metrics
  getMetrics(): PinningMetrics {
    const items = Array.from(this.pinnedItems.values());
    const totalSize = items.reduce((sum, item) => sum + item.size, 0);
    const strategyBreakdown = new Map<PinningStrategy, number>();

    // Count by strategy
    for (const item of items) {
      const rule = this.pinningRules.get(item.modelName);
      if (rule) {
        const strategy = rule.strategy || 'popularity';
        const count = strategyBreakdown.get(strategy) || 0;
        strategyBreakdown.set(strategy, count + 1);
      }
    }

    // Find most/least accessed
    const sortedByAccess = items.sort((a, b) => b.accessCount - a.accessCount);

    return {
      totalPinned: items.length,
      totalSize,
      averageSize: items.length > 0 ? totalSize / items.length : 0,
      oldestPin: items.length > 0 ? Math.min(...items.map((i) => i.pinnedAt)) : 0,
      newestPin: items.length > 0 ? Math.max(...items.map((i) => i.pinnedAt)) : 0,
      mostAccessed: sortedByAccess[0] || null,
      leastAccessed: sortedByAccess[sortedByAccess.length - 1] || null,
      strategyBreakdown,
    };
  }

  // Get pinning statistics
  getStats(): PinningStats {
    const metrics = this.getMetrics();
    return {
      totalPinned: metrics.totalPinned,
      totalSize: metrics.totalSize,
      averageSize: metrics.averageSize,
      strategies: Object.fromEntries(metrics.strategyBreakdown),
      oldestPin: metrics.oldestPin,
      recentActivity: this.getRecentActivity(),
    };
  }

  // Get recent pinning activity
  private getRecentActivity(): Array<{ action: string; hash: string; timestamp: number }> {
    // This would typically be implemented with a proper activity log
    // For now, we'll return recent pins
    const recentItems = Array.from(this.pinnedItems.values())
      .filter((item) => Date.now() - item.pinnedAt < 24 * 60 * 60 * 1000) // Last 24 hours
      .sort((a, b) => b.pinnedAt - a.pinnedAt)
      .slice(0, 10)
      .map((item) => ({
        action: 'pinned',
        hash: item.hash,
        timestamp: item.pinnedAt,
      }));

    return recentItems;
  }

  // Analyze pinning performance
  analyzePerformance(): any {
    const metrics = this.getMetrics();
    const now = Date.now();

    // Calculate hit rate (items accessed recently)
    const recentlyAccessedCount = Array.from(this.pinnedItems.values()).filter(
      (item) => now - item.lastAccessed < 60 * 60 * 1000,
    ).length; // Last hour

    const hitRate = metrics.totalPinned > 0 ? recentlyAccessedCount / metrics.totalPinned : 0;

    // Calculate average priority
    const averagePriority =
      Array.from(this.pinnedItems.values()).reduce((sum, item) => sum + item.priority, 0) /
        metrics.totalPinned || 0;

    // Storage efficiency
    const storageEfficiency =
      this.maxTotalSize > 0 ? (this.maxTotalSize - metrics.totalSize) / this.maxTotalSize : 0;

    return {
      hitRate,
      averagePriority,
      storageEfficiency,
      utilizationRate: metrics.totalPinned / this.maxTotalPins,
      averageItemAge: now - (metrics.oldestPin + metrics.newestPin) / 2,
      totalRules: this.pinningRules.size,
      accessDistribution: this.getAccessDistribution(),
    };
  }

  // Get access distribution statistics
  private getAccessDistribution(): any {
    const items = Array.from(this.pinnedItems.values());
    const accessCounts = items.map((item) => item.accessCount).sort((a, b) => a - b);

    if (accessCounts.length === 0) {
      return { min: 0, max: 0, median: 0, q1: 0, q3: 0 };
    }

    const min = accessCounts[0];
    const max = accessCounts[accessCounts.length - 1];
    const median = accessCounts[Math.floor(accessCounts.length / 2)];
    const q1 = accessCounts[Math.floor(accessCounts.length / 4)];
    const q3 = accessCounts[Math.floor((accessCounts.length * 3) / 4)];

    return { min, max, median, q1, q3 };
  }

  // Get pinned items for a specific model
  getPinnedItemsForModel(modelName: string): PinnedItem[] {
    return Array.from(this.pinnedItems.values()).filter((item) => item.modelName === modelName);
  }

  // Check if specific content is pinned
  isPinned(hash: string): boolean {
    return this.pinnedItems.has(hash);
  }

  // Clear all pins (for testing/reset)
  async clearAllPins(): Promise<void> {
    const hashes = Array.from(this.pinnedItems.keys());

    for (const hash of hashes) {
      await this.unpinContent(hash, true);
    }

    this.pinnedItems.clear();
    this.accessLog.clear();

    console.log(`🧹 Cleared all ${hashes.length} pins`);
  }

  // Shutdown
  async shutdown(): Promise<void> {
    this.stopAutoCleanup();
    console.log('📌 PinningManager shut down');
  }
}
