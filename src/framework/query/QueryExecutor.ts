import { BaseModel } from '../models/BaseModel';
import { QueryBuilder } from './QueryBuilder';
import { QueryCondition } from '../types/queries';
import { StoreType } from '../types/framework';
import { QueryOptimizer, QueryPlan } from './QueryOptimizer';

export class QueryExecutor<T extends BaseModel> {
  private model: typeof BaseModel;
  private query: QueryBuilder<T>;
  private framework: any; // Will be properly typed later
  private queryPlan?: QueryPlan;
  private useCache: boolean = true;

  constructor(model: typeof BaseModel, query: QueryBuilder<T>) {
    this.model = model;
    this.query = query;
    this.framework = this.getFrameworkInstance();
  }

  async execute(): Promise<T[]> {
    const startTime = Date.now();
    console.log(`🔍 Executing query for ${this.model.name} (${this.model.scope})`);

    // Generate query plan for optimization
    this.queryPlan = QueryOptimizer.analyzeQuery(this.query);
    console.log(
      `📊 Query plan: ${this.queryPlan.strategy} (cost: ${this.queryPlan.estimatedCost})`,
    );

    // Check cache first if enabled
    if (this.useCache && this.framework.queryCache) {
      const cached = await this.framework.queryCache.get(this.query);
      if (cached) {
        console.log(`⚡ Cache hit for ${this.model.name} query`);
        return cached;
      }
    }

    // Execute query based on scope
    let results: T[];
    if (this.model.scope === 'user') {
      results = await this.executeUserScopedQuery();
    } else {
      results = await this.executeGlobalQuery();
    }

    // Cache results if enabled
    if (this.useCache && this.framework.queryCache && results.length > 0) {
      this.framework.queryCache.set(this.query, results);
    }

    const duration = Date.now() - startTime;
    console.log(`✅ Query completed in ${duration}ms, returned ${results.length} results`);

    return results;
  }

  async count(): Promise<number> {
    const results = await this.execute();
    return results.length;
  }

  async sum(field: string): Promise<number> {
    const results = await this.execute();
    return results.reduce((sum, item) => {
      const value = this.getNestedValue(item, field);
      return sum + (typeof value === 'number' ? value : 0);
    }, 0);
  }

  async avg(field: string): Promise<number> {
    const results = await this.execute();
    if (results.length === 0) return 0;

    const sum = await this.sum(field);
    return sum / results.length;
  }

  async min(field: string): Promise<any> {
    const results = await this.execute();
    if (results.length === 0) return null;

    return results.reduce((min, item) => {
      const value = this.getNestedValue(item, field);
      return min === null || value < min ? value : min;
    }, null);
  }

  async max(field: string): Promise<any> {
    const results = await this.execute();
    if (results.length === 0) return null;

    return results.reduce((max, item) => {
      const value = this.getNestedValue(item, field);
      return max === null || value > max ? value : max;
    }, null);
  }

  private async executeUserScopedQuery(): Promise<T[]> {
    const conditions = this.query.getConditions();

    // Check if we have user-specific filters
    const userFilter = conditions.find((c) => c.field === 'userId' || c.operator === 'userIn');

    if (userFilter) {
      return await this.executeUserSpecificQuery(userFilter);
    } else {
      // Global query on user-scoped data - use global index
      return await this.executeGlobalIndexQuery();
    }
  }

  private async executeUserSpecificQuery(userFilter: QueryCondition): Promise<T[]> {
    const userIds = userFilter.operator === 'userIn' ? userFilter.value : [userFilter.value];

    console.log(`👤 Querying user databases for ${userIds.length} users`);

    const results: T[] = [];

    // Query each user's database in parallel
    const promises = userIds.map(async (userId: string) => {
      try {
        const userDB = await this.framework.databaseManager.getUserDatabase(
          userId,
          this.model.modelName,
        );

        return await this.queryDatabase(userDB, this.model.storeType);
      } catch (error) {
        console.warn(`Failed to query user ${userId} database:`, error);
        return [];
      }
    });

    const userResults = await Promise.all(promises);

    // Flatten and combine results
    for (const userResult of userResults) {
      results.push(...userResult);
    }

    return this.postProcessResults(results);
  }

