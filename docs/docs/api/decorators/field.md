---
sidebar_position: 2
---

# @Field Decorator

The `@Field` decorator is used to define field properties and validation rules in DebrosFramework models.

## Overview

The `@Field` decorator configures how a model property should be handled, including type validation, required status, default values, and custom validation functions.

## Syntax

```typescript
@Field(config: FieldConfig)
propertyName: PropertyType;
```

## Configuration Options

### FieldConfig Interface

```typescript
interface FieldConfig {
  type: FieldType;
  required?: boolean;
  unique?: boolean;
  default?: any | (() => any);
  validate?: (value: any) => boolean;
  transform?: (value: any) => any;
  serialize?: boolean;
  index?: boolean;
  virtual?: boolean;
}
```

## Field Types

### Supported Types

```typescript
type FieldType = 'string' | 'number' | 'boolean' | 'array' | 'object' | 'date';
```

### Basic Usage

```typescript
class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'number', required: false, default: 0 })
  score: number;

  @Field({ type: 'boolean', default: true })
  isActive: boolean;
}
```

## Configuration Properties

### type (required)

Specifies the data type of the field.

```typescript
@Field({ type: 'string' })
name: string;

@Field({ type: 'number' })
age: number;

@Field({ type: 'boolean' })
isActive: boolean;

@Field({ type: 'array' })
tags: string[];

@Field({ type: 'object' })
metadata: Record<string, any>;
```

### required (optional)

Indicates whether the field is required.

```typescript
@Field({ type: 'string', required: true })
email: string;

@Field({ type: 'string', required: false })
bio?: string;
```

### unique (optional)

Ensures field values are unique across all records.

```typescript
@Field({ type: 'string', required: true, unique: true })
email: string;
```

### default (optional)

Sets a default value for the field.

```typescript
// Static default
@Field({ type: 'boolean', default: true })
isActive: boolean;

// Dynamic default
@Field({ type: 'number', default: () => Date.now() })
createdAt: number;
```

### validate (optional)

Custom validation function for the field.

```typescript
@Field({
  type: 'string',
  validate: (value: string) => value.length >= 3 && value.length <= 20
})
username: string;

@Field({
  type: 'string',
  validate: (email: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)
})
email: string;
```

### transform (optional)

Transforms the value before storing.

```typescript
@Field({
  type: 'string',
  transform: (value: string) => value.toLowerCase().trim()
})
username: string;
```

## Examples

### Basic Field Configuration

```typescript
class User extends BaseModel {
  @Field({ type: 'string', required: true, unique: true })
  username: string;

  @Field({
    type: 'string',
    required: true,
    unique: true,
    validate: (email: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)
  })
  email: string;

  @Field({ type: 'string', required: false })
  bio?: string;

  @Field({ type: 'boolean', default: true })
  isActive: boolean;

  @Field({ type: 'number', default: () => Date.now() })
  createdAt: number;
}
```

### Complex Field Validation

```typescript
class Post extends BaseModel {
  @Field({
    type: 'string',
    required: true,
    validate: (title: string) => title.length >= 3 && title.length <= 100
  })
  title: string;

  @Field({
    type: 'string',
    required: true,
    validate: (content: string) => content.length <= 5000
  })
  content: string;

  @Field({
    type: 'array',
    default: [],
    validate: (tags: string[]) => tags.length <= 10
  })
  tags: string[];

  @Field({
    type: 'string',
    transform: (slug: string) => slug.toLowerCase().replace(/\s+/g, '-')
  })
  slug: string;
}
```

## Related Decorators

- [`@Model`](./model) - Model configuration
- [`@BelongsTo`](./relationships#belongsto) - Relationship decorators
- [`@HasMany`](./relationships#hasmany) - Relationship decorators
