// Model decorator
export { Model } from './Model';

// Field decorator
export { Field, getFieldConfig } from './Field';

// Relationship decorators
export { BelongsTo, HasMany, HasOne, ManyToMany, getRelationshipConfig } from './relationships';

// Hook decorators
export {
  BeforeCreate,
  AfterCreate,
  BeforeUpdate,
  AfterUpdate,
  BeforeDelete,
  AfterDelete,
  BeforeSave,
  AfterSave,
  getHooks,
} from './hooks';

// Type exports
export type { ModelDecorator } from './Model';

export type { FieldDecorator } from './Field';

export type {
  BelongsToDecorator,
  HasManyDecorator,
  HasOneDecorator,
  ManyToManyDecorator,
} from './relationships';

export type { HookDecorator } from './hooks';
