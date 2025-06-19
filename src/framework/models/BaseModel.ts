import { StoreType, ValidationResult, ShardingConfig, PinningConfig } from '../types/framework';
import { FieldConfig, RelationshipConfig, ValidationError } from '../types/models';
import { QueryBuilder } from '../query/QueryBuilder';

export abstract class BaseModel {
  // Instance properties
  public id: string = '';
  public createdAt: number = 0;
  public updatedAt: number = 0;
  public _loadedRelations: Map<string, any> = new Map();
  protected _isDirty: boolean = false;
  protected _isNew: boolean = true;

  // Static properties for model configuration
  static modelName: string;
  static dbType: StoreType = 'docstore';
  static scope: 'user' | 'global' = 'global';
  static sharding?: ShardingConfig;
  static pinning?: PinningConfig;
  static fields: Map<string, FieldConfig> = new Map();
  static relationships: Map<string, RelationshipConfig> = new Map();
  static hooks: Map<string, Function[]> = new Map();

  constructor(data: any = {}) {
    this.fromJSON(data);
  }

  // Core CRUD operations
  async save(): Promise<this> {
    await this.validate();

    if (this._isNew) {
      await this.beforeCreate();

      // Generate ID if not provided
      if (!this.id) {
        this.id = this.generateId();
      }

      this.createdAt = Date.now();
      this.updatedAt = this.createdAt;

      // Save to database (will be implemented when database manager is ready)
      await this._saveToDatabase();

      this._isNew = false;
      this._isDirty = false;

      await this.afterCreate();
    } else if (this._isDirty) {
      await this.beforeUpdate();

      this.updatedAt = Date.now();

      // Update in database
      await this._updateInDatabase();

      this._isDirty = false;

      await this.afterUpdate();
    }

    return this;
  }

  static async create<T extends BaseModel>(this: new (data?: any) => T, data: any): Promise<T> {
    const instance = new this(data);
    return await instance.save();
  }

