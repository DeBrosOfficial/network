import { BaseModel } from '../models/BaseModel';
import { RelationshipConfig } from '../types/models';
import { RelationshipManager, RelationshipLoadOptions } from './RelationshipManager';

export interface LazyLoadPromise<T> extends Promise<T> {
  isLoaded(): boolean;
  getLoadedValue(): T | undefined;
  reload(options?: RelationshipLoadOptions): Promise<T>;
}

export class LazyLoader {
  private relationshipManager: RelationshipManager;

  constructor(relationshipManager: RelationshipManager) {
    this.relationshipManager = relationshipManager;
  }

  createLazyProperty<T>(
    instance: BaseModel,
    relationshipName: string,
    config: RelationshipConfig,
    options: RelationshipLoadOptions = {},
  ): LazyLoadPromise<T> {
    let loadPromise: Promise<T> | null = null;
    let loadedValue: T | undefined = undefined;
    let isLoaded = false;

    const loadRelationship = async (): Promise<T> => {
      if (loadPromise) {
        return loadPromise;
      }

      loadPromise = this.relationshipManager
        .loadRelationship(instance, relationshipName, options)
        .then((result: T) => {
          loadedValue = result;
          isLoaded = true;
          return result;
        })
        .catch((error) => {
          loadPromise = null; // Reset so it can be retried
          throw error;
        });

      return loadPromise;
    };

    const reload = async (newOptions?: RelationshipLoadOptions): Promise<T> => {
      // Clear cache for this relationship
      this.relationshipManager.invalidateRelationshipCache(instance, relationshipName);

      // Reset state
      loadPromise = null;
      loadedValue = undefined;
      isLoaded = false;

      // Load with new options
      const finalOptions = newOptions ? { ...options, ...newOptions } : options;
      return this.relationshipManager.loadRelationship(instance, relationshipName, finalOptions);
    };

    // Create the main promise
    const promise = loadRelationship() as LazyLoadPromise<T>;

    // Add custom methods
    promise.isLoaded = () => isLoaded;
    promise.getLoadedValue = () => loadedValue;
    promise.reload = reload;

    return promise;
  }

