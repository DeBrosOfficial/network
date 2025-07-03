# Tests

This directory contains the test suite for the Debros Network framework.

## Structure

```
tests/
├── unit/                    # Unit tests for individual components
│   ├── core/               # Core framework components
│   ├── models/             # Model-related functionality  
│   ├── relationships/      # Relationship management
│   ├── sharding/          # Data sharding functionality
│   ├── decorators/        # Decorator functionality
│   └── migrations/        # Database migrations
├── real-integration/       # Real integration tests with Docker
│   └── blog-scenario/     # Complete blog application scenario
├── mocks/                 # Mock implementations for testing
└── setup.ts              # Test setup and configuration

```

## Running Tests

### Unit Tests
Run all unit tests (fast, uses mocks):
```bash
pnpm run test:unit
```

### Real Integration Tests
Run full integration tests with Docker (slower, uses real services):
```bash
pnpm run test:real
```

## Test Categories

- **Unit Tests**: Fast, isolated tests that use mocks for external dependencies
- **Real Integration Tests**: End-to-end tests that spin up actual IPFS nodes and OrbitDB instances using Docker

## Coverage

Unit tests provide code coverage reports in the `coverage/` directory after running.