  static async get<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
    _id: string,
  ): Promise<T | null> {
    // Will be implemented when query system is ready
    throw new Error('get method not yet implemented - requires query system');
  }

  static async find<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
    id: string,
  ): Promise<T> {
    const result = await this.get(id);
    if (!result) {
      throw new Error(`${this.name} with id ${id} not found`);
    }
    return result;
  }

  async update(data: Partial<this>): Promise<this> {
    Object.assign(this, data);
    this._isDirty = true;
    return await this.save();
  }

  async delete(): Promise<boolean> {
    await this.beforeDelete();

    // Delete from database (will be implemented when database manager is ready)
    const success = await this._deleteFromDatabase();

    if (success) {
      await this.afterDelete();
    }

    return success;
  }

  // Query operations (return QueryBuilder instances)
  static where<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
    field: string,
    operator: string,
    value: any,
  ): QueryBuilder<T> {
    return new QueryBuilder<T>(this as any).where(field, operator, value);
  }

  static whereIn<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
    field: string,
    values: any[],
  ): QueryBuilder<T> {
    return new QueryBuilder<T>(this as any).whereIn(field, values);
  }

  static orderBy<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
    field: string,
    direction: 'asc' | 'desc' = 'asc',
  ): QueryBuilder<T> {
    return new QueryBuilder<T>(this as any).orderBy(field, direction);
  }

  static limit<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
    count: number,
  ): QueryBuilder<T> {
    return new QueryBuilder<T>(this as any).limit(count);
  }

  static async all<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
  ): Promise<T[]> {
    return await new QueryBuilder<T>(this as any).exec();
  }

  // Relationship operations
  async load(relationships: string[]): Promise<this> {
    const framework = this.getFrameworkInstance();
    if (!framework?.relationshipManager) {
      console.warn('RelationshipManager not available, skipping relationship loading');
      return this;
    }

    await framework.relationshipManager.eagerLoadRelationships([this], relationships);
    return this;
  }

  async loadRelation(relationName: string): Promise<any> {
    // Check if already loaded
    if (this._loadedRelations.has(relationName)) {
      return this._loadedRelations.get(relationName);
    }

    const framework = this.getFrameworkInstance();
    if (!framework?.relationshipManager) {
      console.warn('RelationshipManager not available, cannot load relationship');
      return null;
    }

    return await framework.relationshipManager.loadRelationship(this, relationName);
  }

  // Advanced relationship loading methods
  async loadRelationWithConstraints(
    relationName: string,
    constraints: (query: any) => any,
  ): Promise<any> {
    const framework = this.getFrameworkInstance();
    if (!framework?.relationshipManager) {
      console.warn('RelationshipManager not available, cannot load relationship');
      return null;
    }

    return await framework.relationshipManager.loadRelationship(this, relationName, {
      constraints,
    });
  }

  async reloadRelation(relationName: string): Promise<any> {
    // Clear cached relationship
    this._loadedRelations.delete(relationName);

    const framework = this.getFrameworkInstance();
    if (framework?.relationshipManager) {
      framework.relationshipManager.invalidateRelationshipCache(this, relationName);
    }

    return await this.loadRelation(relationName);
  }

  getLoadedRelations(): string[] {
    return Array.from(this._loadedRelations.keys());
  }

  isRelationLoaded(relationName: string): boolean {
    return this._loadedRelations.has(relationName);
  }

  getRelation(relationName: string): any {
    return this._loadedRelations.get(relationName);
  }

  setRelation(relationName: string, value: any): void {
    this._loadedRelations.set(relationName, value);
  }

  clearRelation(relationName: string): void {
    this._loadedRelations.delete(relationName);
  }

  // Serialization
  toJSON(): any {
    const result: any = {};

    // Include all enumerable properties
    for (const key in this) {
      if (this.hasOwnProperty(key) && !key.startsWith('_')) {
        result[key] = (this as any)[key];
      }
    }

    // Include loaded relations
    this._loadedRelations.forEach((value, key) => {
      result[key] = value;
    });

    return result;
  }

  fromJSON(data: any): this {
    if (!data) return this;

    // Set basic properties  
    Object.keys(data).forEach((key) => {
      if (key !== '_loadedRelations' && key !== '_isDirty' && key !== '_isNew') {
        (this as any)[key] = data[key];
      }
    });

    // Mark as existing if it has an ID
    if (this.id) {
      this._isNew = false;
    }

    return this;
  }

  // Validation
  async validate(): Promise<ValidationResult> {
    const errors: string[] = [];
    const modelClass = this.constructor as typeof BaseModel;

    // Validate each field
    for (const [fieldName, fieldConfig] of modelClass.fields) {
      const value = (this as any)[fieldName];
      const fieldErrors = this.validateField(fieldName, value, fieldConfig);
      errors.push(...fieldErrors);
    }

    const result = { valid: errors.length === 0, errors };

    if (!result.valid) {
      throw new ValidationError(errors);
    }

    return result;
  }

  private validateField(fieldName: string, value: any, config: FieldConfig): string[] {
    const errors: string[] = [];

    // Required validation
    if (config.required && (value === undefined || value === null || value === '')) {
      errors.push(`${fieldName} is required`);
      return errors; // No point in further validation if required field is missing
    }

    // Skip further validation if value is empty and not required
    if (value === undefined || value === null) {
      return errors;
    }

    // Type validation
    if (!this.isValidType(value, config.type)) {
      errors.push(`${fieldName} must be of type ${config.type}`);
    }

    // Custom validation
    if (config.validate) {
      const customResult = config.validate(value);
      if (customResult === false) {
        errors.push(`${fieldName} failed custom validation`);
      } else if (typeof customResult === 'string') {
        errors.push(customResult);
      }
    }

    return errors;
  }

  private isValidType(value: any, expectedType: FieldConfig['type']): boolean {
    switch (expectedType) {
      case 'string':
        return typeof value === 'string';
      case 'number':
        return typeof value === 'number' && !isNaN(value);
      case 'boolean':
        return typeof value === 'boolean';
      case 'array':
        return Array.isArray(value);
      case 'object':
        return typeof value === 'object' && !Array.isArray(value);
      case 'date':
        return value instanceof Date || (typeof value === 'number' && !isNaN(value));
      default:
        return true;
    }
  }

  // Hook methods (can be overridden by subclasses)
  async beforeCreate(): Promise<void> {
    await this.runHooks('beforeCreate');
  }

  async afterCreate(): Promise<void> {
    await this.runHooks('afterCreate');
  }

  async beforeUpdate(): Promise<void> {
    await this.runHooks('beforeUpdate');
  }

  async afterUpdate(): Promise<void> {
    await this.runHooks('afterUpdate');
  }

  async beforeDelete(): Promise<void> {
    await this.runHooks('beforeDelete');
  }

  async afterDelete(): Promise<void> {
    await this.runHooks('afterDelete');
  }

  private async runHooks(hookName: string): Promise<void> {
    const modelClass = this.constructor as typeof BaseModel;
    const hooks = modelClass.hooks.get(hookName) || [];

    for (const hook of hooks) {
      await hook.call(this);
    }
  }

  // Utility methods
  private generateId(): string {
    return Date.now().toString(36) + Math.random().toString(36).substr(2);
  }

  // Database operations integrated with DatabaseManager
  private async _saveToDatabase(): Promise<void> {
    const framework = this.getFrameworkInstance();
    if (!framework) {
      console.warn('Framework not initialized, skipping database save');
      return;
    }

    const modelClass = this.constructor as typeof BaseModel;

    try {
      if (modelClass.scope === 'user') {
        // For user-scoped models, we need a userId
        const userId = (this as any).userId;
        if (!userId) {
          throw new Error('User-scoped models must have a userId field');
        }

        const database = await framework.databaseManager.getUserDatabase(
          userId,
          modelClass.modelName,
        );
        await framework.databaseManager.addDocument(database, modelClass.dbType, this.toJSON());
      } else {
        // For global models
        if (modelClass.sharding) {
          // Use sharded database
          const shard = framework.shardManager.getShardForKey(modelClass.modelName, this.id);
          await framework.databaseManager.addDocument(
            shard.database,
            modelClass.dbType,
            this.toJSON(),
          );
        } else {
          // Use single global database
          const database = await framework.databaseManager.getGlobalDatabase(modelClass.modelName);
          await framework.databaseManager.addDocument(database, modelClass.dbType, this.toJSON());
        }
      }
    } catch (error) {
      console.error('Failed to save to database:', error);
      throw error;
    }
  }

  private async _updateInDatabase(): Promise<void> {
    const framework = this.getFrameworkInstance();
    if (!framework) {
      console.warn('Framework not initialized, skipping database update');
      return;
    }

    const modelClass = this.constructor as typeof BaseModel;

    try {
      if (modelClass.scope === 'user') {
        const userId = (this as any).userId;
        if (!userId) {
          throw new Error('User-scoped models must have a userId field');
        }

        const database = await framework.databaseManager.getUserDatabase(
          userId,
          modelClass.modelName,
        );
        await framework.databaseManager.updateDocument(
          database,
          modelClass.dbType,
          this.id,
          this.toJSON(),
        );
      } else {
        if (modelClass.sharding) {
          const shard = framework.shardManager.getShardForKey(modelClass.modelName, this.id);
          await framework.databaseManager.updateDocument(
            shard.database,
            modelClass.dbType,
            this.id,
            this.toJSON(),
          );
        } else {
          const database = await framework.databaseManager.getGlobalDatabase(modelClass.modelName);
          await framework.databaseManager.updateDocument(
            database,
            modelClass.dbType,
            this.id,
            this.toJSON(),
          );
        }
      }
    } catch (error) {
      console.error('Failed to update in database:', error);
      throw error;
    }
  }

  private async _deleteFromDatabase(): Promise<boolean> {
    const framework = this.getFrameworkInstance();
    if (!framework) {
      console.warn('Framework not initialized, skipping database delete');
      return false;
    }

    const modelClass = this.constructor as typeof BaseModel;

    try {
      if (modelClass.scope === 'user') {
        const userId = (this as any).userId;
        if (!userId) {
          throw new Error('User-scoped models must have a userId field');
        }

        const database = await framework.databaseManager.getUserDatabase(
          userId,
          modelClass.modelName,
        );
        await framework.databaseManager.deleteDocument(database, modelClass.dbType, this.id);
      } else {
        if (modelClass.sharding) {
          const shard = framework.shardManager.getShardForKey(modelClass.modelName, this.id);
          await framework.databaseManager.deleteDocument(
            shard.database,
            modelClass.dbType,
            this.id,
          );
        } else {
          const database = await framework.databaseManager.getGlobalDatabase(modelClass.modelName);
          await framework.databaseManager.deleteDocument(database, modelClass.dbType, this.id);
        }
      }
      return true;
    } catch (error) {
      console.error('Failed to delete from database:', error);
      throw error;
    }
  }

  private getFrameworkInstance(): any {
    // This will be properly typed when DebrosFramework is created
    return (globalThis as any).__debrosFramework;
  }

  // Static methods for framework integration
  static setStore(store: any): void {
    (this as any)._store = store;
  }

  static setShards(shards: any[]): void {
    (this as any)._shards = shards;
  }

  static getStore(): any {
    return (this as any)._store;
  }

  static getShards(): any[] {
    return (this as any)._shards || [];
  }

  static fromJSON<T extends BaseModel>(this: new (data?: any) => T, data: any): T {
    const instance = new this();
    Object.assign(instance, data);
    return instance;
  }

  static query<T extends BaseModel>(this: typeof BaseModel & (new (data?: any) => T)): any {
    const { QueryBuilder } = require('../query/QueryBuilder');
    return new QueryBuilder(this);
  }
}
