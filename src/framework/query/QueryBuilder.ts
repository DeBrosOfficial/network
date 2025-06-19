import { BaseModel } from '../models/BaseModel';
import { QueryCondition, SortConfig } from '../types/queries';
import { QueryExecutor } from './QueryExecutor';

export class QueryBuilder<T extends BaseModel> {
  private model: typeof BaseModel;
  private conditions: QueryCondition[] = [];
  private relations: string[] = [];
  private sorting: SortConfig[] = [];
  private limitation?: number;
  private offsetValue?: number;
  private groupByFields: string[] = [];
  private havingConditions: QueryCondition[] = [];
  private distinctFields: string[] = [];
  private cursorValue?: string;
  private _relationshipConstraints?: Map<string, ((query: QueryBuilder<any>) => QueryBuilder<any>) | undefined>;
  private cacheEnabled: boolean = false;
  private cacheTtl?: number;
  private cacheKey?: string;

  constructor(model: typeof BaseModel) {
    this.model = model;
  }

  // Basic filtering
  where(field: string, operator: string, value: any): this;
  where(field: string, value: any): this;
  where(field: string, operatorOrValue: string | any, value?: any): this {
    if (value !== undefined) {
      // Three parameter version: where('field', 'operator', 'value')
      this.conditions.push({ field, operator: operatorOrValue, value });
    } else {
      // Two parameter version: where('field', 'value') - defaults to equality
      this.conditions.push({ field, operator: 'eq', value: operatorOrValue });
    }
    return this;
  }

  whereIn(field: string, values: any[]): this {
    return this.where(field, 'in', values);
  }

  whereNotIn(field: string, values: any[]): this {
    return this.where(field, 'not_in', values);
  }

  whereNull(field: string): this {
    this.conditions.push({ field, operator: 'is null', value: null });
    return this;
  }

  whereNotNull(field: string): this {
    this.conditions.push({ field, operator: 'is not null', value: null });
    return this;
  }

  whereBetween(field: string, min: any, max: any): this {
    return this.where(field, 'between', [min, max]);
  }

  whereNot(field: string, operator: string, value: any): this {
    return this.where(field, `not_${operator}`, value);
  }

  whereLike(field: string, pattern: string): this {
    return this.where(field, 'like', pattern);
  }

  whereILike(field: string, pattern: string): this {
    return this.where(field, 'ilike', pattern);
  }

  // Date filtering
  whereDate(field: string, operator: string, date: Date | string | number): this {
    return this.where(field, `date_${operator}`, date);
  }

  whereDateBetween(
    field: string,
    startDate: Date | string | number,
    endDate: Date | string | number,
  ): this {
    return this.where(field, 'date_between', [startDate, endDate]);
  }

  whereYear(field: string, year: number): this {
    return this.where(field, 'year', year);
  }

  whereMonth(field: string, month: number): this {
    return this.where(field, 'month', month);
  }

  whereDay(field: string, day: number): this {
    return this.where(field, 'day', day);
  }

  // User-specific filtering (for user-scoped queries)
  whereUser(userId: string): this {
    return this.where('userId', '=', userId);
  }

  whereUserIn(userIds: string[]): this {
    this.conditions.push({
      field: 'userId',
      operator: 'userIn',
      value: userIds,
    });
    return this;
  }

