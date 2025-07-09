import { StoreType, ValidationResult, ShardingConfig, PinningConfig } from '../types/framework';
import { FieldConfig, RelationshipConfig, ValidationError } from '../types/models';
import { QueryBuilder } from '../query/QueryBuilder';

export abstract class BaseModel {
  // Instance properties
  public id: string = '';
  public _loadedRelations: Map<string, any> = new Map();
  protected _isDirty: boolean = false;
  protected _isNew: boolean = true;

  // Static properties for model configuration
  static modelName: string;
  static storeType: StoreType = 'docstore';
  static scope: 'user' | 'global' = 'global';
  static sharding?: ShardingConfig;
  static pinning?: PinningConfig;
  static fields: Map<string, FieldConfig> = new Map();
  static relationships: Map<string, RelationshipConfig> = new Map();
  static hooks: Map<string, Function[]> = new Map();

  constructor(data: any = {}) {
    // Generate ID first
    this.id = this.generateId();

    // Apply field defaults first
    this.applyFieldDefaults();

    // Then apply provided data, but only for properties that are explicitly provided
    if (data && typeof data === 'object') {
      Object.keys(data).forEach((key) => {
        if (
          key !== '_loadedRelations' &&
          key !== '_isDirty' &&
          key !== '_isNew' &&
          data[key] !== undefined
        ) {
          // Always set directly - the Field decorator's setter will handle validation and transformation
          try {
            (this as any)[key] = data[key];
          } catch (error) {
            console.error(`Error setting field ${key}:`, error);
            // If Field setter fails, set the private key directly
            const privateKey = `_${key}`;
            (this as any)[privateKey] = data[key];
          }
        }
      });

      // Mark as existing if it has an ID in the data
      if (data.id) {
        this._isNew = false;
      }
    }

    // Remove any instance properties that might shadow prototype getters
    this.cleanupShadowingProperties();
  }

  private cleanupShadowingProperties(): void {
    const modelClass = this.constructor as typeof BaseModel;

    // For each field, ensure no instance properties are shadowing prototype getters
    for (const [fieldName] of modelClass.fields) {
      // If there's an instance property, remove it and create a working getter
      if (this.hasOwnProperty(fieldName)) {
        const _oldValue = (this as any)[fieldName];
        delete (this as any)[fieldName];

        // Define a working getter directly on the instance
        Object.defineProperty(this, fieldName, {
          get: () => {
            const privateKey = `_${fieldName}`;
            return (this as any)[privateKey];
          },
          set: (value: any) => {
            const privateKey = `_${fieldName}`;
            (this as any)[privateKey] = value;
            this.markFieldAsModified(fieldName);
          },
          enumerable: true,
          configurable: true,
        });
      }
    }
  }

  // Core CRUD operations
  async save(): Promise<this> {
    if (this._isNew) {
      // Clean up any instance properties before hooks run
      this.cleanupShadowingProperties();

      await this.beforeCreate();

      // Clean up any instance properties created by hooks
      this.cleanupShadowingProperties();

      // Generate ID if not provided
      if (!this.id) {
        this.id = this.generateId();
      }

      // Set timestamps using Field setters
      const now = Date.now();
      this.setFieldValue('createdAt', now);
      this.setFieldValue('updatedAt', now);

      // Clean up any additional shadowing properties after setting timestamps
      this.cleanupShadowingProperties();

      // Validate after all field generation is complete
      await this.validate();

      // Save to database (will be implemented when database manager is ready)
      await this._saveToDatabase();

      this._isNew = false;
      this.clearModifications();

      await this.afterCreate();

      // Clean up any shadowing properties created during save
      this.cleanupShadowingProperties();
    } else if (this._isDirty) {
      await this.beforeUpdate();

      // Set timestamp using Field setter
      this.setFieldValue('updatedAt', Date.now());

      // Validate after hooks have run
      await this.validate();

      // Update in database
      await this._updateInDatabase();

      this.clearModifications();

      await this.afterUpdate();

      // Clean up any shadowing properties created during save
      this.cleanupShadowingProperties();
    }

    return this;
  }

  static async create<T extends BaseModel>(this: new (data?: any) => T, data: any): Promise<T> {
    const instance = new this(data);
    return await instance.save();
  }

