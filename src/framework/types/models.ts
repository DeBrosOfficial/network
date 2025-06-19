import { BaseModel } from '../models/BaseModel';
import { StoreType, ShardingConfig, PinningConfig, PubSubConfig } from './framework';

export interface ModelConfig {
  type?: StoreType;
  scope?: 'user' | 'global';
  sharding?: ShardingConfig;
  pinning?: PinningConfig;
  pubsub?: PubSubConfig;
  tableName?: string;
}

export interface FieldConfig {
  type: 'string' | 'number' | 'boolean' | 'array' | 'object' | 'date';
  required?: boolean;
  unique?: boolean;
  index?: boolean | 'global';
  default?: any;
  validate?: (value: any) => boolean | string;
  transform?: (value: any) => any;
}

export interface RelationshipConfig {
  type: 'belongsTo' | 'hasMany' | 'hasOne' | 'manyToMany';
  model: typeof BaseModel;
  foreignKey: string;
  localKey?: string;
  through?: typeof BaseModel;
  lazy?: boolean;
}

export interface UserMappings {
  userId: string;
  databases: Record<string, string>;
}

export class ValidationError extends Error {
  public errors: string[];

  constructor(errors: string[]) {
    super(`Validation failed: ${errors.join(', ')}`);
    this.errors = errors;
    this.name = 'ValidationError';
  }
}
