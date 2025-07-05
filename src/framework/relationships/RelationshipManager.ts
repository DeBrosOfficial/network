import { BaseModel } from '../models/BaseModel';
import { RelationshipConfig } from '../types/models';
import { RelationshipCache } from './RelationshipCache';
import { QueryBuilder } from '../query/QueryBuilder';

export interface RelationshipLoadOptions {
  useCache?: boolean;
  constraints?: (query: QueryBuilder<any>) => QueryBuilder<any>;
  limit?: number;
  orderBy?: { field: string; direction: 'asc' | 'desc' };
}

export interface EagerLoadPlan {
  relationshipName: string;
  config: RelationshipConfig;
  instances: BaseModel[];
  options?: RelationshipLoadOptions;
}

export class RelationshipManager {
  private framework: any;
  private cache: RelationshipCache;

  constructor(framework: any) {
    this.framework = framework;
    this.cache = new RelationshipCache();
  }

  async loadRelationship(
    instance: BaseModel,
    relationshipName: string,
    options: RelationshipLoadOptions = {},
  ): Promise<any> {
    const modelClass = instance.constructor as typeof BaseModel;
    const relationConfig = modelClass.relationships?.get(relationshipName);

    if (!relationConfig) {
      throw new Error(`Relationship '${relationshipName}' not found on ${modelClass.name}`);
    }

    console.log(
      `🔗 Loading ${relationConfig.type} relationship: ${modelClass.name}.${relationshipName}`,
    );

    // Check cache first if enabled
    if (options.useCache !== false) {
      const cacheKey = this.cache.generateKey(instance, relationshipName, options.constraints);
      const cached = this.cache.get(cacheKey);
      if (cached) {
        console.log(`⚡ Cache hit for relationship ${relationshipName}`);
        instance._loadedRelations.set(relationshipName, cached);
        return cached;
      }
    }

    // Load relationship based on type
    let result: any;
    switch (relationConfig.type) {
      case 'belongsTo':
        result = await this.loadBelongsTo(instance, relationConfig, options);
        break;
      case 'hasMany':
        result = await this.loadHasMany(instance, relationConfig, options);
        break;
      case 'hasOne':
        result = await this.loadHasOne(instance, relationConfig, options);
        break;
      case 'manyToMany':
        result = await this.loadManyToMany(instance, relationConfig, options);
        break;
      default:
        throw new Error(`Unsupported relationship type: ${relationConfig.type}`);
    }

    // Cache the result if enabled
    if (options.useCache !== false && result) {
      const cacheKey = this.cache.generateKey(instance, relationshipName, options.constraints);
      const modelType = Array.isArray(result)
        ? result[0]?.constructor?.name || 'unknown'
        : result.constructor?.name || 'unknown';

      this.cache.set(cacheKey, result, modelType, relationConfig.type);
    }

    // Store in instance
    instance.setRelation(relationshipName, result);

    console.log(
      `✅ Loaded ${relationConfig.type} relationship: ${Array.isArray(result) ? result.length : 1} item(s)`,
    );
    return result;
  }

  private async loadBelongsTo(
    instance: BaseModel,
    config: RelationshipConfig,
    options: RelationshipLoadOptions,
  ): Promise<BaseModel | null> {
    const foreignKeyValue = (instance as any)[config.foreignKey];

    if (!foreignKeyValue) {
      return null;
    }

    // Get the related model class
    const RelatedModel = config.model || (config.modelFactory ? config.modelFactory() : null) || (config.targetModel ? config.targetModel() : null);
    if (!RelatedModel) {
      throw new Error(`Cannot resolve related model for belongsTo relationship`);
    }

    // Build query for the related model
    let query = (RelatedModel as any).where('id', '=', foreignKeyValue);

    // Apply constraints if provided
    if (options.constraints) {
      query = options.constraints(query);
    }

    const result = await query.first();
    return result;
  }

  private async loadHasMany(
    instance: BaseModel,
    config: RelationshipConfig,
    options: RelationshipLoadOptions,
  ): Promise<BaseModel[]> {
    if (config.through) {
      return await this.loadManyToMany(instance, config, options);
    }

    const localKeyValue = (instance as any)[config.localKey || 'id'];

    if (!localKeyValue) {
      return [];
    }

    // Get the related model class
    const RelatedModel = config.model || (config.modelFactory ? config.modelFactory() : null) || (config.targetModel ? config.targetModel() : null);
    if (!RelatedModel) {
      throw new Error(`Cannot resolve related model for hasMany relationship`);
    }

    // Build query for the related model
    let query = (RelatedModel as any).where(config.foreignKey, '=', localKeyValue);

    // Apply constraints if provided
    if (options.constraints) {
      query = options.constraints(query);
    }

    // Apply default ordering and limiting
    if (options.orderBy) {
      query = query.orderBy(options.orderBy.field, options.orderBy.direction);
    }

    if (options.limit) {
      query = query.limit(options.limit);
    }

    return await query.exec();
  }