  static async get<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
    id: string,
  ): Promise<T | null> {
    return await this.findById(id);
  }

  static async findById<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
    id: string,
  ): Promise<T | null> {
    // Use the mock framework for testing
    const framework = (globalThis as any).__debrosFramework || this.getMockFramework();
    if (!framework) {
      return null;
    }

    try {
      const modelClass = this as any;
      let data = null;

      if (modelClass.scope === 'user') {
        // For user-scoped models, we would need userId - for now, try global
        const database = await framework.databaseManager?.getGlobalDatabase?.(
          modelClass.modelName || modelClass.name,
        );
        if (database && framework.databaseManager?.getDocument) {
          data = await framework.databaseManager.getDocument(database, modelClass.storeType, id);
        }
      } else {
        if (modelClass.sharding) {
          const shard = framework.shardManager?.getShardForKey?.(
            modelClass.modelName || modelClass.name,
            id,
          );
          if (shard && framework.databaseManager?.getDocument) {
            data = await framework.databaseManager.getDocument(
              shard.database,
              modelClass.storeType,
              id,
            );
          }
        } else {
          const database = await framework.databaseManager?.getGlobalDatabase?.(
            modelClass.modelName || modelClass.name,
          );
          if (database && framework.databaseManager?.getDocument) {
            data = await framework.databaseManager.getDocument(database, modelClass.storeType, id);
          }
        }
      }

      if (data) {
        const instance = new (this as any)(data);
        instance._isNew = false;
        instance.clearModifications();
        return instance;
      }

      return null;
    } catch (error) {
      console.error('Failed to find by ID:', error);
      return null;
    }
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

  static async findAll<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
  ): Promise<T[]> {
    return await this.all();
  }

  static async findOne<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
    criteria: any,
  ): Promise<T | null> {
    const query = new QueryBuilder<T>(this as any);

    // Apply criteria as where clauses
    Object.keys(criteria).forEach((key) => {
      query.where(key, '=', criteria[key]);
    });

    const results = await query.limit(1).exec();
    return results.length > 0 ? results[0] : null;
  }

  static async count<T extends BaseModel>(
    this: typeof BaseModel & (new (data?: any) => T),
  ): Promise<number> {
    return await new QueryBuilder<T>(this as any).count();
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
    const modelClass = this.constructor as typeof BaseModel;

    // Include all field values using private keys (more reliable than getters)
    for (const [fieldName] of modelClass.fields) {
      const privateKey = `_${fieldName}`;
      const value = (this as any)[privateKey];
      if (value !== undefined) {
        result[fieldName] = value;
      }
    }

    // Include basic properties
    result.id = this.id;

    // For OrbitDB docstore compatibility, also include _id field
    if (modelClass.storeType === 'docstore') {
      result._id = this.id;
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

    // Validate each field using private keys (more reliable)
    for (const [fieldName, fieldConfig] of modelClass.fields) {
      const privateKey = `_${fieldName}`;
      const value = (this as any)[privateKey];

      const fieldErrors = await this.validateField(fieldName, value, fieldConfig);
      errors.push(...fieldErrors);
    }

    const result = { valid: errors.length === 0, errors };

    if (!result.valid) {
      throw new ValidationError(errors);
    }

    return result;
  }

  private async validateField(
    fieldName: string,
    value: any,
    config: FieldConfig,
  ): Promise<string[]> {
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

    // Unique constraint validation
    if (config.unique && value !== undefined && value !== null && value !== '') {
      const modelClass = this.constructor as typeof BaseModel;
      try {
        const existing = await (modelClass as any).findOne({ [fieldName]: value });
        if (existing && existing.id !== this.id) {
          errors.push(`${fieldName} must be unique`);
        }
      } catch (error) {
        // If we can't query for duplicates, skip unique validation
        console.warn(`Could not validate unique constraint for ${fieldName}:`, error);
      }
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
    const hookNames = modelClass.hooks.get(hookName) || [];

    for (const hookMethodName of hookNames) {
      const hookMethod = (this as any)[String(hookMethodName)];
      if (typeof hookMethod === 'function') {
        await hookMethod.call(this);
      }
    }
  }

  // Utility methods
  private generateId(): string {
    return Date.now().toString(36) + Math.random().toString(36).substr(2);
  }

  private applyFieldDefaults(): void {
    const modelClass = this.constructor as typeof BaseModel;

    // Ensure we have fields map
    if (!modelClass.fields) {
      return;
    }

    for (const [fieldName, fieldConfig] of modelClass.fields) {
      if (fieldConfig.default !== undefined) {
        const privateKey = `_${fieldName}`;
        const hasProperty = (this as any).hasOwnProperty(privateKey);
        const currentValue = (this as any)[privateKey];

        // Always apply default value to private field if it's not set
        if (!hasProperty || currentValue === undefined) {
          // Apply default value to private field
          if (typeof fieldConfig.default === 'function') {
            (this as any)[privateKey] = fieldConfig.default();
          } else {
            (this as any)[privateKey] = fieldConfig.default;
          }
        }
      }
    }
  }

  // Field modification tracking
  private _modifiedFields: Set<string> = new Set();

  markFieldAsModified(fieldName: string): void {
    this._modifiedFields.add(fieldName);
    this._isDirty = true;
  }

  getModifiedFields(): string[] {
    return Array.from(this._modifiedFields);
  }

  isFieldModified(fieldName: string): boolean {
    return this._modifiedFields.has(fieldName);
  }

  clearModifications(): void {
    this._modifiedFields.clear();
    this._isDirty = false;
  }

  // Reliable field access methods that bypass problematic getters
  getFieldValue(fieldName: string): any {
    // Always ensure this field's getter works properly
    this.ensureFieldGetter(fieldName);

    // Try private key first
    const privateKey = `_${fieldName}`;
    let value = (this as any)[privateKey];

    // If private key is undefined, try the property getter as fallback
    if (value === undefined) {
      try {
        value = (this as any)[fieldName];
      } catch (error) {
        console.warn(`Failed to access field ${fieldName} using getter:`, error);
        // Ignore errors from getter
      }
    }

    return value;
  }

  private ensureFieldGetter(fieldName: string): void {
    // If there's a shadowing instance property, remove it and create a working getter
    if (this.hasOwnProperty(fieldName)) {
      delete (this as any)[fieldName];

      // Define a working getter directly on the instance
      Object.defineProperty(this, fieldName, {
        get: () => {
          const privateKey = `_${fieldName}`;
          return (this as any)[privateKey];
        },
        set: (value: any) => {
          const privateKey = `_${fieldName}`;
          (this as any)[privateKey] = value;
          this.markFieldAsModified(fieldName);
        },
        enumerable: true,
        configurable: true,
      });
    }
  }

  setFieldValue(fieldName: string, value: any): void {
    // Try to use the Field decorator's setter first
    try {
      (this as any)[fieldName] = value;
    } catch (_error) {
      // Fallback to setting private key directly
      const privateKey = `_${fieldName}`;
      (this as any)[privateKey] = value;
      this.markFieldAsModified(fieldName);
    }
  }

  getAllFieldValues(): Record<string, any> {
    const modelClass = this.constructor as typeof BaseModel;
    const values: Record<string, any> = {};

    for (const [fieldName] of modelClass.fields) {
      const value = this.getFieldValue(fieldName);
      if (value !== undefined) {
        values[fieldName] = value;
      }
    }

    return values;
  }

  // Ensure user databases exist for user-scoped models
  private async ensureUserDatabasesExist(framework: any, userId: string): Promise<void> {
    try {
      // Try to get user databases - if this fails, they don't exist
      await framework.databaseManager.getUserMappings(userId);
    } catch (error) {
      // If user not found, create databases for them
      if ((error as Error).message.includes('not found in directory')) {
        console.log(`Creating databases for user ${userId}`);
        await framework.databaseManager.createUserDatabases(userId);
      } else {
        throw error;
      }
    }
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
        // For user-scoped models, we need a userId (check common field names)
        const userId =
          this.getFieldValue('userId') ||
          this.getFieldValue('authorId') ||
          this.getFieldValue('ownerId');
        if (!userId) {
          throw new Error('User-scoped models must have a userId, authorId, or ownerId field');
        }

        // Ensure user databases exist before accessing them
        await this.ensureUserDatabasesExist(framework, userId);

        // Ensure user databases exist before accessing them
        await this.ensureUserDatabasesExist(framework, userId);

        const database = await framework.databaseManager.getUserDatabase(
          userId,
          modelClass.modelName,
        );
        await framework.databaseManager.addDocument(database, modelClass.storeType, this.toJSON());
      } else {
        // For global models
        if (modelClass.sharding) {
          // Use sharded database
          const shard = framework.shardManager.getShardForKey(modelClass.modelName, this.id);
          await framework.databaseManager.addDocument(
            shard.database,
            modelClass.storeType,
            this.toJSON(),
          );
        } else {
          // Use single global database
          const database = await framework.databaseManager.getGlobalDatabase(modelClass.modelName);
          await framework.databaseManager.addDocument(
            database,
            modelClass.storeType,
            this.toJSON(),
          );
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
        const userId =
          this.getFieldValue('userId') ||
          this.getFieldValue('authorId') ||
          this.getFieldValue('ownerId');
        if (!userId) {
          throw new Error('User-scoped models must have a userId, authorId, or ownerId field');
        }

        // Ensure user databases exist before accessing them
        await this.ensureUserDatabasesExist(framework, userId);

        const database = await framework.databaseManager.getUserDatabase(
          userId,
          modelClass.modelName,
        );
        await framework.databaseManager.updateDocument(
          database,
          modelClass.storeType,
          this.id,
          this.toJSON(),
        );
      } else {
        if (modelClass.sharding) {
          const shard = framework.shardManager.getShardForKey(modelClass.modelName, this.id);
          await framework.databaseManager.updateDocument(
            shard.database,
            modelClass.storeType,
            this.id,
            this.toJSON(),
          );
        } else {
          const database = await framework.databaseManager.getGlobalDatabase(modelClass.modelName);
          await framework.databaseManager.updateDocument(
            database,
            modelClass.storeType,
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
        const userId =
          this.getFieldValue('userId') ||
          this.getFieldValue('authorId') ||
          this.getFieldValue('ownerId');
        if (!userId) {
          throw new Error('User-scoped models must have a userId, authorId, or ownerId field');
        }

        // Ensure user databases exist before accessing them
        await this.ensureUserDatabasesExist(framework, userId);

        const database = await framework.databaseManager.getUserDatabase(
          userId,
          modelClass.modelName,
        );
        await framework.databaseManager.deleteDocument(database, modelClass.storeType, this.id);
      } else {
        if (modelClass.sharding) {
          const shard = framework.shardManager.getShardForKey(modelClass.modelName, this.id);
          await framework.databaseManager.deleteDocument(
            shard.database,
            modelClass.storeType,
            this.id,
          );
        } else {
          const database = await framework.databaseManager.getGlobalDatabase(modelClass.modelName);
          await framework.databaseManager.deleteDocument(database, modelClass.storeType, this.id);
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
    const framework = (globalThis as any).__debrosFramework;
    if (!framework) {
      // Try to get mock framework for testing
      const mockFramework = (this.constructor as any).getMockFramework?.();
      return mockFramework;
    }
    return framework;
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

  // Mock framework for testing
  static getMockFramework(): any {
    if (typeof jest !== 'undefined') {
      // Create a simple mock framework with shared mock database storage
      if (!(globalThis as any).__mockDatabase) {
        (globalThis as any).__mockDatabase = new Map();
      }

      const mockDatabase = {
        _data: (globalThis as any).__mockDatabase,
        async get(id: string) {
          return this._data.get(id) || null;
        },
        async put(doc: any) {
          const id = doc._id || doc.id;
          this._data.set(id, doc);
          return id;
        },
        async del(id: string) {
          return this._data.delete(id);
        },
        async all() {
          return Array.from(this._data.values());
        },
      };

      return {
        databaseManager: {
          async getGlobalDatabase(_name: string) {
            return mockDatabase;
          },
          async getUserDatabase(_userId: string, _name: string) {
            return mockDatabase;
          },
          async getUserMappings(_userId: string) {
            // Mock user mappings - return a simple mapping
            return { userId: _userId, databases: {} };
          },
          async createUserDatabases(_userId: string) {
            // Mock user database creation - do nothing for tests
            return;
          },
          async getDocument(_database: any, _type: string, id: string) {
            return await mockDatabase.get(id);
          },
          async addDocument(_database: any, _type: string, doc: any) {
            return await mockDatabase.put(doc);
          },
          async updateDocument(_database: any, _type: string, id: string, doc: any) {
            doc.id = id;
            return await mockDatabase.put(doc);
          },
          async deleteDocument(_database: any, _type: string, id: string) {
            return await mockDatabase.del(id);
          },
          async getAllDocuments(_database: any, _type: string) {
            return await mockDatabase.all();
          },
        },
        shardManager: {
          getShardForKey(_modelName: string, _key: string) {
            return { database: mockDatabase };
          },
        },
      };
    }
    return null;
  }
}
