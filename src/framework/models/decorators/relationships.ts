import { BaseModel } from '../BaseModel';
import { RelationshipConfig } from '../../types/models';

export function BelongsTo(
  modelFactory: () => typeof BaseModel,
  foreignKey: string,
  options: { localKey?: string } = {},
) {
  return function (target: any, propertyKey: string) {
    const config: RelationshipConfig = {
      type: 'belongsTo',
      modelFactory,
      foreignKey,
      localKey: options.localKey || 'id',
      lazy: true,
      options,
      targetModel: modelFactory, // Add targetModel as alias for test compatibility
    };

    registerRelationship(target, propertyKey, config);
    createRelationshipProperty(target, propertyKey, config);
  };
}

export function HasMany(
  modelFactory: () => typeof BaseModel,
  foreignKey: string,
  options: any = {},
) {
  return function (target: any, propertyKey: string) {
    const config: RelationshipConfig = {
      type: 'hasMany',
      modelFactory,
      foreignKey,
      localKey: options.localKey || 'id',
      through: options.through,
      lazy: true,
      options,
      targetModel: modelFactory, // Add targetModel as alias for test compatibility
    };

    registerRelationship(target, propertyKey, config);
    createRelationshipProperty(target, propertyKey, config);
  };
}

export function HasOne(
  modelFactory: () => typeof BaseModel,
  foreignKey: string,
  options: { localKey?: string } = {},
) {
  return function (target: any, propertyKey: string) {
    const config: RelationshipConfig = {
      type: 'hasOne',
      modelFactory,
      foreignKey,
      localKey: options.localKey || 'id',
      lazy: true,
      options,
      targetModel: modelFactory, // Add targetModel as alias for test compatibility
    };

    registerRelationship(target, propertyKey, config);
    createRelationshipProperty(target, propertyKey, config);
  };
}

export function ManyToMany(
  modelFactory: () => typeof BaseModel,
  through: string,
  foreignKey: string,
  otherKey: string,
  options: { localKey?: string; throughForeignKey?: string } = {},
) {
  return function (target: any, propertyKey: string) {
    const config: RelationshipConfig = {
      type: 'manyToMany',
      modelFactory,
      foreignKey,
      otherKey,
      localKey: options.localKey || 'id',
      through,
      lazy: true,
      options,
      targetModel: modelFactory, // Add targetModel as alias for test compatibility
    };

    registerRelationship(target, propertyKey, config);
    createRelationshipProperty(target, propertyKey, config);
  };
}

function registerRelationship(target: any, propertyKey: string, config: RelationshipConfig): void {
  // Initialize relationships map if it doesn't exist on this specific constructor
  if (!target.constructor.hasOwnProperty('relationships')) {
    target.constructor.relationships = new Map();
  }

  // Store relationship configuration
  target.constructor.relationships.set(propertyKey, config);

  const modelName = config.model?.name || (config.modelFactory ? 'LazyModel' : 'UnknownModel');
  console.log(
    `Registered ${config.type} relationship: ${target.constructor.name}.${propertyKey} -> ${modelName}`,
  );
}

function createRelationshipProperty(
  target: any,
  propertyKey: string,
  config: RelationshipConfig,
): void {
  const _relationshipKey = `_relationship_${propertyKey}`; // For future use

  Object.defineProperty(target, propertyKey, {
    get() {
      // Check if relationship is already loaded
      if (this._loadedRelations && this._loadedRelations.has(propertyKey)) {
        return this._loadedRelations.get(propertyKey);
      }

      if (config.lazy) {
        // Return a promise for lazy loading
        return this.loadRelation(propertyKey);
      } else {
        throw new Error(
          `Relationship '${propertyKey}' not loaded. Use .load(['${propertyKey}']) first.`,
        );
      }
    },
    set(value) {
      // Allow manual setting of relationship values
      if (!this._loadedRelations) {
        this._loadedRelations = new Map();
      }
      this._loadedRelations.set(propertyKey, value);
    },
    enumerable: true,
    configurable: true,
  });
}

// Utility function to get relationship configuration
export function getRelationshipConfig(
  target: any,
  propertyKey?: string,
): RelationshipConfig | undefined | RelationshipConfig[] {
  // Handle both class constructors and instances
  const relationships = target.relationships || (target.constructor && target.constructor.relationships);
  if (!relationships) {
    return propertyKey ? undefined : [];
  }
  
  if (propertyKey) {
    return relationships.get(propertyKey);
  } else {
    return Array.from(relationships.values()).map((config, index) => {
      const result = Object.assign({}, config) as any;
      result.propertyKey = Array.from(relationships.keys())[index];
      return result as RelationshipConfig;
    });
  }
}

// Type definitions for decorators
export type BelongsToDecorator = (
  modelFactory: () => typeof BaseModel,
  foreignKey: string,
  options?: { localKey?: string },
) => (target: any, propertyKey: string) => void;

export type HasManyDecorator = (
  modelFactory: () => typeof BaseModel,
  foreignKey: string,
  options?: any,
) => (target: any, propertyKey: string) => void;

export type HasOneDecorator = (
  modelFactory: () => typeof BaseModel,
  foreignKey: string,
  options?: { localKey?: string },
) => (target: any, propertyKey: string) => void;

export type ManyToManyDecorator = (
  modelFactory: () => typeof BaseModel,
  through: string,
  foreignKey: string,
  otherKey: string,
  options?: { localKey?: string; throughForeignKey?: string },
) => (target: any, propertyKey: string) => void;