  private async loadHasOne(
    instance: BaseModel,
    config: RelationshipConfig,
    options: RelationshipLoadOptions,
  ): Promise<BaseModel | null> {
    const results = await this.loadHasMany(
      instance,
      { ...config, type: 'hasMany' },
      {
        ...options,
        limit: 1,
      },
    );

    return results[0] || null;
  }

  private async loadManyToMany(
    instance: BaseModel,
    config: RelationshipConfig,
    options: RelationshipLoadOptions,
  ): Promise<BaseModel[]> {
    if (!config.through) {
      throw new Error('Many-to-many relationships require a through model');
    }

    const localKeyValue = (instance as any)[config.localKey || 'id'];

    if (!localKeyValue) {
      return [];
    }

    // Step 1: Get junction table records
    // For many-to-many relationships, we need to query the junction table with the foreign key for this side
    const junctionLocalKey = config.otherKey || config.foreignKey; // The key in junction table that points to this model
    let junctionQuery = (config.through as any).where(junctionLocalKey, '=', localKeyValue);

    // Apply constraints to junction if needed
    if (options.constraints) {
      // Note: This is simplified - in a full implementation we'd need to handle
      // constraints that apply to the final model vs the junction model
    }

    const junctionRecords = await junctionQuery.exec();

    if (junctionRecords.length === 0) {
      return [];
    }

    // Step 2: Extract foreign keys
    const foreignKeys = junctionRecords.map((record: any) => record[config.foreignKey]);

    // Step 3: Get related models
    // Get the related model class
    const RelatedModel = config.model || (config.modelFactory ? config.modelFactory() : null) || (config.targetModel ? config.targetModel() : null);
    if (!RelatedModel) {
      throw new Error(`Cannot resolve related model for manyToMany relationship`);
    }

    let relatedQuery = (RelatedModel as any).whereIn('id', foreignKeys);

    // Apply constraints if provided
    if (options.constraints) {
      relatedQuery = options.constraints(relatedQuery);
    }

    // Apply ordering and limiting
    if (options.orderBy) {
      relatedQuery = relatedQuery.orderBy(options.orderBy.field, options.orderBy.direction);
    }

    if (options.limit) {
      relatedQuery = relatedQuery.limit(options.limit);
    }

    return await relatedQuery.exec();
  }

  // Eager loading for multiple instances
  async eagerLoadRelationships(
    instances: BaseModel[],
    relationships: string[],
    options: Record<string, RelationshipLoadOptions> = {},
  ): Promise<void> {
    if (instances.length === 0) return;

    console.log(
      `🚀 Eager loading ${relationships.length} relationships for ${instances.length} instances`,
    );

    // Group instances by model type for efficient processing
    const instanceGroups = this.groupInstancesByModel(instances);

    // Load each relationship for each model group
    for (const relationshipName of relationships) {
      await this.eagerLoadSingleRelationship(
        instanceGroups,
        relationshipName,
        options[relationshipName] || {},
      );
    }

    console.log(`✅ Eager loading completed for ${relationships.length} relationships`);
  }

  private async eagerLoadSingleRelationship(
    instanceGroups: Map<string, BaseModel[]>,
    relationshipName: string,
    options: RelationshipLoadOptions,
  ): Promise<void> {
    for (const [modelName, instances] of instanceGroups) {
      if (instances.length === 0) continue;

      const firstInstance = instances[0];
      const modelClass = firstInstance.constructor as typeof BaseModel;
      const relationConfig = modelClass.relationships?.get(relationshipName);

      if (!relationConfig) {
        console.warn(`Relationship '${relationshipName}' not found on ${modelName}`);
        continue;
      }

      console.log(
        `🔗 Eager loading ${relationConfig.type} for ${instances.length} ${modelName} instances`,
      );

      switch (relationConfig.type) {
        case 'belongsTo':
          await this.eagerLoadBelongsTo(instances, relationshipName, relationConfig, options);
          break;
        case 'hasMany':
          await this.eagerLoadHasMany(instances, relationshipName, relationConfig, options);
          break;
        case 'hasOne':
          await this.eagerLoadHasOne(instances, relationshipName, relationConfig, options);
          break;
        case 'manyToMany':
          await this.eagerLoadManyToMany(instances, relationshipName, relationConfig, options);
          break;
      }
    }
  }

