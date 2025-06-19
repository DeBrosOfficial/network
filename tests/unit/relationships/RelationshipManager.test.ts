import { describe, beforeEach, it, expect, jest } from '@jest/globals';
import { RelationshipManager, RelationshipLoadOptions } from '../../../src/framework/relationships/RelationshipManager';
import { BaseModel } from '../../../src/framework/models/BaseModel';
import { Model, Field, BelongsTo, HasMany, HasOne, ManyToMany } from '../../../src/framework/models/decorators';
import { QueryBuilder } from '../../../src/framework/query/QueryBuilder';
import { createMockServices } from '../../mocks/services';

// Test models for relationship testing
@Model({
  scope: 'global',
  type: 'docstore'
})
class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'string', required: true })
  email: string;

  @HasMany(() => Post, 'userId')
  posts: Post[];

  @HasOne(() => Profile, 'userId')
  profile: Profile;

  @ManyToMany(() => Role, 'user_roles', 'userId', 'roleId')
  roles: Role[];

  // Mock query methods
  static where = jest.fn().mockReturnThis();
  static whereIn = jest.fn().mockReturnThis();
  static first = jest.fn();
  static exec = jest.fn();
}

@Model({
  scope: 'user',
  type: 'docstore'
})
class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title: string;

  @Field({ type: 'string', required: true })
  content: string;

  @Field({ type: 'string', required: true })
  userId: string;

  @BelongsTo(() => User, 'userId')
  user: User;

  // Mock query methods
  static where = jest.fn().mockReturnThis();
  static whereIn = jest.fn().mockReturnThis();
  static first = jest.fn();
  static exec = jest.fn();
}

@Model({
  scope: 'global',
  type: 'docstore'
})
class Profile extends BaseModel {
  @Field({ type: 'string', required: true })
  bio: string;

  @Field({ type: 'string', required: true })
  userId: string;

  @BelongsTo(() => User, 'userId')
  user: User;

  // Mock query methods
  static where = jest.fn().mockReturnThis();
  static whereIn = jest.fn().mockReturnThis();
  static first = jest.fn();
  static exec = jest.fn();
}

@Model({
  scope: 'global',
  type: 'docstore'
})
class Role extends BaseModel {
  @Field({ type: 'string', required: true })
  name: string;

  @ManyToMany(() => User, 'user_roles', 'roleId', 'userId')
  users: User[];

  // Mock query methods
  static where = jest.fn().mockReturnThis();
  static whereIn = jest.fn().mockReturnThis();
  static first = jest.fn();
  static exec = jest.fn();
}

@Model({
  scope: 'global',
  type: 'docstore'
})
class UserRole extends BaseModel {
  @Field({ type: 'string', required: true })
  userId: string;

  @Field({ type: 'string', required: true })
  roleId: string;

  // Mock query methods
  static where = jest.fn().mockReturnThis();
  static whereIn = jest.fn().mockReturnThis();
  static first = jest.fn();
  static exec = jest.fn();
}

