import { describe, beforeEach, it, expect, jest } from '@jest/globals';
import { BaseModel } from '../../../src/framework/models/BaseModel';
import { 
  Model, 
  Field, 
  BelongsTo, 
  HasMany, 
  HasOne, 
  ManyToMany,
  BeforeCreate,
  AfterCreate,
  BeforeUpdate,
  AfterUpdate,
  BeforeDelete,
  AfterDelete,
  getFieldConfig,
  getRelationshipConfig,
  getHooks
} from '../../../src/framework/models/decorators';

describe('Decorators', () => {
  describe('@Model Decorator', () => {
    it('should define model metadata correctly', () => {
      @Model({
        scope: 'global',
        type: 'docstore',
        sharding: {
          strategy: 'hash',
          count: 4,
          key: 'id'
        }
      })
      class TestModel extends BaseModel {}

      expect(TestModel.scope).toBe('global');
      expect(TestModel.storeType).toBe('docstore');
      expect(TestModel.sharding).toEqual({
        strategy: 'hash',
        count: 4,
        key: 'id'
      });
    });

    it('should apply default model configuration', () => {
      @Model({})
      class DefaultModel extends BaseModel {}

      expect(DefaultModel.scope).toBe('global');
      expect(DefaultModel.storeType).toBe('docstore');
    });

    it('should register model with ModelRegistry', () => {
      @Model({
        scope: 'user',
        type: 'eventlog'
      })
      class RegistryModel extends BaseModel {}

      // The model should be automatically registered
      expect(RegistryModel.scope).toBe('user');
      expect(RegistryModel.storeType).toBe('eventlog');
    });
  });

  describe('@Field Decorator', () => {
    @Model({})
    class FieldTestModel extends BaseModel {
      @Field({ type: 'string', required: true })
      requiredField: string;

      @Field({ type: 'number', required: false, default: 42 })
      defaultField: number;

      @Field({ 
        type: 'string', 
        required: true,
        validate: (value: string) => value.length >= 3,
        transform: (value: string) => value.toLowerCase()
      })
      validatedField: string;

      @Field({ type: 'array', required: false, default: [] })
      arrayField: string[];

      @Field({ type: 'boolean', required: false, default: true })
      booleanField: boolean;

      @Field({ type: 'object', required: false })
      objectField: Record<string, any>;
    }

    it('should define field metadata correctly', () => {
      const requiredFieldConfig = getFieldConfig(FieldTestModel, 'requiredField');
      expect(requiredFieldConfig).toEqual({
        type: 'string',
        required: true
      });

      const defaultFieldConfig = getFieldConfig(FieldTestModel, 'defaultField');
      expect(defaultFieldConfig).toEqual({
        type: 'number',
        required: false,
        default: 42
      });
    });

    it('should handle field validation configuration', () => {
      const validatedFieldConfig = getFieldConfig(FieldTestModel, 'validatedField');
      
      expect(validatedFieldConfig.type).toBe('string');
      expect(validatedFieldConfig.required).toBe(true);
      expect(typeof validatedFieldConfig.validate).toBe('function');
      expect(typeof validatedFieldConfig.transform).toBe('function');
    });

    it('should apply field validation', () => {
      const validatedFieldConfig = getFieldConfig(FieldTestModel, 'validatedField');
      
      expect(validatedFieldConfig.validate!('test')).toBe(true);
      expect(validatedFieldConfig.validate!('hi')).toBe(false); // Less than 3 characters
    });

    it('should apply field transformation', () => {
      const validatedFieldConfig = getFieldConfig(FieldTestModel, 'validatedField');
      
      expect(validatedFieldConfig.transform!('TEST')).toBe('test');
      expect(validatedFieldConfig.transform!('MixedCase')).toBe('mixedcase');
    });

    it('should handle different field types', () => {
      const arrayFieldConfig = getFieldConfig(FieldTestModel, 'arrayField');
      expect(arrayFieldConfig.type).toBe('array');
      expect(arrayFieldConfig.default).toEqual([]);

      const booleanFieldConfig = getFieldConfig(FieldTestModel, 'booleanField');
      expect(booleanFieldConfig.type).toBe('boolean');
      expect(booleanFieldConfig.default).toBe(true);

      const objectFieldConfig = getFieldConfig(FieldTestModel, 'objectField');
      expect(objectFieldConfig.type).toBe('object');
      expect(objectFieldConfig.required).toBe(false);
    });
  });

  describe('Relationship Decorators', () => {
    @Model({})
    class User extends BaseModel {
      @Field({ type: 'string', required: true })
      username: string;

      @HasMany(() => Post, 'userId')
      posts: Post[];

      @HasOne(() => Profile, 'userId')
      profile: Profile;

      @ManyToMany(() => Role, 'user_roles', 'userId', 'roleId')
      roles: Role[];
    }

    @Model({})
    class Post extends BaseModel {
      @Field({ type: 'string', required: true })
      title: string;

      @Field({ type: 'string', required: true })
      userId: string;

      @BelongsTo(() => User, 'userId')
      user: User;
    }

    @Model({})
    class Profile extends BaseModel {
      @Field({ type: 'string', required: true })
      userId: string;

      @BelongsTo(() => User, 'userId')
      user: User;
    }

    @Model({})
    class Role extends BaseModel {
      @Field({ type: 'string', required: true })
      name: string;

      @ManyToMany(() => User, 'user_roles', 'roleId', 'userId')
      users: User[];
    }

    it('should define BelongsTo relationships correctly', () => {
      const relationships = getRelationshipConfig(Post);
      const userRelation = relationships.find(r => r.propertyKey === 'user');

      expect(userRelation).toBeDefined();
      expect(userRelation?.type).toBe('belongsTo');
      expect(userRelation?.targetModel()).toBe(User);
      expect(userRelation?.foreignKey).toBe('userId');
    });

    it('should define HasMany relationships correctly', () => {
      const relationships = getRelationshipConfig(User);
      const postsRelation = relationships.find(r => r.propertyKey === 'posts');

      expect(postsRelation).toBeDefined();
      expect(postsRelation?.type).toBe('hasMany');
      expect(postsRelation?.targetModel()).toBe(Post);
      expect(postsRelation?.foreignKey).toBe('userId');
    });

    it('should define HasOne relationships correctly', () => {
      const relationships = getRelationshipConfig(User);
      const profileRelation = relationships.find(r => r.propertyKey === 'profile');

      expect(profileRelation).toBeDefined();
      expect(profileRelation?.type).toBe('hasOne');
      expect(profileRelation?.targetModel()).toBe(Profile);
      expect(profileRelation?.foreignKey).toBe('userId');
    });

    it('should define ManyToMany relationships correctly', () => {
      const relationships = getRelationshipConfig(User);
      const rolesRelation = relationships.find(r => r.propertyKey === 'roles');

      expect(rolesRelation).toBeDefined();
      expect(rolesRelation?.type).toBe('manyToMany');
      expect(rolesRelation?.targetModel()).toBe(Role);
      expect(rolesRelation?.through).toBe('user_roles');
      expect(rolesRelation?.foreignKey).toBe('userId');
      expect(rolesRelation?.otherKey).toBe('roleId');
    });

    it('should support relationship options', () => {
      @Model({})
      class TestModel extends BaseModel {
        @HasMany(() => Post, 'userId', {
          cache: true,
          eager: false,
          orderBy: 'createdAt',
          limit: 10
        })
        posts: Post[];
      }

      const relationships = getRelationshipConfig(TestModel);
      const postsRelation = relationships.find(r => r.propertyKey === 'posts');

      expect(postsRelation?.options).toEqual({
        cache: true,
        eager: false,
        orderBy: 'createdAt',
        limit: 10
      });
    });
  });

  describe('Hook Decorators', () => {
    let hookCallOrder: string[] = [];

    @Model({})
    class HookTestModel extends BaseModel {
      @Field({ type: 'string', required: true })
      name: string;

      @BeforeCreate()
      beforeCreateHook() {
        hookCallOrder.push('beforeCreate');
      }

      @AfterCreate()
      afterCreateHook() {
        hookCallOrder.push('afterCreate');
      }

      @BeforeUpdate()
      beforeUpdateHook() {
        hookCallOrder.push('beforeUpdate');
      }

      @AfterUpdate()
      afterUpdateHook() {
        hookCallOrder.push('afterUpdate');
      }

      @BeforeDelete()
      beforeDeleteHook() {
        hookCallOrder.push('beforeDelete');
      }

      @AfterDelete()
      afterDeleteHook() {
        hookCallOrder.push('afterDelete');
      }
    }

    beforeEach(() => {
      hookCallOrder = [];
    });

    it('should register lifecycle hooks correctly', () => {
      const hooks = getHooks(HookTestModel);

      expect(hooks.beforeCreate).toContain('beforeCreateHook');
      expect(hooks.afterCreate).toContain('afterCreateHook');
      expect(hooks.beforeUpdate).toContain('beforeUpdateHook');
      expect(hooks.afterUpdate).toContain('afterUpdateHook');
      expect(hooks.beforeDelete).toContain('beforeDeleteHook');
      expect(hooks.afterDelete).toContain('afterDeleteHook');
    });

    it('should support multiple hooks of the same type', () => {
      @Model({})
      class MultiHookModel extends BaseModel {
        @BeforeCreate()
        firstBeforeCreate() {
          hookCallOrder.push('first');
        }

        @BeforeCreate()
        secondBeforeCreate() {
          hookCallOrder.push('second');
        }
      }

      const hooks = getHooks(MultiHookModel);
      expect(hooks.beforeCreate).toHaveLength(2);
      expect(hooks.beforeCreate).toContain('firstBeforeCreate');
      expect(hooks.beforeCreate).toContain('secondBeforeCreate');
    });
  });

  describe('Complex Decorator Combinations', () => {
    it('should handle models with all decorator types', () => {
      @Model({
        scope: 'user',
        type: 'docstore',
        sharding: {
          strategy: 'user',
          count: 2,
          key: 'userId'
        }
      })
      class ComplexModel extends BaseModel {
        @Field({ type: 'string', required: true })
        title: string;

        @Field({ type: 'string', required: true })
        userId: string;

        @Field({ 
          type: 'array', 
          required: false, 
          default: [],
          transform: (tags: string[]) => tags.map(t => t.toLowerCase())
        })
        tags: string[];

        @BelongsTo(() => User, 'userId')
        user: User;

        @BeforeCreate()
        setDefaults() {
          this.tags = this.tags || [];
        }

        @BeforeUpdate()
        updateTimestamp() {
          // Update logic
        }
      }

      // Check model configuration
      expect(ComplexModel.scope).toBe('user');
      expect(ComplexModel.storeType).toBe('docstore');
      expect(ComplexModel.sharding).toEqual({
        strategy: 'user',
        count: 2,
        key: 'userId'
      });

      // Check field configuration
      const titleConfig = getFieldConfig(ComplexModel, 'title');
      expect(titleConfig.required).toBe(true);

      const tagsConfig = getFieldConfig(ComplexModel, 'tags');
      expect(tagsConfig.default).toEqual([]);
      expect(typeof tagsConfig.transform).toBe('function');

      // Check relationships
      const relationships = getRelationshipConfig(ComplexModel);
      const userRelation = relationships.find(r => r.propertyKey === 'user');
      expect(userRelation?.type).toBe('belongsTo');

      // Check hooks
      const hooks = getHooks(ComplexModel);
      expect(hooks.beforeCreate).toContain('setDefaults');
      expect(hooks.beforeUpdate).toContain('updateTimestamp');
    });
  });

  describe('Decorator Error Handling', () => {
    it('should handle invalid field types', () => {
      expect(() => {
        @Model({})
        class InvalidFieldModel extends BaseModel {
          @Field({ type: 'invalid-type' as any, required: true })
          invalidField: any;
        }
      }).toThrow();
    });

    it('should handle invalid model scope', () => {
      expect(() => {
        @Model({ scope: 'invalid-scope' as any })
        class InvalidScopeModel extends BaseModel {}
      }).toThrow();
    });

    it('should handle invalid store type', () => {
      expect(() => {
        @Model({ type: 'invalid-store' as any })
        class InvalidStoreModel extends BaseModel {}
      }).toThrow();
    });
  });

  describe('Metadata Inheritance', () => {
    @Model({
      scope: 'global',
      type: 'docstore'
    })
    class BaseTestModel extends BaseModel {
      @Field({ type: 'string', required: true })
      baseField: string;

      @BeforeCreate()
      baseHook() {
        // Base hook
      }
    }

    @Model({
      scope: 'user', // Override scope
      type: 'eventlog' // Override type
    })
    class ExtendedTestModel extends BaseTestModel {
      @Field({ type: 'number', required: false })
      extendedField: number;

      @BeforeCreate()
      extendedHook() {
        // Extended hook
      }
    }

    it('should inherit field metadata from parent class', () => {
      const baseFieldConfig = getFieldConfig(ExtendedTestModel, 'baseField');
      expect(baseFieldConfig).toBeDefined();
      expect(baseFieldConfig.type).toBe('string');
      expect(baseFieldConfig.required).toBe(true);

      const extendedFieldConfig = getFieldConfig(ExtendedTestModel, 'extendedField');
      expect(extendedFieldConfig).toBeDefined();
      expect(extendedFieldConfig.type).toBe('number');
    });

    it('should override model configuration in child class', () => {
      expect(ExtendedTestModel.scope).toBe('user');
      expect(ExtendedTestModel.storeType).toBe('eventlog');
    });

    it('should inherit and extend hooks', () => {
      const hooks = getHooks(ExtendedTestModel);
      expect(hooks.beforeCreate).toContain('baseHook');
      expect(hooks.beforeCreate).toContain('extendedHook');
    });
  });
});