  private async eagerLoadBelongsTo(
    instances: BaseModel[],
    relationshipName: string,
    config: RelationshipConfig,
    options: RelationshipLoadOptions,
  ): Promise<void> {
    // Get all foreign key values
    const foreignKeys = instances
      .map((instance) => (instance as any)[config.foreignKey])
      .filter((key) => key != null);

    if (foreignKeys.length === 0) {
      // Set null for all instances
      instances.forEach((instance) => {
        instance._loadedRelations.set(relationshipName, null);
      });
      return;
    }

    // Remove duplicates
    const uniqueForeignKeys = [...new Set(foreignKeys)];

    // Load all related models at once
    const RelatedModel = config.model || (config.modelFactory ? config.modelFactory() : null) || (config.targetModel ? config.targetModel() : null);
    if (!RelatedModel) {
      throw new Error(`Could not resolve related model for ${relationshipName}`);
    }
    let query = (RelatedModel as any).whereIn('id', uniqueForeignKeys);

    if (options.constraints) {
      query = options.constraints(query);
    }

    const relatedModels = await query.exec();

    // Create lookup map
    const relatedMap = new Map();
    relatedModels.forEach((model: any) => relatedMap.set(model.id, model));

    // Assign to instances and cache
    instances.forEach((instance) => {
      const foreignKeyValue = (instance as any)[config.foreignKey];
      const related = relatedMap.get(foreignKeyValue) || null;
      instance.setRelation(relationshipName, related);

      // Cache individual relationship
      if (options.useCache !== false) {
        const cacheKey = this.cache.generateKey(instance, relationshipName, options.constraints);
        const modelType = related?.constructor?.name || 'null';
        this.cache.set(cacheKey, related, modelType, config.type);
      }
    });
  }

  private async eagerLoadHasMany(
    instances: BaseModel[],
    relationshipName: string,
    config: RelationshipConfig,
    options: RelationshipLoadOptions,
  ): Promise<void> {
    if (config.through) {
      return await this.eagerLoadManyToMany(instances, relationshipName, config, options);
    }

    // Get all local key values
    const localKeys = instances
      .map((instance) => (instance as any)[config.localKey || 'id'])
      .filter((key) => key != null);

    if (localKeys.length === 0) {
      instances.forEach((instance) => {
        instance.setRelation(relationshipName, []);
      });
      return;
    }

    // Get the related model class
    const RelatedModel = config.model || (config.modelFactory ? config.modelFactory() : null) || (config.targetModel ? config.targetModel() : null);
    if (!RelatedModel) {
      throw new Error(`Cannot resolve related model for hasMany eager loading`);
    }

    // Load all related models
    let query = (RelatedModel as any).whereIn(config.foreignKey, localKeys);

    if (options.constraints) {
      query = options.constraints(query);
    }

    if (options.orderBy) {
      query = query.orderBy(options.orderBy.field, options.orderBy.direction);
    }

    const relatedModels = await query.exec();

    // Group by foreign key
    const relatedGroups = new Map<string, BaseModel[]>();
    relatedModels.forEach((model: any) => {
      const foreignKeyValue = model[config.foreignKey];
      if (!relatedGroups.has(foreignKeyValue)) {
        relatedGroups.set(foreignKeyValue, []);
      }
      relatedGroups.get(foreignKeyValue)!.push(model);
    });

    // Apply limit per instance if specified
    if (options.limit) {
      relatedGroups.forEach((group) => {
        if (group.length > options.limit!) {
          group.splice(options.limit!);
        }
      });
    }

    // Assign to instances and cache
    instances.forEach((instance) => {
      const localKeyValue = (instance as any)[config.localKey || 'id'];
      const related = relatedGroups.get(localKeyValue) || [];
      instance.setRelation(relationshipName, related);

      // Cache individual relationship
      if (options.useCache !== false) {
        const cacheKey = this.cache.generateKey(instance, relationshipName, options.constraints);
        const modelType = related[0]?.constructor?.name || 'array';
        this.cache.set(cacheKey, related, modelType, config.type);
      }
    });
  }

  private async eagerLoadHasOne(
    instances: BaseModel[],
    relationshipName: string,
    config: RelationshipConfig,
    options: RelationshipLoadOptions,
  ): Promise<void> {
    // Load as hasMany but take only the first result for each instance
    await this.eagerLoadHasMany(instances, relationshipName, config, {
      ...options,
      limit: 1,
    });

    // Convert arrays to single items
    instances.forEach((instance) => {
      const relatedArray = instance._loadedRelations.get(relationshipName) || [];
      const relatedItem = relatedArray[0] || null;
      instance._loadedRelations.set(relationshipName, relatedItem);
    });
  }

