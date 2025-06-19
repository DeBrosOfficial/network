export type StoreType = 'eventlog' | 'keyvalue' | 'docstore' | 'counter' | 'feed';

export interface FrameworkConfig {
  cache?: CacheConfig;
  defaultPinning?: PinningConfig;
  autoMigration?: boolean;
}

export interface CacheConfig {
  enabled?: boolean;
  maxSize?: number;
  ttl?: number;
}

export type PinningStrategy = 'fixed' | 'popularity' | 'size' | 'age' | 'custom';

export interface PinningConfig {
  strategy?: PinningStrategy;
  factor?: number;
  maxPins?: number;
  minAccessCount?: number;
  maxAge?: number;
}

export interface PinningStats {
  totalPinned: number;
  totalSize: number;
  averageSize: number;
  strategies: Record<string, number>;
  oldestPin: number;
  recentActivity: Array<{ action: string; hash: string; timestamp: number }>;
}

export interface PubSubConfig {
  enabled?: boolean;
  events?: string[];
  channels?: string[];
}

export interface ShardingConfig {
  strategy: 'hash' | 'range' | 'user';
  count: number;
  key: string;
}

export interface ValidationResult {
  valid: boolean;
  errors: string[];
}

export interface ValidationError {
  field: string;
  message: string;
}