  createLazyPropertyWithProxy<T>(
    instance: BaseModel,
    relationshipName: string,
    config: RelationshipConfig,
    options: RelationshipLoadOptions = {},
  ): T {
    const lazyPromise = this.createLazyProperty<T>(instance, relationshipName, config, options);

    // For single relationships, return a proxy that loads on property access
    if (config.type === 'belongsTo' || config.type === 'hasOne') {
      return new Proxy({} as any, {
        get(target: any, prop: string | symbol) {
          // Special methods
          if (prop === 'then') {
            return lazyPromise.then.bind(lazyPromise);
          }
          if (prop === 'catch') {
            return lazyPromise.catch.bind(lazyPromise);
          }
          if (prop === 'finally') {
            return lazyPromise.finally.bind(lazyPromise);
          }
          if (prop === 'isLoaded') {
            return lazyPromise.isLoaded;
          }
          if (prop === 'reload') {
            return lazyPromise.reload;
          }

          // If already loaded, return the property from loaded value
          if (lazyPromise.isLoaded()) {
            const loadedValue = lazyPromise.getLoadedValue();
            return loadedValue ? (loadedValue as any)[prop] : undefined;
          }

          // Trigger loading and return undefined for now
          lazyPromise.catch(() => {}); // Prevent unhandled promise rejection
          return undefined;
        },

        has(target: any, prop: string | symbol) {
          if (lazyPromise.isLoaded()) {
            const loadedValue = lazyPromise.getLoadedValue();
            return loadedValue ? prop in (loadedValue as any) : false;
          }
          return false;
        },

        ownKeys(_target: any) {
          if (lazyPromise.isLoaded()) {
            const loadedValue = lazyPromise.getLoadedValue();
            return loadedValue ? Object.keys(loadedValue as any) : [];
          }
          return [];
        },
      });
    }

    // For collection relationships, return a proxy array
    if (config.type === 'hasMany' || config.type === 'manyToMany') {
      return new Proxy([] as any, {
        get(target: any[], prop: string | symbol) {
          // Array methods and properties
          if (prop === 'length') {
            if (lazyPromise.isLoaded()) {
              const loadedValue = lazyPromise.getLoadedValue() as any[];
              return loadedValue ? loadedValue.length : 0;
            }
            return 0;
          }

          // Promise methods
          if (prop === 'then') {
            return lazyPromise.then.bind(lazyPromise);
          }
          if (prop === 'catch') {
            return lazyPromise.catch.bind(lazyPromise);
          }
          if (prop === 'finally') {
            return lazyPromise.finally.bind(lazyPromise);
          }
          if (prop === 'isLoaded') {
            return lazyPromise.isLoaded;
          }
          if (prop === 'reload') {
            return lazyPromise.reload;
          }

          // Array methods that should trigger loading
          if (
            typeof prop === 'string' &&
            [
              'forEach',
              'map',
              'filter',
              'find',
              'some',
              'every',
              'reduce',
              'slice',
              'indexOf',
              'includes',
            ].includes(prop)
          ) {
            return async (...args: any[]) => {
              const loadedValue = await lazyPromise;
              return (loadedValue as any)[prop](...args);
            };
          }

          // Numeric index access
          if (typeof prop === 'string' && /^\d+$/.test(prop)) {
            if (lazyPromise.isLoaded()) {
              const loadedValue = lazyPromise.getLoadedValue() as any[];
              return loadedValue ? loadedValue[parseInt(prop, 10)] : undefined;
            }
            // Trigger loading
            lazyPromise.catch(() => {});
            return undefined;
          }

          // If already loaded, delegate to the actual array
          if (lazyPromise.isLoaded()) {
            const loadedValue = lazyPromise.getLoadedValue() as any[];
            return loadedValue ? (loadedValue as any)[prop] : undefined;
          }

          return undefined;
        },

        has(target: any[], prop: string | symbol) {
          if (lazyPromise.isLoaded()) {
            const loadedValue = lazyPromise.getLoadedValue() as any[];
            return loadedValue ? prop in loadedValue : false;
          }
          return false;
        },

        ownKeys(_target: any[]) {
          if (lazyPromise.isLoaded()) {
            const loadedValue = lazyPromise.getLoadedValue() as any[];
            return loadedValue ? Object.keys(loadedValue) : [];
          }
          return [];
        },
      }) as T;
    }

    // Fallback to promise for other types
    return lazyPromise as any;
  }

  // Helper method to check if a value is a lazy-loaded relationship
  static isLazyLoaded(value: any): value is LazyLoadPromise<any> {
    return (
      value &&
      typeof value === 'object' &&
      typeof value.then === 'function' &&
      typeof value.isLoaded === 'function' &&
      typeof value.reload === 'function'
    );
  }

  // Helper method to await all lazy relationships in an object
  static async resolveAllLazy(obj: any): Promise<any> {
    if (!obj || typeof obj !== 'object') {
      return obj;
    }

    if (Array.isArray(obj)) {
      return Promise.all(obj.map((item) => this.resolveAllLazy(item)));
    }

    const resolved: any = {};
    const promises: Array<Promise<void>> = [];

    for (const [key, value] of Object.entries(obj)) {
      if (this.isLazyLoaded(value)) {
        promises.push(
          value.then((resolvedValue) => {
            resolved[key] = resolvedValue;
          }),
        );
      } else {
        resolved[key] = value;
      }
    }

    await Promise.all(promises);
    return resolved;
  }

  // Helper method to get loaded relationships without triggering loading
  static getLoadedRelationships(instance: BaseModel): Record<string, any> {
    const loaded: Record<string, any> = {};

    const loadedRelations = instance.getLoadedRelations();
    for (const relationName of loadedRelations) {
      const value = instance.getRelation(relationName);
      if (this.isLazyLoaded(value)) {
        if (value.isLoaded()) {
          loaded[relationName] = value.getLoadedValue();
        }
      } else {
        loaded[relationName] = value;
      }
    }

    return loaded;
  }