  private async executeGlobalIndexQuery(): Promise<T[]> {
    console.log(`📇 Querying global index for ${this.model.name}`);

    // Query global index for user-scoped models
    const globalIndexName = `${this.model.modelName}GlobalIndex`;
    const indexShards = this.framework.shardManager.getAllShards(globalIndexName);

    if (!indexShards || indexShards.length === 0) {
      console.warn(`No global index found for ${this.model.name}, falling back to all users query`);
      return await this.executeAllUsersQuery();
    }

    const indexResults: any[] = [];

    // Query all index shards in parallel
    const promises = indexShards.map((shard: any) =>
      this.queryDatabase(shard.database, 'keyvalue'),
    );
    const shardResults = await Promise.all(promises);

    for (const shardResult of shardResults) {
      indexResults.push(...shardResult);
    }

    // Now fetch actual documents from user databases
    return await this.fetchActualDocuments(indexResults);
  }

  private async executeAllUsersQuery(): Promise<T[]> {
    // This is a fallback for when global index is not available
    // It's expensive but ensures completeness
    console.warn(`⚠️  Executing expensive all-users query for ${this.model.name}`);

    // This would require getting all user IDs from the directory
    // For now, return empty array and log warning
    console.warn('All-users query not implemented - please ensure global indexes are set up');
    return [];
  }

  private async executeGlobalQuery(): Promise<T[]> {
    // For globally scoped models
    if (this.model.sharding) {
      return await this.executeShardedQuery();
    } else {
      const db = await this.framework.databaseManager.getGlobalDatabase(this.model.modelName);
      return await this.queryDatabase(db, this.model.storeType);
    }
  }

  private async executeShardedQuery(): Promise<T[]> {
    console.log(`🔀 Executing sharded query for ${this.model.name}`);

    const conditions = this.query.getConditions();
    const shardingConfig = this.model.sharding!;

    // Check if we can route to specific shard(s)
    const shardKeyCondition = conditions.find((c) => c.field === shardingConfig.key);

    if (shardKeyCondition && shardKeyCondition.operator === '=') {
      // Single shard query
      const shard = this.framework.shardManager.getShardForKey(
        this.model.modelName,
        shardKeyCondition.value,
      );
      return await this.queryDatabase(shard.database, this.model.storeType);
    } else if (shardKeyCondition && shardKeyCondition.operator === 'in') {
      // Multiple specific shards
      const results: T[] = [];
      const shardKeys = shardKeyCondition.value;

      const shardQueries = shardKeys.map(async (key: string) => {
        const shard = this.framework.shardManager.getShardForKey(this.model.modelName, key);
        return await this.queryDatabase(shard.database, this.model.storeType);
      });

      const shardResults = await Promise.all(shardQueries);
      for (const shardResult of shardResults) {
        results.push(...shardResult);
      }

      return this.postProcessResults(results);
    } else {
      // Query all shards
      const results: T[] = [];
      const allShards = this.framework.shardManager.getAllShards(this.model.modelName);

      const promises = allShards.map((shard: any) =>
        this.queryDatabase(shard.database, this.model.storeType),
      );
      const shardResults = await Promise.all(promises);

      for (const shardResult of shardResults) {
        results.push(...shardResult);
      }

      return this.postProcessResults(results);
    }
  }

  private async queryDatabase(database: any, dbType: StoreType): Promise<T[]> {
    // Get all documents from OrbitDB based on database type
    let documents: any[];

    try {
      documents = await this.framework.databaseManager.getAllDocuments(database, dbType);
    } catch (error) {
      console.error(`Error querying ${dbType} database:`, error);
      return [];
    }

    // Apply filters in memory
    documents = this.applyFilters(documents);

    // Apply sorting
    documents = this.applySorting(documents);

    // Apply limit/offset
    documents = this.applyLimitOffset(documents);

    // Convert to model instances
    const ModelClass = this.model as any; // Type assertion for abstract class
    return documents.map((doc) => new ModelClass(doc) as T);
  }

