import { BaseModel } from '../BaseModel';
import { ModelConfig } from '../../types/models';
import { StoreType } from '../../types/framework';
import { ModelRegistry } from '../../core/ModelRegistry';

export function Model(config: ModelConfig = {}) {
  return function <T extends typeof BaseModel>(target: T): T {
    // Set model configuration on the class
    target.modelName = config.tableName || target.name;
    target.dbType = config.type || autoDetectType(target);
    target.scope = config.scope || 'global';
    target.sharding = config.sharding;
    target.pinning = config.pinning;

    // Register with framework
    ModelRegistry.register(target.name, target, config);

    // TODO: Set up automatic database creation when DatabaseManager is ready
    // DatabaseManager.scheduleCreation(target);

    return target;
  };
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
