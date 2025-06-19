import { BaseModel } from '../BaseModel';
import { RelationshipConfig } from '../../types/models';

export function BelongsTo(
  model: typeof BaseModel,
  foreignKey: string,
  options: { localKey?: string } = {},
) {
  return function (target: any, propertyKey: string) {
    const config: RelationshipConfig = {
      type: 'belongsTo',
      model,
      foreignKey,
      localKey: options.localKey || 'id',
      lazy: true,
    };

    registerRelationship(target, propertyKey, config);
    createRelationshipProperty(target, propertyKey, config);
  };
}

export function HasMany(
  model: typeof BaseModel,
  foreignKey: string,
  options: { localKey?: string; through?: typeof BaseModel } = {},
) {
  return function (target: any, propertyKey: string) {
    const config: RelationshipConfig = {
      type: 'hasMany',
      model,
      foreignKey,
      localKey: options.localKey || 'id',
      through: options.through,
      lazy: true,
    };

    registerRelationship(target, propertyKey, config);
    createRelationshipProperty(target, propertyKey, config);
  };
}

export function HasOne(
  model: typeof BaseModel,
  foreignKey: string,
  options: { localKey?: string } = {},
) {
  return function (target: any, propertyKey: string) {
    const config: RelationshipConfig = {
      type: 'hasOne',
      model,
      foreignKey,
      localKey: options.localKey || 'id',
      lazy: true,
    };

    registerRelationship(target, propertyKey, config);
    createRelationshipProperty(target, propertyKey, config);
  };
}

export function ManyToMany(
  model: typeof BaseModel,
  through: typeof BaseModel,
  foreignKey: string,
  options: { localKey?: string; throughForeignKey?: string } = {},
) {
  return function (target: any, propertyKey: string) {
    const config: RelationshipConfig = {
      type: 'manyToMany',
      model,
      foreignKey,
      localKey: options.localKey || 'id',
      through,
      lazy: true,
    };

    registerRelationship(target, propertyKey, config);
    createRelationshipProperty(target, propertyKey, config);
  };
}

function registerRelationship(target: any, propertyKey: string, config: RelationshipConfig): void {
  // Initialize relationships map if it doesn't exist
  if (!target.constructor.relationships) {
    target.constructor.relationships = new Map();
  }

  // Store relationship configuration
  target.constructor.relationships.set(propertyKey, config);

  console.log(
    `Registered ${config.type} relationship: ${target.constructor.name}.${propertyKey} -> ${config.model.name}`,
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
  propertyKey: string,
): RelationshipConfig | undefined {
  if (!target.constructor.relationships) {
    return undefined;
  }
  return target.constructor.relationships.get(propertyKey);
}

// Type definitions for decorators
export type BelongsToDecorator = (
  model: typeof BaseModel,
  foreignKey: string,
  options?: { localKey?: string },
) => (target: any, propertyKey: string) => void;

export type HasManyDecorator = (
  model: typeof BaseModel,
  foreignKey: string,
  options?: { localKey?: string; through?: typeof BaseModel },
) => (target: any, propertyKey: string) => void;

export type HasOneDecorator = (
  model: typeof BaseModel,
  foreignKey: string,
  options?: { localKey?: string },
) => (target: any, propertyKey: string) => void;

export type ManyToManyDecorator = (
  model: typeof BaseModel,
  through: typeof BaseModel,
  foreignKey: string,
  options?: { localKey?: string; throughForeignKey?: string },
) => (target: any, propertyKey: string) => void;
