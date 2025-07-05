import { describe, beforeEach, it, expect, jest } from '@jest/globals';
import { BaseModel } from '../../../src/framework/models/BaseModel';
import { Model, Field, BeforeCreate, AfterCreate, BeforeUpdate, AfterUpdate } from '../../../src/framework/models/decorators';
import { createMockServices } from '../../mocks/services';

// Test model for testing BaseModel functionality
@Model({
  scope: 'global',
  type: 'docstore'
})
class TestUser extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  declare username: string;

  @Field({ 
    type: 'string', 
    required: true, 
    unique: true,
    validate: (value: string) => {
      const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
      return emailRegex.test(value);
    }
  })
  declare email: string;

  @Field({ type: 'number', required: false, default: 0 })
  declare score: number;

  @Field({ type: 'boolean', required: false, default: true })
  declare isActive: boolean;

  @Field({ type: 'array', required: false, default: [] })
  declare tags: string[];

  @Field({ type: 'number', required: false })
  declare createdAt: number;

  @Field({ type: 'number', required: false })
  declare updatedAt: number;

  // Hook counters for testing
  static beforeCreateCount = 0;
  static afterCreateCount = 0;
  static beforeUpdateCount = 0;
  static afterUpdateCount = 0;

  @BeforeCreate()
  beforeCreateHook() {
    this.createdAt = Date.now();
    this.updatedAt = Date.now();
    TestUser.beforeCreateCount++;
  }

  @AfterCreate()
  afterCreateHook() {
    TestUser.afterCreateCount++;
  }

  @BeforeUpdate()
  beforeUpdateHook() {
    this.updatedAt = Date.now();
    TestUser.beforeUpdateCount++;
  }

  @AfterUpdate()
  afterUpdateHook() {
    TestUser.afterUpdateCount++;
  }

  // Custom validation method
  validateEmail(): boolean {
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return emailRegex.test(this.email);
  }
}

// Test model with validation
@Model({
  scope: 'user',
  type: 'docstore'
})
class TestPost extends BaseModel {
  @Field({ 
    type: 'string', 
    required: true,
    validate: (value: string) => {
      if (value.length < 3) {
        throw new Error('Title must be at least 3 characters');
      }
      return true;
    }
  })
  declare title: string;

  @Field({ 
    type: 'string', 
    required: true,
    validate: (value: string) => value.length <= 1000
  })
  declare content: string;

  @Field({ type: 'string', required: true })
  declare userId: string;

  @Field({ 
    type: 'array', 
    required: false, 
    default: [],
    transform: (tags: string[]) => tags.map(tag => tag.toLowerCase())
  })
  declare tags: string[];
}