  private async eagerLoadManyToMany(
    instances: BaseModel[],
    relationshipName: string,
    config: RelationshipConfig,
    options: RelationshipLoadOptions,
  ): Promise<void> {
    if (!config.through) {
      throw new Error('Many-to-many relationships require a through model');
    }

    // Get all local key values
    const localKeys = instances
      .map((instance) => (instance as any)[config.localKey || 'id'])
      .filter((key) => key != null);

    if (localKeys.length === 0) {
      instances.forEach((instance) => {
        instance.setRelation(relationshipName, []);
      });
      return;
    }

    // Step 1: Get all junction records
    const junctionLocalKey = config.otherKey || config.foreignKey; // The key in junction table that points to this model
    const junctionRecords = await (config.through as any)
      .whereIn(junctionLocalKey, localKeys)
      .exec();

    if (junctionRecords.length === 0) {
      instances.forEach((instance) => {
        instance.setRelation(relationshipName, []);
      });
      return;
    }

    // Step 2: Group junction records by local key
    const junctionGroups = new Map<string, any[]>();
    junctionRecords.forEach((record: any) => {
      const localKeyValue = (record as any)[junctionLocalKey];
      if (!junctionGroups.has(localKeyValue)) {
        junctionGroups.set(localKeyValue, []);
      }
      junctionGroups.get(localKeyValue)!.push(record);
    });

    // Step 3: Get all foreign keys
    const allForeignKeys = junctionRecords.map((record: any) => (record as any)[config.foreignKey]);
    const uniqueForeignKeys = [...new Set(allForeignKeys)];

    // Step 4: Load all related models
    // Get the related model class
    const RelatedModel = config.model || (config.modelFactory ? config.modelFactory() : null) || (config.targetModel ? config.targetModel() : null);
    if (!RelatedModel) {
      throw new Error(`Cannot resolve related model for manyToMany eager loading`);
    }

    let relatedQuery = (RelatedModel as any).whereIn('id', uniqueForeignKeys);

    if (options.constraints) {
      relatedQuery = options.constraints(relatedQuery);
    }

    if (options.orderBy) {
      relatedQuery = relatedQuery.orderBy(options.orderBy.field, options.orderBy.direction);
    }

    const relatedModels = await relatedQuery.exec();

    // Create lookup map for related models
    const relatedMap = new Map();
    relatedModels.forEach((model: any) => relatedMap.set(model.id, model));

    // Step 5: Assign to instances
    instances.forEach((instance) => {
      const localKeyValue = (instance as any)[config.localKey || 'id'];
      const junctionRecordsForInstance = junctionGroups.get(localKeyValue) || [];

      const relatedForInstance = junctionRecordsForInstance
        .map((junction) => {
          const foreignKeyValue = (junction as any)[config.foreignKey];
          return relatedMap.get(foreignKeyValue);
        })
        .filter((related) => related != null);

      // Apply limit if specified
      const finalRelated = options.limit
        ? relatedForInstance.slice(0, options.limit)
        : relatedForInstance;

      instance.setRelation(relationshipName, finalRelated);

      // Cache individual relationship
      if (options.useCache !== false) {
        const cacheKey = this.cache.generateKey(instance, relationshipName, options.constraints);
        const modelType = finalRelated[0]?.constructor?.name || 'array';
        this.cache.set(cacheKey, finalRelated, modelType, config.type);
      }
    });
  }

  private groupInstancesByModel(instances: BaseModel[]): Map<string, BaseModel[]> {
    const groups = new Map<string, BaseModel[]>();

    instances.forEach((instance) => {
      const modelName = instance.constructor.name;
      if (!groups.has(modelName)) {
        groups.set(modelName, []);
      }
      groups.get(modelName)!.push(instance);
    });

    return groups;
  }

  // Cache management methods
  invalidateRelationshipCache(instance: BaseModel, relationshipName?: string): number {
    if (relationshipName) {
      const key = this.cache.generateKey(instance, relationshipName);
      return this.cache.invalidate(key) ? 1 : 0;
    } else {
      return this.cache.invalidateByInstance(instance);
    }
  }

  invalidateModelCache(modelName: string): number {
    return this.cache.invalidateByModel(modelName);
  }

  getRelationshipCacheStats(): any {
    return {
      cache: this.cache.getStats(),
      performance: this.cache.analyzePerformance(),
    };
  }

  // Preload relationships for better performance
  async warmupRelationshipCache(instances: BaseModel[], relationships: string[]): Promise<void> {
    await this.cache.warmup(instances, relationships, (instance, relationshipName) =>
      this.loadRelationship(instance, relationshipName, { useCache: false }),
    );
  }

  // Cleanup and maintenance
  cleanupExpiredCache(): number {
    return this.cache.cleanup();
  }

  clearRelationshipCache(): void {
    this.cache.clear();
  }
}
