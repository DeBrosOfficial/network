import { BaseModel } from '../BaseModel';
import { ModelConfig } from '../../types/models';
import { StoreType } from '../../types/framework';
import { ModelRegistry } from '../../core/ModelRegistry';

export function Model(config: ModelConfig = {}) {
  return function <T extends typeof BaseModel>(target: T): T {
    // Validate model configuration
    validateModelConfig(config);
    
    // Initialize model-specific metadata maps, preserving existing ones
    if (!target.hasOwnProperty('fields')) {
      // Copy existing fields from prototype if any
      const parentFields = target.fields;
      Object.defineProperty(target, 'fields', {
        value: new Map(),
        writable: true,
        enumerable: false,
        configurable: true
      });
      if (parentFields) {
        for (const [key, value] of parentFields) {
          target.fields.set(key, value);
        }
      }
    }
    if (!target.hasOwnProperty('relationships')) {
      // Copy existing relationships from prototype if any
      const parentRelationships = target.relationships;
      target.relationships = new Map();
      if (parentRelationships) {
        for (const [key, value] of parentRelationships) {
          target.relationships.set(key, value);
        }
      }
    }
    if (!target.hasOwnProperty('hooks')) {
      // Copy existing hooks from prototype if any
      const parentHooks = target.hooks;
      target.hooks = new Map();
      if (parentHooks) {
        for (const [key, value] of parentHooks) {
          target.hooks.set(key, value);
        }
      }
    }

    // Set model configuration on the class using defineProperty to ensure they're own properties
    const modelName = config.tableName || target.name;
    const storeType = config.type || autoDetectType(target);
    const scope = config.scope || 'global';
    
    Object.defineProperty(target, 'modelName', {
      value: modelName,
      writable: true,
      enumerable: false,
      configurable: true
    });
    
    Object.defineProperty(target, 'storeType', {
      value: storeType,
      writable: true,
      enumerable: true,
      configurable: true
    });
    
    // Also set dbType for backwards compatibility
    Object.defineProperty(target, 'dbType', {
      value: storeType,
      writable: true,
      enumerable: true,
      configurable: true
    });
    
    Object.defineProperty(target, 'scope', {
      value: scope,
      writable: true,
      enumerable: false,
      configurable: true
    });
    
    if (config.sharding) {
      Object.defineProperty(target, 'sharding', {
        value: config.sharding,
        writable: true,
        enumerable: false,
        configurable: true
      });
    }
    
    if (config.pinning) {
      Object.defineProperty(target, 'pinning', {
        value: config.pinning,
        writable: true,
        enumerable: false,
        configurable: true
      });
    }
    

    // Register with framework
    ModelRegistry.register(target.name, target, config);

    // TODO: Set up automatic database creation when DatabaseManager is ready
    // DatabaseManager.scheduleCreation(target);

    return target;
  };
}

function validateModelConfig(config: ModelConfig): void {
  if (config.scope && !['user', 'global'].includes(config.scope)) {
    throw new Error(`Invalid model scope: ${config.scope}. Valid scopes are: user, global`);
  }
  
  if (config.type && !['docstore', 'keyvalue', 'eventlog'].includes(config.type)) {
    throw new Error(`Invalid store type: ${config.type}. Valid types are: docstore, keyvalue, eventlog`);
  }
}

function autoDetectType(modelClass: typeof BaseModel): StoreType {
  // Analyze model fields to suggest optimal database type
  const fields = modelClass.fields;

  if (!fields || fields.size === 0) {
    return 'docstore'; // Default for complex objects
  }

  let hasComplexFields = false;
  let _hasSimpleFields = false;

  for (const [_fieldName, fieldConfig] of fields) {
    if (fieldConfig.type === 'object' || fieldConfig.type === 'array') {
      hasComplexFields = true;
    } else {
      _hasSimpleFields = true;
    }
  }

  // If we have complex fields, use docstore
  if (hasComplexFields) {
    return 'docstore';
  }

  // If we only have simple fields, we could use keyvalue
  // But docstore is more flexible, so let's default to that
  return 'docstore';
}

// Export the decorator type for TypeScript
export type ModelDecorator = (config?: ModelConfig) => <T extends typeof BaseModel>(target: T) => T;