  private async fetchActualDocuments(indexResults: any[]): Promise<T[]> {
    console.log(`📄 Fetching ${indexResults.length} documents from user databases`);

    const results: T[] = [];

    // Group by userId for efficient database access
    const userGroups = new Map<string, any[]>();

    for (const indexEntry of indexResults) {
      const userId = indexEntry.userId;
      if (!userGroups.has(userId)) {
        userGroups.set(userId, []);
      }
      userGroups.get(userId)!.push(indexEntry);
    }

    // Fetch documents from each user's database
    const promises = Array.from(userGroups.entries()).map(async ([userId, entries]) => {
      try {
        const userDB = await this.framework.databaseManager.getUserDatabase(
          userId,
          this.model.modelName,
        );

        const userResults: T[] = [];

        // Fetch specific documents by ID
        for (const entry of entries) {
          try {
            const doc = await this.getDocumentById(userDB, this.model.storeType, entry.id);
            if (doc) {
              const ModelClass = this.model as any; // Type assertion for abstract class
              userResults.push(new ModelClass(doc) as T);
            }
          } catch (error) {
            console.warn(`Failed to fetch document ${entry.id} from user ${userId}:`, error);
          }
        }

        return userResults;
      } catch (error) {
        console.warn(`Failed to access user ${userId} database:`, error);
        return [];
      }
    });

    const userResults = await Promise.all(promises);

    // Flatten results
    for (const userResult of userResults) {
      results.push(...userResult);
    }

    return this.postProcessResults(results);
  }

  private async getDocumentById(database: any, dbType: StoreType, id: string): Promise<any | null> {
    try {
      switch (dbType) {
        case 'keyvalue':
          return await database.get(id);

        case 'docstore':
          return await database.get(id);

        case 'eventlog':
        case 'feed':
          // For append-only stores, we need to search through entries
          const iterator = database.iterator();
          const entries = iterator.collect();
          return (
            entries.find((entry: any) => entry.payload?.value?.id === id)?.payload?.value || null
          );

        default:
          return null;
      }
    } catch (error) {
      console.warn(`Error fetching document ${id} from ${dbType}:`, error);
      return null;
    }
  }

  private applyFilters(documents: any[]): any[] {
    const conditions = this.query.getConditions();

    return documents.filter((doc) => {
      return conditions.every((condition) => {
        return this.evaluateCondition(doc, condition);
      });
    });
  }

  private evaluateCondition(doc: any, condition: QueryCondition): boolean {
    const { field, operator, value } = condition;

    // Handle special operators
    if (operator === 'or') {
      return value.some((subCondition: QueryCondition) =>
        this.evaluateCondition(doc, subCondition),
      );
    }

    if (field === '__raw__') {
      // Raw conditions would need custom evaluation
      console.warn('Raw conditions not fully implemented');
      return true;
    }

    const docValue = this.getNestedValue(doc, field);

    switch (operator) {
      case '=':
      case '==':
      case 'eq':
        return docValue === value;

      case '!=':
      case '<>':
        return docValue !== value;

      case '>':
        return docValue > value;

      case '>=':
      case 'gte':
        return docValue >= value;

      case '<':
        return docValue < value;

      case '<=':
      case 'lte':
        return docValue <= value;

      case 'in':
        return Array.isArray(value) && value.includes(docValue);

      case 'not_in':
        return Array.isArray(value) && !value.includes(docValue);

      case 'contains':
        return Array.isArray(docValue) && docValue.includes(value);

      case 'like':
        return String(docValue).toLowerCase().includes(String(value).toLowerCase());

      case 'ilike':
        return String(docValue).toLowerCase().includes(String(value).toLowerCase());

      case 'is_null':
        return docValue === null || docValue === undefined;

      case 'is_not_null':
        return docValue !== null && docValue !== undefined;

      case 'between':
        return Array.isArray(value) && docValue >= value[0] && docValue <= value[1];

      case 'array_contains':
        return Array.isArray(docValue) && docValue.includes(value);

      case 'array_length_=':
        return Array.isArray(docValue) && docValue.length === value;

      case 'array_length_>':
        return Array.isArray(docValue) && docValue.length > value;

      case 'array_length_<':
        return Array.isArray(docValue) && docValue.length < value;

      case 'object_has_key':
        return typeof docValue === 'object' && docValue !== null && value in docValue;

      case 'date_=':
        return this.compareDates(docValue, '=', value);

      case 'date_>':
        return this.compareDates(docValue, '>', value);

      case 'date_<':
        return this.compareDates(docValue, '<', value);

      case 'date_between':
        return (
          this.compareDates(docValue, '>=', value[0]) && this.compareDates(docValue, '<=', value[1])
        );

      case 'year':
        return this.getDatePart(docValue, 'year') === value;

      case 'month':
        return this.getDatePart(docValue, 'month') === value;

      case 'day':
        return this.getDatePart(docValue, 'day') === value;

      default:
        console.warn(`Unsupported operator: ${operator}`);
        return true;
    }
  }

