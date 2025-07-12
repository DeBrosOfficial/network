import { BaseModel } from '../models/BaseModel';
import { ModelConfig } from '../types/models';
import { StoreType } from '../types/framework';

export class ModelRegistry {
  private static models: Map<string, typeof BaseModel> = new Map();
  private static configs: Map<string, ModelConfig> = new Map();

  static register(name: string, modelClass: typeof BaseModel, config: ModelConfig = {}): void {
    this.models.set(name, modelClass);
    this.configs.set(name, config);

    // Validate model configuration
    this.validateModel(modelClass, config);

    console.log(`Registered model: ${name} with scope: ${config.scope || 'global'}`);
  }

  static get(name: string): typeof BaseModel | undefined {
    return this.models.get(name);
  }

  static getConfig(name: string): ModelConfig | undefined {
    return this.configs.get(name);
  }

  static getAllModels(): Map<string, typeof BaseModel> {
    return new Map(this.models);
  }

  static getUserScopedModels(): Array<typeof BaseModel> {
    return Array.from(this.models.values()).filter((model) => model.scope === 'user');
  }

  static getGlobalModels(): Array<typeof BaseModel> {
    return Array.from(this.models.values()).filter((model) => model.scope === 'global');
  }

  static getModelNames(): string[] {
    return Array.from(this.models.keys());
  }

  static clear(): void {
    this.models.clear();
    this.configs.clear();
  }

  private static validateModel(modelClass: typeof BaseModel, config: ModelConfig): void {
    // Validate model name
    if (!modelClass.name) {
      throw new Error('Model class must have a name');
    }

    // Validate database type
    if (config.type && !this.isValidStoreType(config.type)) {
      throw new Error(`Invalid store type: ${config.type}`);
    }

    // Validate scope
    if (config.scope && !['user', 'global'].includes(config.scope)) {
      throw new Error(`Invalid scope: ${config.scope}. Must be 'user' or 'global'`);
    }

    // Validate sharding configuration
    if (config.sharding) {
      this.validateShardingConfig(config.sharding);
    }

    // Validate pinning configuration
    if (config.pinning) {
      this.validatePinningConfig(config.pinning);
    }

    console.log(`✓ Model ${modelClass.name} configuration validated`);
  }

  private static isValidStoreType(type: StoreType): boolean {
    return ['eventlog', 'keyvalue', 'docstore', 'counter', 'feed'].includes(type);
  }

  private static validateShardingConfig(config: any): void {
    if (!config.strategy || !['hash', 'range', 'user'].includes(config.strategy)) {
      throw new Error('Sharding strategy must be one of: hash, range, user');
    }

    if (!config.count || config.count < 1) {
      throw new Error('Sharding count must be a positive number');
    }

    if (!config.key) {
      throw new Error('Sharding key is required');
    }
  }

  private static validatePinningConfig(config: any): void {
    if (config.strategy && !['fixed', 'popularity', 'tiered'].includes(config.strategy)) {
      throw new Error('Pinning strategy must be one of: fixed, popularity, tiered');
    }

    if (config.factor && (typeof config.factor !== 'number' || config.factor < 1)) {
      throw new Error('Pinning factor must be a positive number');
    }
  }
}
