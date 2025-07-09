---
sidebar_position: 1
---

# Automatic Pinning

Automatic pinning optimizes data availability by keeping frequently accessed data readily available across the network.

## Overview

DebrosFramework includes automatic pinning functionality that intelligently pins important data to improve performance and availability.

## Pinning Strategies

### Fixed Pinning

Pin specific content permanently:

```typescript
@Model({
  scope: 'global',
  type: 'docstore',
  pinning: {
    strategy: 'fixed',
    maxPins: 1000
  }
})
class ImportantData extends BaseModel {
  // Model definition
}
```

### Popularity-based Pinning

Pin content based on access frequency:

```typescript
@Model({
  scope: 'global',
  type: 'docstore',
  pinning: {
    strategy: 'popularity',
    factor: 2,
    ttl: 3600000 // 1 hour
  }
})
class PopularContent extends BaseModel {
  // Model definition
}
```

### Tiered Pinning

Use multiple tiers for different pin priorities:

```typescript
@Model({
  scope: 'global',
  type: 'docstore',
  pinning: {
    strategy: 'tiered',
    maxPins: 500,
    factor: 1.5
  }
})
class TieredData extends BaseModel {
  // Model definition
}
```

## Configuration

### Pinning Configuration

```typescript
interface PinningConfig {
  strategy: 'fixed' | 'popularity' | 'tiered';
  factor?: number;
  maxPins?: number;
  ttl?: number;
}
```

### Framework-level Configuration

```typescript
const framework = new DebrosFramework({
  automaticPinning: {
    enabled: true,
    strategy: 'popularity',
    maxPins: 1000
  }
});
```

## Benefits

1. **Improved Performance** - Faster access to frequently used data
2. **Better Availability** - Reduced risk of data unavailability
3. **Automatic Management** - No manual intervention required
4. **Scalable** - Adapts to usage patterns

## Related Topics

- [Database Management](../core-concepts/database-management) - Database handling
- [Performance Optimization](./performance) - General performance tips