  // Advanced filtering with OR conditions
  orWhere(field: string, operator: string, value: any): this;
  orWhere(field: string, value: any): this;
  orWhere(callback: (query: QueryBuilder<T>) => void): this;
  orWhere(fieldOrCallback: string | ((query: QueryBuilder<T>) => void), operatorOrValue?: string | any, value?: any): this {
    if (typeof fieldOrCallback === 'function') {
      // Callback version: orWhere((query) => { ... })
      const subQuery = new QueryBuilder<T>(this.model);
      fieldOrCallback(subQuery);

      this.conditions.push({
        field: '__or__',
        operator: 'or',
        value: subQuery.getWhereConditions(),
      });
    } else {
      // Simple orWhere version: orWhere('field', 'operator', 'value') or orWhere('field', 'value')
      let finalOperator = '=';
      let finalValue = operatorOrValue;
      
      if (value !== undefined) {
        finalOperator = operatorOrValue;
        finalValue = value;
      }

      const lastCondition = this.conditions[this.conditions.length - 1];
      if (lastCondition) {
        lastCondition.logical = 'or';
      }

      this.conditions.push({
        field: fieldOrCallback,
        operator: finalOperator,
        value: finalValue,
        logical: 'or'
      });
    }

    return this;
  }

  // Array and object field queries
  whereArrayContains(field: string, value: any): this {
    return this.where(field, 'array_contains', value);
  }

  whereArrayLength(field: string, operator: string, length: number): this {
    return this.where(field, `array_length_${operator}`, length);
  }

  whereObjectHasKey(field: string, key: string): this {
    return this.where(field, 'object_has_key', key);
  }

  whereObjectPath(field: string, path: string, operator: string, value: any): this {
    return this.where(field, `object_path_${operator}`, { path, value });
  }

  // Sorting
  orderBy(field: string, direction: 'asc' | 'desc' = 'asc'): this {
    this.sorting.push({ field, direction });
    return this;
  }

  orderByDesc(field: string): this {
    return this.orderBy(field, 'desc');
  }

  orderByRaw(expression: string): this {
    this.sorting.push({ field: expression, direction: 'asc' });
    return this;
  }

  // Multiple field sorting
  orderByMultiple(sorts: Array<{ field: string; direction: 'asc' | 'desc' }>): this {
    sorts.forEach((sort) => this.orderBy(sort.field, sort.direction));
    return this;
  }

  // Pagination
  limit(count: number): this {
    this.limitation = count;
    return this;
  }

  offset(count: number): this {
    this.offsetValue = count;
    return this;
  }

  skip(count: number): this {
    return this.offset(count);
  }

  take(count: number): this {
    return this.limit(count);
  }

  // Pagination helpers
  page(pageNumber: number, pageSize: number): this {
    this.limitation = pageSize;
    this.offsetValue = (pageNumber - 1) * pageSize;
    return this;
  }

  // Relationship loading
  load(relationships: string[]): this {
    this.relations = [...this.relations, ...relationships];
    return this;
  }

  with(relationships: string[]): this {
    return this.load(relationships);
  }

  loadNested(relationship: string, _callback: (query: QueryBuilder<any>) => void): this {
    // For nested relationship loading with constraints
    this.relations.push(relationship);
    // Store callback for nested query (implementation in QueryExecutor)
    return this;
  }

  // Aggregation
  groupBy(...fields: string[]): this {
    this.groupByFields.push(...fields);
    return this;
  }

  having(field: string, operator: string, value: any): this {
    this.havingConditions.push({ field, operator, value });
    return this;
  }

  // Distinct
  distinct(...fields: string[]): this {
    this.distinctFields.push(...fields);
    return this;
  }

  // Execution methods
  async exec(): Promise<T[]> {
    const executor = new QueryExecutor<T>(this.model, this);
    return await executor.execute();
  }

  async get(): Promise<T[]> {
    return await this.exec();
  }

  async first(): Promise<T | null> {
    const results = await this.limit(1).exec();
    return results[0] || null;
  }

  async firstOrFail(): Promise<T> {
    const result = await this.first();
    if (!result) {
      throw new Error(`No ${this.model.name} found matching the query`);
    }
    return result;
  }

  async find(id: string): Promise<T | null> {
    return await this.where('id', '=', id).first();
  }

  async findOrFail(id: string): Promise<T> {
    const result = await this.find(id);
    if (!result) {
      throw new Error(`${this.model.name} with id ${id} not found`);
    }
    return result;
  }

