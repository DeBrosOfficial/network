import { Metrics, ErrorCode } from '../types';
import { DBError } from '../core/error';
import * as cacheService from '../cache/cacheService';

// Metrics tracking
const metrics: Metrics = {
  operations: {
    creates: 0,
    reads: 0,
    updates: 0,
    deletes: 0,
    queries: 0,
    fileUploads: 0,
    fileDownloads: 0,
  },
  performance: {
    totalOperationTime: 0,
    operationCount: 0,
  },
  errors: {
    count: 0,
    byCode: {},
  },
  cacheStats: {
    hits: 0,
    misses: 0,
  },
  startTime: Date.now(),
};

/**
 * Measure performance of a database operation
 */
export async function measurePerformance<T>(operation: () => Promise<T>): Promise<T> {
  const startTime = performance.now();
  try {
    const result = await operation();
    const endTime = performance.now();
    metrics.performance.totalOperationTime += (endTime - startTime);
    metrics.performance.operationCount++;
    return result;
  } catch (error) {
    const endTime = performance.now();
    metrics.performance.totalOperationTime += (endTime - startTime);
    metrics.performance.operationCount++;
    
    // Track error metrics
    metrics.errors.count++;
    if (error instanceof DBError) {
      metrics.errors.byCode[error.code] = (metrics.errors.byCode[error.code] || 0) + 1;
    }
    
    throw error;
  }
}

/**
 * Get database metrics
 */
export const getMetrics = (): Metrics => {
  return {
    ...metrics,
    // Sync cache stats
    cacheStats: cacheService.cacheStats,
    // Calculate some derived metrics
    performance: {
      ...metrics.performance,
      averageOperationTime: metrics.performance.operationCount > 0 ?
        metrics.performance.totalOperationTime / metrics.performance.operationCount :
        0
    }
  };
};

/**
 * Reset metrics (useful for testing)
 */
export const resetMetrics = (): void => {
  metrics.operations = {
    creates: 0,
    reads: 0,
    updates: 0,
    deletes: 0,
    queries: 0,
    fileUploads: 0,
    fileDownloads: 0,
  };
  metrics.performance = {
    totalOperationTime: 0,
    operationCount: 0,
  };
  metrics.errors = {
    count: 0,
    byCode: {},
  };
  
  // Reset cache stats too
  cacheService.resetStats();
  
  metrics.startTime = Date.now();
};