  // Helper method to preload specific relationships
  static async preloadRelationships(
    instances: BaseModel[],
    relationships: string[],
    relationshipManager: RelationshipManager,
  ): Promise<void> {
    await relationshipManager.eagerLoadRelationships(instances, relationships);
  }

  // Helper method to create lazy collection with advanced features
  createLazyCollection<T extends BaseModel>(
    instance: BaseModel,
    relationshipName: string,
    config: RelationshipConfig,
    options: RelationshipLoadOptions = {},
  ): LazyCollection<T> {
    return new LazyCollection<T>(
      instance,
      relationshipName,
      config,
      options,
      this.relationshipManager,
    );
  }
}

// Advanced lazy collection with pagination and filtering
export class LazyCollection<T extends BaseModel> {
  private instance: BaseModel;
  private relationshipName: string;
  private config: RelationshipConfig;
  private options: RelationshipLoadOptions;
  private relationshipManager: RelationshipManager;
  private loadedItems: T[] = [];
  private isFullyLoaded = false;
  private currentPage = 1;
  private pageSize = 20;

  constructor(
    instance: BaseModel,
    relationshipName: string,
    config: RelationshipConfig,
    options: RelationshipLoadOptions,
    relationshipManager: RelationshipManager,
  ) {
    this.instance = instance;
    this.relationshipName = relationshipName;
    this.config = config;
    this.options = options;
    this.relationshipManager = relationshipManager;
  }

  async loadPage(page: number = 1, pageSize: number = this.pageSize): Promise<T[]> {
    const offset = (page - 1) * pageSize;

    const pageOptions: RelationshipLoadOptions = {
      ...this.options,
      constraints: (query) => {
        let q = query.offset(offset).limit(pageSize);
        if (this.options.constraints) {
          q = this.options.constraints(q);
        }
        return q;
      },
    };

    const pageItems = (await this.relationshipManager.loadRelationship(
      this.instance,
      this.relationshipName,
      pageOptions,
    )) as T[];

    // Update loaded items if this is sequential loading
    if (page === this.currentPage) {
      this.loadedItems.push(...pageItems);
      this.currentPage++;

      if (pageItems.length < pageSize) {
        this.isFullyLoaded = true;
      }
    }

    return pageItems;
  }

  async loadMore(count: number = this.pageSize): Promise<T[]> {
    return this.loadPage(this.currentPage, count);
  }

  async loadAll(): Promise<T[]> {
    if (this.isFullyLoaded) {
      return this.loadedItems;
    }

    const allItems = (await this.relationshipManager.loadRelationship(
      this.instance,
      this.relationshipName,
      this.options,
    )) as T[];

    this.loadedItems = allItems;
    this.isFullyLoaded = true;

    return allItems;
  }

  getLoadedItems(): T[] {
    return [...this.loadedItems];
  }

  isLoaded(): boolean {
    return this.loadedItems.length > 0;
  }

  isCompletelyLoaded(): boolean {
    return this.isFullyLoaded;
  }

  async filter(predicate: (item: T) => boolean): Promise<T[]> {
    if (!this.isFullyLoaded) {
      await this.loadAll();
    }
    return this.loadedItems.filter(predicate);
  }

  async find(predicate: (item: T) => boolean): Promise<T | undefined> {
    // Try loaded items first
    const found = this.loadedItems.find(predicate);
    if (found) {
      return found;
    }

    // If not fully loaded, load all and search
    if (!this.isFullyLoaded) {
      await this.loadAll();
      return this.loadedItems.find(predicate);
    }

    return undefined;
  }

  async count(): Promise<number> {
    if (this.isFullyLoaded) {
      return this.loadedItems.length;
    }

    // For a complete count, we need to load all items
    // In a more sophisticated implementation, we might have a separate count query
    await this.loadAll();
    return this.loadedItems.length;
  }

  clear(): void {
    this.loadedItems = [];
    this.isFullyLoaded = false;
    this.currentPage = 1;
  }
}