  async count(): Promise<number> {
    const executor = new QueryExecutor<T>(this.model, this);
    return await executor.count();
  }

  async exists(): Promise<boolean> {
    const count = await this.count();
    return count > 0;
  }

  async sum(field: string): Promise<number> {
    const executor = new QueryExecutor<T>(this.model, this);
    return await executor.sum(field);
  }

  async avg(field: string): Promise<number> {
    const executor = new QueryExecutor<T>(this.model, this);
    return await executor.avg(field);
  }

  async min(field: string): Promise<any> {
    const executor = new QueryExecutor<T>(this.model, this);
    return await executor.min(field);
  }

  async max(field: string): Promise<any> {
    const executor = new QueryExecutor<T>(this.model, this);
    return await executor.max(field);
  }

  // Pagination with metadata
  async paginate(
    page: number = 1,
    perPage: number = 15,
  ): Promise<{
    data: T[];
    total: number;
    perPage: number;
    currentPage: number;
    lastPage: number;
    hasNextPage: boolean;
    hasPrevPage: boolean;
  }> {
    const total = await this.count();
    const lastPage = Math.ceil(total / perPage);

    const data = await this.page(page, perPage).exec();

    return {
      data,
      total,
      perPage,
      currentPage: page,
      lastPage,
      hasNextPage: page < lastPage,
      hasPrevPage: page > 1,
    };
  }

  // Chunked processing
  async chunk(
    size: number,
    callback: (items: T[], page: number) => Promise<void | boolean>,
  ): Promise<void> {
    let page = 1;
    let hasMore = true;

    while (hasMore) {
      const items = await this.page(page, size).exec();

      if (items.length === 0) {
        break;
      }

      const result = await callback(items, page);

      // If callback returns false, stop processing
      if (result === false) {
        break;
      }

      hasMore = items.length === size;
      page++;
    }
  }

  // Query optimization hints
  useIndex(indexName: string): this {
    // Hint for query optimizer (implementation in QueryExecutor)
    (this as any)._indexHint = indexName;
    return this;
  }

  preferShard(shardIndex: number): this {
    // Force query to specific shard (for global sharded models)
    (this as any)._preferredShard = shardIndex;
    return this;
  }

  // Raw queries (for advanced users)
  whereRaw(expression: string, bindings: any[] = []): this {
    this.conditions.push({
      field: '__raw__',
      operator: 'raw',
      value: { expression, bindings },
    });
    return this;
  }

  // Getters for query configuration (used by QueryExecutor)
  getConditions(): QueryCondition[] {
    return [...this.conditions];
  }

  getRelations(): string[] {
    return [...this.relations];
  }

  getSorting(): SortConfig[] {
    return [...this.sorting];
  }

  getLimit(): number | undefined {
    return this.limitation;
  }

  getOffset(): number | undefined {
    return this.offsetValue;
  }

  getGroupBy(): string[] {
    return [...this.groupByFields];
  }

  getHaving(): QueryCondition[] {
    return [...this.havingConditions];
  }

  getDistinct(): string[] {
    return [...this.distinctFields];
  }

  getModel(): typeof BaseModel {
    return this.model;
  }

  // Getter methods for testing
  getWhereConditions(): QueryCondition[] {
    return [...this.conditions];
  }

  getOrderBy(): SortConfig[] {
    return [...this.sorting];
  }

  getLimit(): number | undefined {
    return this.limitation;
  }

  getOffset(): number | undefined {
    return this.offsetValue;
  }

  getRelationships(): any[] {
    return this.relations.map(relation => ({
      relation,
      constraints: this._relationshipConstraints?.get(relation)
    }));
  }

  getCacheOptions(): any {
    return {
      enabled: this.cacheEnabled,
      ttl: this.cacheTtl,
      key: this.cacheKey
    };
  }

  getCursor(): string | undefined {
    return this.cursorValue;
  }