  private compareDates(docValue: any, operator: string, compareValue: any): boolean {
    const docDate = this.normalizeDate(docValue);
    const compDate = this.normalizeDate(compareValue);

    if (!docDate || !compDate) return false;

    switch (operator) {
      case '=':
        return docDate.getTime() === compDate.getTime();
      case '>':
        return docDate.getTime() > compDate.getTime();
      case '<':
        return docDate.getTime() < compDate.getTime();
      case '>=':
        return docDate.getTime() >= compDate.getTime();
      case '<=':
        return docDate.getTime() <= compDate.getTime();
      default:
        return false;
    }
  }

  private normalizeDate(value: any): Date | null {
    if (value instanceof Date) return value;
    if (typeof value === 'number') return new Date(value);
    if (typeof value === 'string') return new Date(value);
    return null;
  }

  private getDatePart(value: any, part: 'year' | 'month' | 'day'): number | null {
    const date = this.normalizeDate(value);
    if (!date) return null;

    switch (part) {
      case 'year':
        return date.getFullYear();
      case 'month':
        return date.getMonth() + 1; // 1-based month
      case 'day':
        return date.getDate();
      default:
        return null;
    }
  }

  private applySorting(documents: any[]): any[] {
    const sorting = this.query.getSorting();

    if (sorting.length === 0) {
      return documents;
    }

    return documents.sort((a, b) => {
      for (const sort of sorting) {
        const aValue = this.getNestedValue(a, sort.field);
        const bValue = this.getNestedValue(b, sort.field);

        let comparison = 0;

        if (aValue < bValue) comparison = -1;
        else if (aValue > bValue) comparison = 1;

        if (comparison !== 0) {
          return sort.direction === 'desc' ? -comparison : comparison;
        }
      }

      return 0;
    });
  }

  private applyLimitOffset(documents: any[]): any[] {
    const limit = this.query.getLimit();
    const offset = this.query.getOffset();

    let result = documents;

    if (offset && offset > 0) {
      result = result.slice(offset);
    }

    if (limit && limit > 0) {
      result = result.slice(0, limit);
    }

    return result;
  }

  private postProcessResults(results: T[]): T[] {
    // Apply global sorting across all results
    results = this.applySorting(results);

    // Apply global limit/offset
    results = this.applyLimitOffset(results);

    return results;
  }

  private getNestedValue(obj: any, path: string): any {
    if (!path) return obj;

    const keys = path.split('.');
    let current = obj;

    for (const key of keys) {
      if (current === null || current === undefined) {
        return undefined;
      }
      current = current[key];
    }

    return current;
  }

  // Public methods for query control
  disableCache(): this {
    this.useCache = false;
    return this;
  }

  enableCache(): this {
    this.useCache = true;
    return this;
  }

  getQueryPlan(): QueryPlan | undefined {
    return this.queryPlan;
  }

  explain(): any {
    const plan = this.queryPlan || QueryOptimizer.analyzeQuery(this.query);
    const suggestions = QueryOptimizer.suggestOptimizations(this.query);

    return {
      query: this.query.explain(),
      plan,
      suggestions,
      estimatedResultSize: QueryOptimizer.estimateResultSize(this.query),
    };
  }

  private getFrameworkInstance(): any {
    const framework = (globalThis as any).__debrosFramework;
    if (!framework) {
      // Try to get mock framework from BaseModel for testing
      const mockFramework = (this.model as any).getMockFramework?.();
      if (!mockFramework) {
        throw new Error('Framework not initialized. Call framework.initialize() first.');
      }
      return mockFramework;
    }
    return framework;
  }
}