describe('BaseModel', () => {
  let mockServices: any;

  beforeEach(() => {
    mockServices = createMockServices();
    
    // Clear the shared mock database to prevent test isolation issues
    if ((globalThis as any).__mockDatabase) {
      (globalThis as any).__mockDatabase.clear();
    }
    
    // Reset hook counters
    TestUser.beforeCreateCount = 0;
    TestUser.afterCreateCount = 0;
    TestUser.beforeUpdateCount = 0;
    TestUser.afterUpdateCount = 0;

    // Mock the framework initialization
    jest.clearAllMocks();
  });

  describe('Model Creation', () => {
    it('should create a new model instance with required fields', () => {
      const user = new TestUser();
      user.username = 'testuser';
      user.email = 'test@example.com';

      expect(user.username).toBe('testuser');
      expect(user.email).toBe('test@example.com');
      expect(user.score).toBe(0); // Default value
      expect(user.isActive).toBe(true); // Default value
      expect(user.tags).toEqual([]); // Default value
    });

    it('should generate a unique ID for new instances', () => {
      const user1 = new TestUser();
      const user2 = new TestUser();

      expect(user1.id).toBeDefined();
      expect(user2.id).toBeDefined();
      expect(user1.id).not.toBe(user2.id);
    });

    it('should create instance using static create method', async () => {
      const userData = {
        username: 'alice',
        email: 'alice@example.com',
        score: 100
      };

      const user = await TestUser.create(userData);

      expect(user).toBeInstanceOf(TestUser);
      expect(user.username).toBe('alice');
      expect(user.email).toBe('alice@example.com');
      expect(user.score).toBe(100);
      expect(user.isActive).toBe(true); // Default value
    });
  });

  describe('Validation', () => {
    it('should validate required fields on create', async () => {
      await expect(async () => {
        await TestUser.create({
          // Missing required username and email
          score: 50
        });
      }).rejects.toThrow();
    });

    it('should validate field constraints', async () => {
      await expect(async () => {
        await TestPost.create({
          title: 'Hi', // Too short (< 3 characters)
          content: 'Test content',
          userId: 'user123'
        });
      }).rejects.toThrow('Title must be at least 3 characters');
    });

    it('should apply field transformations', async () => {
      const post = await TestPost.create({
        title: 'Test Post',
        content: 'Test content',
        userId: 'user123',
        tags: ['JavaScript', 'TypeScript', 'REACT']
      });

      // Tags should be transformed to lowercase
      expect(post.tags).toEqual(['javascript', 'typescript', 'react']);
    });

    it('should validate field types', async () => {
      await expect(async () => {
        await TestUser.create({
          username: 'testuser',
          email: 'test@example.com',
          score: 'invalid-number' as any // Wrong type
        });
      }).rejects.toThrow();
    });
  });

  describe('CRUD Operations', () => {
    let user: TestUser;

    beforeEach(async () => {
      user = await TestUser.create({
        username: 'testuser',
        email: 'test@example.com',
        score: 50
      });
    });

    it('should save a model instance', async () => {
      user.score = 100;
      await user.save();

      expect(user.score).toBe(100);
      expect(TestUser.beforeUpdateCount).toBe(1);
      expect(TestUser.afterUpdateCount).toBe(1);
    });

    it('should find a model by ID', async () => {
      const foundUser = await TestUser.findById(user.id);

      expect(foundUser).toBeInstanceOf(TestUser);
      expect(foundUser?.id).toBe(user.id);
      expect(foundUser?.username).toBe(user.username);
    });

    it('should return null when model not found', async () => {
      const foundUser = await TestUser.findById('non-existent-id');
      expect(foundUser).toBeNull();
    });

    it('should find model by criteria', async () => {
      const foundUser = await TestUser.findOne({ username: 'testuser' });

      expect(foundUser).toBeInstanceOf(TestUser);
      expect(foundUser?.username).toBe('testuser');
    });

    it('should delete a model instance', async () => {
      const userId = user.id;
      await user.delete();

      const foundUser = await TestUser.findById(userId);
      expect(foundUser).toBeNull();
    });

    it('should find all models', async () => {
      // Create another user with unique username and email
      await TestUser.create({
        username: 'testuser2',
        email: 'testuser2@example.com'
      });

      const allUsers = await TestUser.findAll();
      expect(allUsers.length).toBeGreaterThanOrEqual(2);
      expect(allUsers.every(u => u instanceof TestUser)).toBe(true);
    });
  });

  describe('Model Hooks', () => {
    it('should execute beforeCreate and afterCreate hooks', async () => {
      const initialBeforeCount = TestUser.beforeCreateCount;
      const initialAfterCount = TestUser.afterCreateCount;

      const user = await TestUser.create({
        username: 'hooktest',
        email: 'hook@example.com'
      });

      expect(TestUser.beforeCreateCount).toBe(initialBeforeCount + 1);
      expect(TestUser.afterCreateCount).toBe(initialAfterCount + 1);
      expect(user.createdAt).toBeDefined();
      expect(user.updatedAt).toBeDefined();
    });

    it('should execute beforeUpdate and afterUpdate hooks', async () => {
      const user = await TestUser.create({
        username: 'updatetest',
        email: 'update@example.com'
      });

      const initialBeforeCount = TestUser.beforeUpdateCount;
      const initialAfterCount = TestUser.afterUpdateCount;
      const initialUpdatedAt = user.updatedAt;

      // Wait a bit to ensure different timestamp
      await new Promise(resolve => setTimeout(resolve, 10));

      user.score = 100;
      await user.save();

      expect(TestUser.beforeUpdateCount).toBe(initialBeforeCount + 1);
      expect(TestUser.afterUpdateCount).toBe(initialAfterCount + 1);
      expect(user.updatedAt).toBeGreaterThan(initialUpdatedAt!);
    });
  });

  describe('Serialization', () => {
    it('should serialize to JSON correctly', async () => {
      const user = await TestUser.create({
        username: 'serialtest',
        email: 'serial@example.com',
        score: 75,
        tags: ['test', 'user']
      });

      const json = user.toJSON();

      expect(json).toMatchObject({
        id: user.id,
        username: 'serialtest',
        email: 'serial@example.com',
        score: 75,
        isActive: true,
        tags: ['test', 'user'],
        createdAt: expect.any(Number),
        updatedAt: expect.any(Number)
      });
    });

    it('should create instance from JSON', () => {
      const data = {
        id: 'test-id',
        username: 'fromjson',
        email: 'json@example.com',
        score: 80,
        isActive: false,
        tags: ['json'],
        createdAt: Date.now(),
        updatedAt: Date.now()
      };

      const user = TestUser.fromJSON(data);

      expect(user).toBeInstanceOf(TestUser);
      expect(user.id).toBe('test-id');
      expect(user.username).toBe('fromjson');
      expect(user.email).toBe('json@example.com');
      expect(user.score).toBe(80);
      expect(user.isActive).toBe(false);
      expect(user.tags).toEqual(['json']);
    });
  });

  describe('Query Interface', () => {
    it('should provide query interface', () => {
      const queryBuilder = TestUser.query();

      expect(queryBuilder).toBeDefined();
      expect(typeof queryBuilder.where).toBe('function');
      expect(typeof queryBuilder.find).toBe('function');
      expect(typeof queryBuilder.findOne).toBe('function');
      expect(typeof queryBuilder.count).toBe('function');
    });

    it('should support method chaining in queries', () => {
      const queryBuilder = TestUser.query()
        .where('isActive', true)
        .where('score', '>', 50)
        .orderBy('username')
        .limit(10);

      expect(queryBuilder).toBeDefined();
      // The query builder should return itself for chaining
      expect(typeof queryBuilder.find).toBe('function');
    });
  });

  describe('Field Modification Tracking', () => {
    it('should track field modifications', async () => {
      const user = await TestUser.create({
        username: 'tracktest',
        email: 'track@example.com'
      });

      expect(user.isFieldModified('username')).toBe(false);

      user.username = 'newusername';
      expect(user.isFieldModified('username')).toBe(true);

      user.score = 100;
      expect(user.isFieldModified('score')).toBe(true);
      expect(user.isFieldModified('email')).toBe(false);
    });

    it('should get modified fields', async () => {
      const user = await TestUser.create({
        username: 'modifytest',
        email: 'modify@example.com'
      });

      user.username = 'newusername';
      user.score = 200;

      const modifiedFields = user.getModifiedFields();
      expect(modifiedFields).toContain('username');
      expect(modifiedFields).toContain('score');
      expect(modifiedFields).not.toContain('email');
    });

    it('should clear modifications after save', async () => {
      const user = await TestUser.create({
        username: 'cleartest',
        email: 'clear@example.com'
      });

      user.username = 'newusername';
      expect(user.isFieldModified('username')).toBe(true);

      await user.save();
      expect(user.isFieldModified('username')).toBe(false);
    });
  });

  describe('Error Handling', () => {
    it('should handle validation errors gracefully', async () => {
      try {
        await TestPost.create({
          // Missing required title
          content: 'Test content',
          userId: 'user123'
        });
        fail('Should have thrown validation error');
      } catch (error: any) {
        expect(error.message).toContain('required');
      }
    });

    it('should handle database errors gracefully', async () => {
      // This would test database connection errors, timeouts, etc.
      // For now, we'll test with a simple validation error
      const user = new TestUser();
      user.username = 'test';

      expect(() => {
        user.email = 'invalid-email'; // Invalid email format
      }).toThrow();
    });
  });

  describe('Custom Methods', () => {
    it('should support custom validation methods', async () => {
      const user = await TestUser.create({
        username: 'emailtest',
        email: 'valid@example.com'
      });

      expect(user.validateEmail()).toBe(true);

      // Test that setting an invalid email throws validation error
      expect(() => {
        user.email = 'invalid-email';
      }).toThrow('email failed custom validation');
      
      // Email should still be the original valid value
      expect(user.email).toBe('valid@example.com');
      expect(user.validateEmail()).toBe(true);
    });
  });
});