describe('RelationshipManager', () => {
  let relationshipManager: RelationshipManager;
  let mockFramework: any;
  let user: User;
  let post: Post;
  let profile: Profile;
  let role: Role;

  beforeEach(() => {
    const mockServices = createMockServices();
    mockFramework = {
      services: mockServices
    };

    relationshipManager = new RelationshipManager(mockFramework);

    // Create test instances
    user = new User();
    user.id = 'user-123';
    user.username = 'testuser';
    user.email = 'test@example.com';

    post = new Post();
    post.id = 'post-123';
    post.title = 'Test Post';
    post.content = 'Test content';
    post.userId = 'user-123';

    profile = new Profile();
    profile.id = 'profile-123';
    profile.bio = 'Test bio';
    profile.userId = 'user-123';

    role = new Role();
    role.id = 'role-123';
    role.name = 'admin';

    // Clear all mocks
    jest.clearAllMocks();
  });

  describe('BelongsTo Relationships', () => {
    it('should load belongsTo relationship correctly', async () => {
      const mockUser = new User();
      mockUser.id = 'user-123';
      
      User.first.mockResolvedValue(mockUser);

      const result = await relationshipManager.loadRelationship(post, 'user');

      expect(User.where).toHaveBeenCalledWith('id', '=', 'user-123');
      expect(User.first).toHaveBeenCalled();
      expect(result).toBe(mockUser);
      expect(post._loadedRelations.get('user')).toBe(mockUser);
    });

    it('should return null for belongsTo when foreign key is null', async () => {
      post.userId = null as any;

      const result = await relationshipManager.loadRelationship(post, 'user');

      expect(result).toBeNull();
      expect(User.where).not.toHaveBeenCalled();
    });

    it('should apply constraints to belongsTo queries', async () => {
      const mockUser = new User();
      User.first.mockResolvedValue(mockUser);

      const mockQueryBuilder = {
        where: jest.fn().mockReturnThis(),
        first: jest.fn().mockResolvedValue(mockUser)
      };
      User.where.mockReturnValue(mockQueryBuilder);

      const options: RelationshipLoadOptions = {
        constraints: (query) => query.where('isActive', true)
      };

      await relationshipManager.loadRelationship(post, 'user', options);

      expect(User.where).toHaveBeenCalledWith('id', '=', 'user-123');
      expect(options.constraints).toBeDefined();
    });
  });

  describe('HasMany Relationships', () => {
    it('should load hasMany relationship correctly', async () => {
      const mockPosts = [
        { id: 'post-1', title: 'Post 1', userId: 'user-123' },
        { id: 'post-2', title: 'Post 2', userId: 'user-123' }
      ];

      Post.exec.mockResolvedValue(mockPosts);

      const result = await relationshipManager.loadRelationship(user, 'posts');

      expect(Post.where).toHaveBeenCalledWith('userId', '=', 'user-123');
      expect(Post.exec).toHaveBeenCalled();
      expect(result).toEqual(mockPosts);
      expect(user._loadedRelations.get('posts')).toEqual(mockPosts);
    });

    it('should return empty array for hasMany when local key is null', async () => {
      user.id = null as any;

      const result = await relationshipManager.loadRelationship(user, 'posts');

      expect(result).toEqual([]);
      expect(Post.where).not.toHaveBeenCalled();
    });

    it('should apply ordering and limits to hasMany queries', async () => {
      const mockPosts = [{ id: 'post-1', title: 'Post 1' }];
      
      const mockQueryBuilder = {
        where: jest.fn().mockReturnThis(),
        orderBy: jest.fn().mockReturnThis(),
        limit: jest.fn().mockReturnThis(),
        exec: jest.fn().mockResolvedValue(mockPosts)
      };
      Post.where.mockReturnValue(mockQueryBuilder);

      const options: RelationshipLoadOptions = {
        orderBy: { field: 'createdAt', direction: 'desc' },
        limit: 5
      };

      await relationshipManager.loadRelationship(user, 'posts', options);

      expect(mockQueryBuilder.orderBy).toHaveBeenCalledWith('createdAt', 'desc');
      expect(mockQueryBuilder.limit).toHaveBeenCalledWith(5);
    });
  });

  describe('HasOne Relationships', () => {
    it('should load hasOne relationship correctly', async () => {
      const mockProfile = { id: 'profile-1', bio: 'Test bio', userId: 'user-123' };

      const mockQueryBuilder = {
        where: jest.fn().mockReturnThis(),
        limit: jest.fn().mockReturnThis(),
        exec: jest.fn().mockResolvedValue([mockProfile])
      };
      Profile.where.mockReturnValue(mockQueryBuilder);

      const result = await relationshipManager.loadRelationship(user, 'profile');

      expect(Profile.where).toHaveBeenCalledWith('userId', '=', 'user-123');
      expect(mockQueryBuilder.limit).toHaveBeenCalledWith(1);
      expect(result).toBe(mockProfile);
    });

    it('should return null for hasOne when no results found', async () => {
      const mockQueryBuilder = {
        where: jest.fn().mockReturnThis(),
        limit: jest.fn().mockReturnThis(),
        exec: jest.fn().mockResolvedValue([])
      };
      Profile.where.mockReturnValue(mockQueryBuilder);

      const result = await relationshipManager.loadRelationship(user, 'profile');

      expect(result).toBeNull();
    });
  });

  describe('ManyToMany Relationships', () => {
    it('should load manyToMany relationship correctly', async () => {
      const mockJunctionRecords = [
        { userId: 'user-123', roleId: 'role-1' },
        { userId: 'user-123', roleId: 'role-2' }
      ];
      const mockRoles = [
        { id: 'role-1', name: 'admin' },
        { id: 'role-2', name: 'editor' }
      ];

      // Mock UserRole (junction table)
      const mockJunctionQuery = {
        where: jest.fn().mockReturnThis(),
        exec: jest.fn().mockResolvedValue(mockJunctionRecords)
      };

      // Mock Role query
      const mockRoleQuery = {
        whereIn: jest.fn().mockReturnThis(),
        exec: jest.fn().mockResolvedValue(mockRoles)
      };

      UserRole.where.mockReturnValue(mockJunctionQuery);
      Role.whereIn.mockReturnValue(mockRoleQuery);

      // Mock the relationship config to include the through model
      const originalRelationships = User.relationships;
      User.relationships = new Map();
      User.relationships.set('roles', {
        type: 'manyToMany',
        model: Role,
        through: UserRole,
        foreignKey: 'roleId',
        localKey: 'id',
        propertyKey: 'roles'
      });

      const result = await relationshipManager.loadRelationship(user, 'roles');

      expect(UserRole.where).toHaveBeenCalledWith('id', '=', 'user-123');
      expect(Role.whereIn).toHaveBeenCalledWith('id', ['role-1', 'role-2']);
      expect(result).toEqual(mockRoles);

      // Restore original relationships
      User.relationships = originalRelationships;
    });

    it('should handle empty junction table for manyToMany', async () => {
      const mockJunctionQuery = {
        where: jest.fn().mockReturnThis(),
        exec: jest.fn().mockResolvedValue([])
      };

      UserRole.where.mockReturnValue(mockJunctionQuery);

      // Mock the relationship config
      const originalRelationships = User.relationships;
      User.relationships = new Map();
      User.relationships.set('roles', {
        type: 'manyToMany',
        model: Role,
        through: UserRole,
        foreignKey: 'roleId',
        localKey: 'id',
        propertyKey: 'roles'
      });

      const result = await relationshipManager.loadRelationship(user, 'roles');

      expect(result).toEqual([]);

      // Restore original relationships
      User.relationships = originalRelationships;
    });

    it('should throw error for manyToMany without through model', async () => {
      // Mock the relationship config without through model
      const originalRelationships = User.relationships;
      User.relationships = new Map();
      User.relationships.set('roles', {
        type: 'manyToMany',
        model: Role,
        through: null as any,
        foreignKey: 'roleId',
        localKey: 'id',
        propertyKey: 'roles'
      });

      await expect(relationshipManager.loadRelationship(user, 'roles')).rejects.toThrow(
        'Many-to-many relationships require a through model'
      );

      // Restore original relationships
      User.relationships = originalRelationships;
    });
  });

  describe('Eager Loading', () => {
    it('should eager load multiple relationships for multiple instances', async () => {
      const users = [user, new User()];
      users[1].id = 'user-456';

      const mockPosts = [
        { id: 'post-1', userId: 'user-123' },
        { id: 'post-2', userId: 'user-456' }
      ];
      const mockProfiles = [
        { id: 'profile-1', userId: 'user-123' },
        { id: 'profile-2', userId: 'user-456' }
      ];

      // Mock hasMany query for posts
      const mockPostQuery = {
        whereIn: jest.fn().mockReturnThis(),
        exec: jest.fn().mockResolvedValue(mockPosts)
      };
      Post.whereIn.mockReturnValue(mockPostQuery);

      // Mock hasOne query for profiles
      const mockProfileQuery = {
        whereIn: jest.fn().mockReturnThis(),
        limit: jest.fn().mockReturnThis(),
        exec: jest.fn().mockResolvedValue(mockProfiles)
      };
      Profile.whereIn.mockReturnValue(mockProfileQuery);

      await relationshipManager.eagerLoadRelationships(users, ['posts', 'profile']);

      expect(Post.whereIn).toHaveBeenCalledWith('userId', ['user-123', 'user-456']);
      expect(Profile.whereIn).toHaveBeenCalledWith('userId', ['user-123', 'user-456']);

      // Check that relationships were loaded on instances
      expect(users[0]._loadedRelations.has('posts')).toBe(true);
      expect(users[0]._loadedRelations.has('profile')).toBe(true);
      expect(users[1]._loadedRelations.has('posts')).toBe(true);
      expect(users[1]._loadedRelations.has('profile')).toBe(true);
    });

    it('should handle empty instances array', async () => {
      await relationshipManager.eagerLoadRelationships([], ['posts']);

      expect(Post.whereIn).not.toHaveBeenCalled();
    });

    it('should skip non-existent relationships during eager loading', async () => {
      const consoleSpy = jest.spyOn(console, 'warn').mockImplementation();

      await relationshipManager.eagerLoadRelationships([user], ['nonExistentRelation']);

      expect(consoleSpy).toHaveBeenCalledWith(
        "Relationship 'nonExistentRelation' not found on User"
      );

      consoleSpy.mockRestore();
    });
  });

  describe('Caching', () => {
    it('should use cache when available', async () => {
      const mockUser = new User();
      
      // Mock cache hit
      jest.spyOn(relationshipManager['cache'], 'get').mockReturnValue(mockUser);
      jest.spyOn(relationshipManager['cache'], 'generateKey').mockReturnValue('cache-key');

      const result = await relationshipManager.loadRelationship(post, 'user');

      expect(result).toBe(mockUser);
      expect(User.where).not.toHaveBeenCalled(); // Should not query database
    });

    it('should store in cache after loading', async () => {
      const mockUser = new User();
      User.first.mockResolvedValue(mockUser);

      const setCacheSpy = jest.spyOn(relationshipManager['cache'], 'set');
      const generateKeySpy = jest.spyOn(relationshipManager['cache'], 'generateKey').mockReturnValue('cache-key');

      await relationshipManager.loadRelationship(post, 'user');

      expect(setCacheSpy).toHaveBeenCalledWith('cache-key', mockUser, 'User', 'belongsTo');
      expect(generateKeySpy).toHaveBeenCalled();
    });

    it('should skip cache when useCache is false', async () => {
      const mockUser = new User();
      User.first.mockResolvedValue(mockUser);

      const getCacheSpy = jest.spyOn(relationshipManager['cache'], 'get');
      const setCacheSpy = jest.spyOn(relationshipManager['cache'], 'set');

      await relationshipManager.loadRelationship(post, 'user', { useCache: false });

      expect(getCacheSpy).not.toHaveBeenCalled();
      expect(setCacheSpy).not.toHaveBeenCalled();
    });
  });

  describe('Cache Management', () => {
    it('should invalidate relationship cache for specific relationship', () => {
      const invalidateSpy = jest.spyOn(relationshipManager['cache'], 'invalidate').mockReturnValue(true);
      const generateKeySpy = jest.spyOn(relationshipManager['cache'], 'generateKey').mockReturnValue('cache-key');

      const result = relationshipManager.invalidateRelationshipCache(user, 'posts');

      expect(generateKeySpy).toHaveBeenCalledWith(user, 'posts');
      expect(invalidateSpy).toHaveBeenCalledWith('cache-key');
      expect(result).toBe(1);
    });

    it('should invalidate all cache for instance when no relationship specified', () => {
      const invalidateByInstanceSpy = jest.spyOn(relationshipManager['cache'], 'invalidateByInstance').mockReturnValue(3);

      const result = relationshipManager.invalidateRelationshipCache(user);

      expect(invalidateByInstanceSpy).toHaveBeenCalledWith(user);
      expect(result).toBe(3);
    });

    it('should invalidate cache by model name', () => {
      const invalidateByModelSpy = jest.spyOn(relationshipManager['cache'], 'invalidateByModel').mockReturnValue(5);

      const result = relationshipManager.invalidateModelCache('User');

      expect(invalidateByModelSpy).toHaveBeenCalledWith('User');
      expect(result).toBe(5);
    });

    it('should get cache statistics', () => {
      const mockStats = { cache: { hitRate: 0.85 }, performance: { avgLoadTime: 50 } };
      jest.spyOn(relationshipManager['cache'], 'getStats').mockReturnValue(mockStats.cache);
      jest.spyOn(relationshipManager['cache'], 'analyzePerformance').mockReturnValue(mockStats.performance);

      const result = relationshipManager.getRelationshipCacheStats();

      expect(result).toEqual(mockStats);
    });

    it('should warmup cache', async () => {
      const warmupSpy = jest.spyOn(relationshipManager['cache'], 'warmup').mockResolvedValue();

      await relationshipManager.warmupRelationshipCache([user], ['posts']);

      expect(warmupSpy).toHaveBeenCalledWith([user], ['posts'], expect.any(Function));
    });

    it('should cleanup expired cache', () => {
      const cleanupSpy = jest.spyOn(relationshipManager['cache'], 'cleanup').mockReturnValue(10);

      const result = relationshipManager.cleanupExpiredCache();

      expect(cleanupSpy).toHaveBeenCalled();
      expect(result).toBe(10);
    });

    it('should clear all cache', () => {
      const clearSpy = jest.spyOn(relationshipManager['cache'], 'clear');

      relationshipManager.clearRelationshipCache();

      expect(clearSpy).toHaveBeenCalled();
    });
  });

  describe('Error Handling', () => {
    it('should throw error for non-existent relationship', async () => {
      await expect(relationshipManager.loadRelationship(user, 'nonExistentRelation')).rejects.toThrow(
        "Relationship 'nonExistentRelation' not found on User"
      );
    });

    it('should throw error for unsupported relationship type', async () => {
      // Mock an invalid relationship type
      const originalRelationships = User.relationships;
      User.relationships = new Map();
      User.relationships.set('invalidRelation', {
        type: 'unsupported' as any,
        model: Post,
        foreignKey: 'userId',
        propertyKey: 'invalidRelation'
      });

      await expect(relationshipManager.loadRelationship(user, 'invalidRelation')).rejects.toThrow(
        'Unsupported relationship type: unsupported'
      );

      // Restore original relationships
      User.relationships = originalRelationships;
    });
  });
});