  reset(): this {
    this.conditions = [];
    this.relations = [];
    this.sorting = [];
    this.limitation = undefined;
    this.offsetValue = undefined;
    this.groupByFields = [];
    this.havingConditions = [];
    this.distinctFields = [];
    return this;
  }

  // Caching methods
  cache(ttl: number, key?: string): this {
    this.cacheEnabled = true;
    this.cacheTtl = ttl;
    this.cacheKey = key;
    return this;
  }

  noCache(): this {
    this.cacheEnabled = false;
    this.cacheTtl = undefined;
    this.cacheKey = undefined;
    return this;
  }

  // Relationship loading
  with(relations: string[], constraints?: (query: QueryBuilder<any>) => QueryBuilder<any>): this {
    relations.forEach(relation => {
      // Store relationship with its constraints
      if (!this._relationshipConstraints) {
        this._relationshipConstraints = new Map();
      }
      this._relationshipConstraints.set(relation, constraints);
      this.relations.push(relation);
    });
    return this;
  }

  // Pagination
  after(cursor: string): this {
    this.cursorValue = cursor;
    return this;
  }

  // Query execution methods
  async exists(): Promise<boolean> {
    // Mock implementation
    return false;
  }

  async count(): Promise<number> {
    // Mock implementation
    return 0;
  }

  async sum(field: string): Promise<number> {
    // Mock implementation
    return 0;
  }

  async average(field: string): Promise<number> {
    // Mock implementation
    return 0;
  }

  async min(field: string): Promise<number> {
    // Mock implementation
    return 0;
  }

  async max(field: string): Promise<number> {
    // Mock implementation
    return 0;
  }

  async find(): Promise<T[]> {
    // Mock implementation
    return [];
  }

  async findOne(): Promise<T | null> {
    // Mock implementation
    return null;
  }

  async exec(): Promise<T[]> {
    // Mock implementation - same as find
    return [];
  }

  async first(): Promise<T | null> {
    // Mock implementation - same as findOne
    return null;
  }

  async paginate(page: number, perPage: number): Promise<any> {
    // Mock implementation
    return {
      data: [],
      total: 0,
      page,
      perPage,
      totalPages: 0,
      hasMore: false
    };
  }

  // Clone query for reuse
  clone(): QueryBuilder<T> {
    const cloned = new QueryBuilder<T>(this.model);
    cloned.conditions = [...this.conditions];
    cloned.relations = [...this.relations];
    cloned.sorting = [...this.sorting];
    cloned.limitation = this.limitation;
    cloned.offsetValue = this.offsetValue;
    cloned.groupByFields = [...this.groupByFields];
    cloned.havingConditions = [...this.havingConditions];
    cloned.distinctFields = [...this.distinctFields];

    return cloned;
  }

  // Debug methods
  toSQL(): string {
    // Generate SQL-like representation for debugging
    let sql = `SELECT * FROM ${this.model.name}`;

    if (this.conditions.length > 0) {
      const whereClause = this.conditions
        .map((c) => `${c.field} ${c.operator} ${JSON.stringify(c.value)}`)
        .join(' AND ');
      sql += ` WHERE ${whereClause}`;
    }

    if (this.sorting.length > 0) {
      const orderClause = this.sorting
        .map((s) => `${s.field} ${s.direction.toUpperCase()}`)
        .join(', ');
      sql += ` ORDER BY ${orderClause}`;
    }

    if (this.limitation) {
      sql += ` LIMIT ${this.limitation}`;
    }

    if (this.offsetValue) {
      sql += ` OFFSET ${this.offsetValue}`;
    }

    return sql;
  }

  explain(): any {
    return {
      model: this.model.name,
      scope: this.model.scope,
      conditions: this.conditions,
      relations: this.relations,
      sorting: this.sorting,
      limit: this.limitation,
      offset: this.offsetValue,
      sql: this.toSQL(),
    };
  }
}
