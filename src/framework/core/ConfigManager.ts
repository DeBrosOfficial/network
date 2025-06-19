import { FrameworkConfig, CacheConfig, PinningConfig } from '../types/framework';

export interface DatabaseConfig {
  userDirectoryShards?: number;
  defaultGlobalShards?: number;
  cacheSize?: number;
}

export interface ExtendedFrameworkConfig extends FrameworkConfig {
  database?: DatabaseConfig;
  debug?: boolean;
  logLevel?: 'error' | 'warn' | 'info' | 'debug';
}

export class ConfigManager {
  private config: ExtendedFrameworkConfig;
  private defaults: ExtendedFrameworkConfig = {
    cache: {
      enabled: true,
      maxSize: 1000,
      ttl: 300000, // 5 minutes
    },
    defaultPinning: {
      strategy: 'fixed' as const,
      factor: 2,
    },
    database: {
      userDirectoryShards: 4,
      defaultGlobalShards: 8,
      cacheSize: 100,
    },
    autoMigration: true,
    debug: false,
    logLevel: 'info',
  };

  constructor(config: ExtendedFrameworkConfig = {}) {
    this.config = this.mergeWithDefaults(config);
    this.validateConfig();
  }

  private mergeWithDefaults(config: ExtendedFrameworkConfig): ExtendedFrameworkConfig {
    return {
      ...this.defaults,
      ...config,
      cache: {
        ...this.defaults.cache,
        ...config.cache,
      },
      defaultPinning: {
        ...this.defaults.defaultPinning,
        ...(config.defaultPinning || {}),
      },
      database: {
        ...this.defaults.database,
        ...config.database,
      },
    };
  }

  private validateConfig(): void {
    // Validate cache configuration
    if (this.config.cache) {
      if (this.config.cache.maxSize && this.config.cache.maxSize < 1) {
        throw new Error('Cache maxSize must be at least 1');
      }
      if (this.config.cache.ttl && this.config.cache.ttl < 1000) {
        throw new Error('Cache TTL must be at least 1000ms');
      }
    }

    // Validate pinning configuration
    if (this.config.defaultPinning) {
      if (this.config.defaultPinning.factor && this.config.defaultPinning.factor < 1) {
        throw new Error('Pinning factor must be at least 1');
      }
    }

    // Validate database configuration
    if (this.config.database) {
      if (
        this.config.database.userDirectoryShards &&
        this.config.database.userDirectoryShards < 1
      ) {
        throw new Error('User directory shards must be at least 1');
      }
      if (
        this.config.database.defaultGlobalShards &&
        this.config.database.defaultGlobalShards < 1
      ) {
        throw new Error('Default global shards must be at least 1');
      }
    }
  }

  // Getters for configuration values
  get cacheConfig(): CacheConfig | undefined {
    return this.config.cache;
  }

  get defaultPinningConfig(): PinningConfig | undefined {
    return this.config.defaultPinning;
  }

  get databaseConfig(): DatabaseConfig | undefined {
    return this.config.database;
  }

  get autoMigration(): boolean {
    return this.config.autoMigration || false;
  }

  get debug(): boolean {
    return this.config.debug || false;
  }

  get logLevel(): string {
    return this.config.logLevel || 'info';
  }

  // Update configuration at runtime
  updateConfig(newConfig: Partial<ExtendedFrameworkConfig>): void {
    this.config = this.mergeWithDefaults({
      ...this.config,
      ...newConfig,
    });
    this.validateConfig();
  }

  // Get full configuration
  getConfig(): ExtendedFrameworkConfig {
    return { ...this.config };
  }

  // Configuration presets
  static developmentConfig(): ExtendedFrameworkConfig {
    return {
      debug: true,
      logLevel: 'debug',
      cache: {
        enabled: true,
        maxSize: 100,
        ttl: 60000, // 1 minute for development
      },
      database: {
        userDirectoryShards: 2,
        defaultGlobalShards: 2,
        cacheSize: 50,
      },
      defaultPinning: {
        strategy: 'fixed' as const,
        factor: 1, // Minimal pinning for development
      },
    };
  }

  static productionConfig(): ExtendedFrameworkConfig {
    return {
      debug: false,
      logLevel: 'warn',
      cache: {
        enabled: true,
        maxSize: 10000,
        ttl: 600000, // 10 minutes
      },
      database: {
        userDirectoryShards: 16,
        defaultGlobalShards: 32,
        cacheSize: 1000,
      },
      defaultPinning: {
        strategy: 'popularity' as const,
        factor: 5, // Higher redundancy for production
      },
    };
  }

  static testConfig(): ExtendedFrameworkConfig {
    return {
      debug: true,
      logLevel: 'error', // Minimal logging during tests
      cache: {
        enabled: false, // Disable caching for predictable tests
      },
      database: {
        userDirectoryShards: 1,
        defaultGlobalShards: 1,
        cacheSize: 10,
      },
      defaultPinning: {
        strategy: 'fixed',
        factor: 1,
      },
      autoMigration: false, // Manual migration control in tests
    };
  }
}
