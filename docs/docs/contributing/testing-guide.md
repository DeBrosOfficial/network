---
sidebar_position: 1
---

# Testing Guide

This document provides guidelines for writing and running tests in DebrosFramework.

## Overview

DebrosFramework uses Jest as the testing framework for unit tests and a combination of Docker and custom scripts for integration testing.

## Unit Tests

### Structure

Unit tests are located in the `tests/unit/` directory and follow the naming convention `*.test.ts`.

### Running Tests

```bash
pnpm run test:unit
```

Use the following command to run specific tests:

```bash
npx jest tests/unit/path/to/your.test.ts
```

### Best Practices

- Ensure comprehensive coverage for all public methods.
- Use `describe` blocks to group related tests.
- Use `beforeEach` and `afterEach` for setup and teardown logic.

## Integration Tests

### Structure

Integration tests are located in the `tests/real-integration/` directory and simulate real-world scenarios.

### Running Tests

Ensure Docker is running before executing:

```bash
pnpm run test:real
```

### Best Practices

- Test real-world use cases that involve multiple components.
- Use Docker to simulate network environments and distributed systems.
- Validate data consistency and integrity.

## Writing Tests

### Unit Test Example

Here's a simple example of a unit test:

```typescript
describe('User Model', () => {
  it('should create a new user', async () => {
    const user = await User.create({ username: 'testuser', email: 'test@example.com' });
    expect(user).toBeDefined();
    expect(user.username).toBe('testuser');
  });
});
```

### Integration Test Example

Here's an example of a simple integration test:

```typescript
describe('Blog Scenario Integration', () => {
  it('should handle complete blog workflow', async () => {
    const user = await User.create({ username: 'blogger', email: 'blogger@example.com' });
    const post = await Post.create({ title: 'Test Post', content: 'Test content', userId: user.id });
    const postWithComments = await Post.query().where('id', post.id).with(['comments']).findOne();

    expect(postWithComments).toBeDefined();
  });
});
```

## Test Utilities

### Shared Test Utilities

Test utilities are located in `tests/shared/` and provide common functions and mocks for tests.

### Setup and Teardown

Use Jest hooks for setup and teardown logic:

```typescript
beforeEach(async () => {
  await setupTestDatabase();
});

afterEach(async () => {
  await cleanupTestDatabase();
});
```

## Conclusion

- Always aim for high test coverage and meaningful scenarios.
- Validate each critical aspect of your code with real-world data.
- Continually run tests during development to catch issues early.

Follow the guidelines above to ensure your contributions maintain the reliability and performance of DebrosFramework.
