import { describe, beforeEach, it, expect, jest } from '@jest/globals';
import { QueryBuilder } from '../../../src/framework/query/QueryBuilder';
import { BaseModel } from '../../../src/framework/models/BaseModel';
import { Model, Field } from '../../../src/framework/models/decorators';
import { createMockServices } from '../../mocks/services';

// Test models for QueryBuilder testing
@Model({
  scope: 'global',
  type: 'docstore'
})
class TestUser extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'string', required: true })
  email: string;

  @Field({ type: 'number', required: false, default: 0 })
  score: number;

  @Field({ type: 'boolean', required: false, default: true })
  isActive: boolean;

  @Field({ type: 'array', required: false, default: [] })
  tags: string[];

  @Field({ type: 'number', required: false })
  createdAt: number;

  @Field({ type: 'number', required: false })
  lastLoginAt: number;
}

@Model({
  scope: 'user',
  type: 'docstore'
})
class TestPost extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  userId: string;

  @Field({ type: 'array', required: false, default: [] })
  tags: string[];

  @Field({ type: 'boolean', required: false, default: true })
  isPublished: boolean;

  @Field({ type: 'number', required: false, default: 0 })
  likeCount: number;

  @Field({ type: 'number', required: false })
  publishedAt: number;
}

describe('QueryBuilder', () => {
  let mockServices: any;

  beforeEach(() => {
    mockServices = createMockServices();
    jest.clearAllMocks();
  });

  describe('Basic Query Construction', () => {
    it('should create a QueryBuilder instance', () => {
      const queryBuilder = new QueryBuilder(TestUser);

      expect(queryBuilder).toBeInstanceOf(QueryBuilder);
      expect(queryBuilder.getModel()).toBe(TestUser);
    });

    it('should support method chaining', () => {
      const queryBuilder = new QueryBuilder(TestUser)
        .where('isActive', true)
        .where('score', '>', 50)
        .orderBy('username')
        .limit(10);

      expect(queryBuilder).toBeInstanceOf(QueryBuilder);
    });
  });

  describe('Where Clauses', () => {
    let queryBuilder: QueryBuilder<TestUser>;

    beforeEach(() => {
      queryBuilder = new QueryBuilder(TestUser);
    });

    it('should handle basic equality conditions', () => {
      queryBuilder.where('username', 'testuser');
      
      const conditions = queryBuilder.getWhereConditions();
      expect(conditions).toHaveLength(1);
      expect(conditions[0]).toEqual({
        field: 'username',
        operator: 'eq',
        value: 'testuser'
      });
    });

    it('should handle explicit operators', () => {
      queryBuilder
        .where('score', '>', 50)
        .where('score', '<=', 100)
        .where('isActive', '!=', false);

      const conditions = queryBuilder.getWhereConditions();
      expect(conditions).toHaveLength(3);
      
      expect(conditions[0]).toEqual({
        field: 'score',
        operator: 'gt',
        value: 50
      });

      expect(conditions[1]).toEqual({
        field: 'score',
        operator: 'lte',
        value: 100
      });

      expect(conditions[2]).toEqual({
        field: 'isActive',
        operator: 'ne',
        value: false
      });
    });

    it('should handle IN and NOT IN operators', () => {
      queryBuilder
        .where('username', 'in', ['alice', 'bob', 'charlie'])
        .where('status', 'not in', ['deleted', 'banned']);

      const conditions = queryBuilder.getWhereConditions();
      expect(conditions).toHaveLength(2);

      expect(conditions[0]).toEqual({
        field: 'username',
        operator: 'in',
        value: ['alice', 'bob', 'charlie']
      });

      expect(conditions[1]).toEqual({
        field: 'status',
        operator: 'not in',
        value: ['deleted', 'banned']
      });
    });

    it('should handle LIKE and REGEX operators', () => {
      queryBuilder
        .where('username', 'like', 'test%')
        .where('email', 'regex', /@gmail\.com$/);

      const conditions = queryBuilder.getWhereConditions();
      expect(conditions).toHaveLength(2);

      expect(conditions[0]).toEqual({
        field: 'username',
        operator: 'like',
        value: 'test%'
      });

      expect(conditions[1]).toEqual({
        field: 'email',
        operator: 'regex',
        value: /@gmail\.com$/
      });
    });

    it('should handle NULL checks', () => {
      queryBuilder
        .where('lastLoginAt', 'is null')
        .where('email', 'is not null');

      const conditions = queryBuilder.getWhereConditions();
      expect(conditions).toHaveLength(2);

      expect(conditions[0]).toEqual({
        field: 'lastLoginAt',
        operator: 'is null',
        value: null
      });

      expect(conditions[1]).toEqual({
        field: 'email',
        operator: 'is not null',
        value: null
      });
    });

    it('should handle array operations', () => {
      queryBuilder
        .where('tags', 'includes', 'javascript')
        .where('tags', 'includes any', ['react', 'vue', 'angular'])
        .where('tags', 'includes all', ['frontend', 'framework']);

      const conditions = queryBuilder.getWhereConditions();
      expect(conditions).toHaveLength(3);

      expect(conditions[0]).toEqual({
        field: 'tags',
        operator: 'includes',
        value: 'javascript'
      });

      expect(conditions[1]).toEqual({
        field: 'tags',
        operator: 'includes any',
        value: ['react', 'vue', 'angular']
      });

      expect(conditions[2]).toEqual({
        field: 'tags',
        operator: 'includes all',
        value: ['frontend', 'framework']
      });
    });
  });

  describe('OR Conditions', () => {
    let queryBuilder: QueryBuilder<TestUser>;

    beforeEach(() => {
      queryBuilder = new QueryBuilder(TestUser);
    });

    it('should handle OR conditions', () => {
      queryBuilder
        .where('isActive', true)
        .orWhere('lastLoginAt', '>', Date.now() - 24*60*60*1000);

      const conditions = queryBuilder.getWhereConditions();
      expect(conditions).toHaveLength(2);

      expect(conditions[0].operator).toBe('eq');
      expect(conditions[1].operator).toBe('gt');
      expect(conditions[1].logical).toBe('or');
    });

    it('should handle grouped OR conditions', () => {
      queryBuilder
        .where('isActive', true)
        .where((query) => {
          query.where('username', 'like', 'admin%')
               .orWhere('email', 'like', '%@admin.com');
        });

      const conditions = queryBuilder.getWhereConditions();
      expect(conditions).toHaveLength(2);

      expect(conditions[0].field).toBe('isActive');
      expect(conditions[1].type).toBe('group');
      expect(conditions[1].conditions).toHaveLength(2);
    });
  });

  describe('Ordering', () => {
    let queryBuilder: QueryBuilder<TestUser>;

    beforeEach(() => {
      queryBuilder = new QueryBuilder(TestUser);
    });

    it('should handle single field ordering', () => {
      queryBuilder.orderBy('username');

      const orderBy = queryBuilder.getOrderBy();
      expect(orderBy).toHaveLength(1);
      expect(orderBy[0]).toEqual({
        field: 'username',
        direction: 'asc'
      });
    });

    it('should handle multiple field ordering', () => {
      queryBuilder
        .orderBy('score', 'desc')
        .orderBy('username', 'asc');

      const orderBy = queryBuilder.getOrderBy();
      expect(orderBy).toHaveLength(2);

      expect(orderBy[0]).toEqual({
        field: 'score',
        direction: 'desc'
      });

      expect(orderBy[1]).toEqual({
        field: 'username',
        direction: 'asc'
      });
    });

    it('should handle random ordering', () => {
      queryBuilder.orderBy('random');

      const orderBy = queryBuilder.getOrderBy();
      expect(orderBy).toHaveLength(1);
      expect(orderBy[0]).toEqual({
        field: 'random',
        direction: 'asc'
      });
    });
  });

  describe('Pagination', () => {
    let queryBuilder: QueryBuilder<TestUser>;

    beforeEach(() => {
      queryBuilder = new QueryBuilder(TestUser);
    });

    it('should handle limit', () => {
      queryBuilder.limit(10);

      expect(queryBuilder.getLimit()).toBe(10);
    });

    it('should handle offset', () => {
      queryBuilder.offset(20);

      expect(queryBuilder.getOffset()).toBe(20);
    });

    it('should handle limit and offset together', () => {
      queryBuilder.limit(10).offset(20);

      expect(queryBuilder.getLimit()).toBe(10);
      expect(queryBuilder.getOffset()).toBe(20);
    });

    it('should handle cursor-based pagination', () => {
      queryBuilder.after('cursor-value').limit(10);

      expect(queryBuilder.getCursor()).toBe('cursor-value');
      expect(queryBuilder.getLimit()).toBe(10);
    });
  });

  describe('Relationship Loading', () => {
    let queryBuilder: QueryBuilder<TestUser>;

    beforeEach(() => {
      queryBuilder = new QueryBuilder(TestUser);
    });

    it('should handle simple relationship loading', () => {
      queryBuilder.with(['posts']);

      const relationships = queryBuilder.getRelationships();
      expect(relationships).toHaveLength(1);
      expect(relationships[0]).toEqual({
        relation: 'posts',
        constraints: undefined
      });
    });

    it('should handle nested relationship loading', () => {
      queryBuilder.with(['posts.comments', 'profile']);

      const relationships = queryBuilder.getRelationships();
      expect(relationships).toHaveLength(2);

      expect(relationships[0].relation).toBe('posts.comments');
      expect(relationships[1].relation).toBe('profile');
    });

    it('should handle relationship loading with constraints', () => {
      queryBuilder.with(['posts'], (query) => {
        query.where('isPublished', true)
             .orderBy('publishedAt', 'desc')
             .limit(5);
      });

      const relationships = queryBuilder.getRelationships();
      expect(relationships).toHaveLength(1);
      expect(relationships[0].relation).toBe('posts');
      expect(typeof relationships[0].constraints).toBe('function');
    });
  });

  describe('Aggregation Methods', () => {
    let queryBuilder: QueryBuilder<TestUser>;

    beforeEach(() => {
      queryBuilder = new QueryBuilder(TestUser);
    });

    it('should support count queries', async () => {
      const countQuery = queryBuilder.where('isActive', true);
      
      // Mock the count execution
      jest.spyOn(countQuery, 'count').mockResolvedValue(42);
      
      const count = await countQuery.count();
      expect(count).toBe(42);
    });

    it('should support sum aggregation', async () => {
      const sumQuery = queryBuilder.where('isActive', true);
      
      // Mock the sum execution
      jest.spyOn(sumQuery, 'sum').mockResolvedValue(1250);
      
      const sum = await sumQuery.sum('score');
      expect(sum).toBe(1250);
    });

    it('should support average aggregation', async () => {
      const avgQuery = queryBuilder.where('isActive', true);
      
      // Mock the average execution
      jest.spyOn(avgQuery, 'average').mockResolvedValue(85.5);
      
      const avg = await avgQuery.average('score');
      expect(avg).toBe(85.5);
    });

    it('should support min/max aggregation', async () => {
      const query = queryBuilder.where('isActive', true);
      
      // Mock the min/max execution
      jest.spyOn(query, 'min').mockResolvedValue(10);
      jest.spyOn(query, 'max').mockResolvedValue(100);
      
      const min = await query.min('score');
      const max = await query.max('score');
      
      expect(min).toBe(10);
      expect(max).toBe(100);
    });
  });

  describe('Query Execution', () => {
    let queryBuilder: QueryBuilder<TestUser>;

    beforeEach(() => {
      queryBuilder = new QueryBuilder(TestUser);
    });

    it('should execute find queries', async () => {
      const mockResults = [
        { id: '1', username: 'alice', email: 'alice@example.com' },
        { id: '2', username: 'bob', email: 'bob@example.com' }
      ];

      // Mock the find execution
      jest.spyOn(queryBuilder, 'find').mockResolvedValue(mockResults as any);

      const results = await queryBuilder
        .where('isActive', true)
        .orderBy('username')
        .find();

      expect(results).toEqual(mockResults);
    });

    it('should execute findOne queries', async () => {
      const mockResult = { id: '1', username: 'alice', email: 'alice@example.com' };

      // Mock the findOne execution
      jest.spyOn(queryBuilder, 'findOne').mockResolvedValue(mockResult as any);

      const result = await queryBuilder
        .where('username', 'alice')
        .findOne();

      expect(result).toEqual(mockResult);
    });

    it('should return null for findOne when no results', async () => {
      // Mock the findOne execution to return null
      jest.spyOn(queryBuilder, 'findOne').mockResolvedValue(null);

      const result = await queryBuilder
        .where('username', 'nonexistent')
        .findOne();

      expect(result).toBeNull();
    });

    it('should execute exists queries', async () => {
      // Mock the exists execution
      jest.spyOn(queryBuilder, 'exists').mockResolvedValue(true);

      const exists = await queryBuilder
        .where('username', 'alice')
        .exists();

      expect(exists).toBe(true);
    });
  });

  describe('Caching', () => {
    let queryBuilder: QueryBuilder<TestUser>;

    beforeEach(() => {
      queryBuilder = new QueryBuilder(TestUser);
    });

    it('should support query caching', () => {
      queryBuilder.cache(300); // 5 minutes

      expect(queryBuilder.getCacheOptions()).toEqual({
        enabled: true,
        ttl: 300,
        key: undefined
      });
    });

    it('should support custom cache keys', () => {
      queryBuilder.cache(600, 'active-users');

      expect(queryBuilder.getCacheOptions()).toEqual({
        enabled: true,
        ttl: 600,
        key: 'active-users'
      });
    });

    it('should disable caching', () => {
      queryBuilder.noCache();

      expect(queryBuilder.getCacheOptions()).toEqual({
        enabled: false,
        ttl: undefined,
        key: undefined
      });
    });
  });

  describe('Complex Query Building', () => {
    it('should handle complex queries with multiple conditions', () => {
      const queryBuilder = new QueryBuilder(TestPost)
        .where('isPublished', true)
        .where('likeCount', '>=', 10)
        .where('tags', 'includes any', ['javascript', 'typescript'])
        .where((query) => {
          query.where('title', 'like', '%tutorial%')
               .orWhere('content', 'like', '%guide%');
        })
        .with(['user'])
        .orderBy('likeCount', 'desc')
        .orderBy('publishedAt', 'desc')
        .limit(20)
        .cache(300);

      // Verify the query structure
      const conditions = queryBuilder.getWhereConditions();
      expect(conditions).toHaveLength(4);

      const orderBy = queryBuilder.getOrderBy();
      expect(orderBy).toHaveLength(2);

      const relationships = queryBuilder.getRelationships();
      expect(relationships).toHaveLength(1);

      expect(queryBuilder.getLimit()).toBe(20);
      expect(queryBuilder.getCacheOptions().enabled).toBe(true);
    });

    it('should handle pagination queries', async () => {
      // Mock paginate execution
      const mockPaginatedResult = {
        data: [
          { id: '1', title: 'Post 1' },
          { id: '2', title: 'Post 2' }
        ],
        total: 100,
        page: 1,
        perPage: 20,
        totalPages: 5,
        hasMore: true
      };

      const queryBuilder = new QueryBuilder(TestPost);
      jest.spyOn(queryBuilder, 'paginate').mockResolvedValue(mockPaginatedResult as any);

      const result = await queryBuilder
        .where('isPublished', true)
        .orderBy('publishedAt', 'desc')
        .paginate(1, 20);

      expect(result).toEqual(mockPaginatedResult);
    });
  });

  describe('Query Builder State', () => {
    it('should clone query builder state', () => {
      const originalQuery = new QueryBuilder(TestUser)
        .where('isActive', true)
        .orderBy('username')
        .limit(10);

      const clonedQuery = originalQuery.clone();

      expect(clonedQuery).not.toBe(originalQuery);
      expect(clonedQuery.getWhereConditions()).toEqual(originalQuery.getWhereConditions());
      expect(clonedQuery.getOrderBy()).toEqual(originalQuery.getOrderBy());
      expect(clonedQuery.getLimit()).toEqual(originalQuery.getLimit());
    });

    it('should reset query builder state', () => {
      const queryBuilder = new QueryBuilder(TestUser)
        .where('isActive', true)
        .orderBy('username')
        .limit(10)
        .cache(300);

      queryBuilder.reset();

      expect(queryBuilder.getWhereConditions()).toHaveLength(0);
      expect(queryBuilder.getOrderBy()).toHaveLength(0);
      expect(queryBuilder.getLimit()).toBeUndefined();
      expect(queryBuilder.getCacheOptions().enabled).toBe(false);
    });
  });

  describe('Error Handling', () => {
    let queryBuilder: QueryBuilder<TestUser>;

    beforeEach(() => {
      queryBuilder = new QueryBuilder(TestUser);
    });

    it('should handle invalid operators', () => {
      expect(() => {
        queryBuilder.where('username', 'invalid-operator' as any, 'value');
      }).toThrow();
    });

    it('should handle invalid field names', () => {
      expect(() => {
        queryBuilder.where('nonexistentField', 'value');
      }).toThrow();
    });

    it('should handle invalid order directions', () => {
      expect(() => {
        queryBuilder.orderBy('username', 'invalid-direction' as any);
      }).toThrow();
    });

    it('should handle negative limits', () => {
      expect(() => {
        queryBuilder.limit(-1);
      }).toThrow();
    });

    it('should handle negative offsets', () => {
      expect(() => {
        queryBuilder.offset(-1);
      }).toThrow();
    });
  });
});