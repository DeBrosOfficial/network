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

    createRelationshipProperty(target, propertyKey, config);
  };
}

function createRelationshipProperty(
  target: any,
  propertyKey: string,
  config: RelationshipConfig,
): void {
  // Handle ESM case where target might be undefined
  if (!target) {
    // In ESM environment, defer the decorator application
    // Create a deferred setup that will be called when the class is actually used
    deferredRelationshipSetup(config, propertyKey);
    return;
  }

  // Get the constructor function - handle ESM case where constructor might be undefined
  const ctor = (target.constructor || target) as typeof BaseModel;

  // Additional safety check for constructor
  if (!ctor) {
    console.warn(`Constructor is undefined for property ${propertyKey}, skipping decorator setup`);
    return;
  }

  // Initialize relationships map if it doesn't exist
  if (!ctor.hasOwnProperty('relationships')) {
    const parentRelationships = ctor.relationships ? new Map(ctor.relationships) : new Map();
    Object.defineProperty(ctor, 'relationships', {
      value: parentRelationships,
      writable: true,
      enumerable: false,
      configurable: true,
    });
  }

  // Store relationship configuration
  ctor.relationships.set(propertyKey, config);

  // Define property on the prototype
  Object.defineProperty(target, propertyKey, {
    get() {
      const ctor = this.constructor as typeof BaseModel;

      // Ensure relationships map exists on the constructor
      if (!ctor.hasOwnProperty('relationships')) {
        const parentRelationships = ctor.relationships ? new Map(ctor.relationships) : new Map();
        Object.defineProperty(ctor, 'relationships', {
          value: parentRelationships,
          writable: true,
          enumerable: false,
          configurable: true,
        });
      }

      // Store relationship configuration if it's not already there
      if (!ctor.relationships.has(propertyKey)) {
        ctor.relationships.set(propertyKey, config);
      }

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
      const ctor = this.constructor as typeof BaseModel;

      // Ensure relationships map exists on the constructor
      if (!ctor.hasOwnProperty('relationships')) {
        const parentRelationships = ctor.relationships ? new Map(ctor.relationships) : new Map();
        Object.defineProperty(ctor, 'relationships', {
          value: parentRelationships,
          writable: true,
          enumerable: false,
          configurable: true,
        });
      }

      // Store relationship configuration if it's not already there
      if (!ctor.relationships.has(propertyKey)) {
        ctor.relationships.set(propertyKey, config);
      }

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
  const relationships =
    target.relationships || (target.constructor && target.constructor.relationships);
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

// Deferred setup function for ESM environments
function deferredRelationshipSetup(config: RelationshipConfig, propertyKey: string) {
  // Return a function that will be called when the class is properly initialized
  return function () {
    // This function will be called later when the class prototype is ready
    console.warn(`Deferred relationship setup not yet implemented for property ${propertyKey}`);
  };
}
