# Contributing to DebrosFramework

Thank you for your interest in contributing to DebrosFramework! This document provides guidelines and information for contributors.

## Development Status

DebrosFramework is currently in **beta (v0.5.0-beta)** and under active development. We welcome contributions from the community to help improve the framework.

## Getting Started

### Prerequisites

- Node.js 18.0 or higher
- npm or pnpm package manager
- Git
- TypeScript knowledge
- Familiarity with IPFS and OrbitDB concepts

### Development Setup

1. **Fork and Clone**

   ```bash
   git clone https://github.com/YOUR_USERNAME/network.git
   cd network
   ```

2. **Install Dependencies**

   ```bash
   pnpm install
   ```

3. **Build the Project**

   ```bash
   pnpm run build
   ```

4. **Run Tests**

   ```bash
   # Unit tests
   pnpm run test:unit

   # Integration tests
   pnpm run test:real
   ```

## Project Structure

```
src/framework/
├── core/           # Core framework components
├── models/         # Model system and decorators
├── query/          # Query builder and execution
├── relationships/  # Relationship management
├── sharding/       # Data sharding logic
├── migrations/     # Schema migration system
├── pinning/        # Automatic pinning features
├── pubsub/         # Event publishing system
└── types/          # TypeScript type definitions

docs/docs/          # Documentation source
tests/              # Test suites
└── real-integration/ # Integration test scenarios
```

## How to Contribute

### Types of Contributions

We welcome the following types of contributions:

1. **🐛 Bug Reports** - Report issues and bugs
2. **✨ Feature Requests** - Suggest new features
3. **📖 Documentation** - Improve docs and examples
4. **🔧 Code Contributions** - Bug fixes and new features
5. **🧪 Testing** - Add tests and improve test coverage
6. **💡 Examples** - Create usage examples and tutorials

### Bug Reports

When reporting bugs, please include:

- **Clear description** of the issue
- **Steps to reproduce** the problem
- **Expected vs actual behavior**
- **Environment details** (Node.js version, OS, etc.)
- **Code examples** that demonstrate the issue
- **Error messages** and stack traces

Use our bug report template:

````markdown
## Bug Description

[Clear description of the bug]

## Steps to Reproduce

1. [First step]
2. [Second step]
3. [etc.]

## Expected Behavior

[What you expected to happen]

## Actual Behavior

[What actually happened]

## Environment

- DebrosFramework version: [version]
- Node.js version: [version]
- OS: [operating system]

## Code Example

```typescript
// Minimal code example that reproduces the issue
```
````

````

### Feature Requests

For feature requests, please provide:

- **Clear use case** and motivation
- **Detailed description** of the proposed feature
- **API design suggestions** (if applicable)
- **Examples** of how it would be used
- **Alternatives considered**

### Code Contributions

#### Before You Start

1. **Check existing issues** to avoid duplicate work
2. **Discuss large changes** in an issue first
3. **Follow the coding standards** outlined below
4. **Write tests** for your changes
5. **Update documentation** as needed

#### Development Workflow

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
````

2. **Make your changes**

   - Write clean, well-documented code
   - Follow TypeScript best practices
   - Add tests for new functionality
   - Update relevant documentation

3. **Test your changes**

   ```bash
   pnpm run test:unit
   pnpm run test:real
   pnpm run lint
   ```

4. **Commit your changes**

   ```bash
   git add .
   git commit -m "feat: add new feature description"
   ```

5. **Push and create PR**
   ```bash
   git push origin feature/your-feature-name
   ```

#### Commit Message Format

We use conventional commits for consistent commit messages:

```
type(scope): description

body (optional)

footer (optional)
```

**Types:**

- `feat`: New features
- `fix`: Bug fixes
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**

```
feat(models): add support for computed fields
fix(query): resolve relationship loading issue
docs(readme): update installation instructions
```

#### Code Style Guidelines

1. **TypeScript**

   - Use strict TypeScript configuration
   - Provide proper type annotations
   - Use interfaces for object types
   - Follow naming conventions

2. **Formatting**

   - Use Prettier for code formatting
   - Run `pnpm run format` before committing
   - Use 2 spaces for indentation

3. **ESLint**

   - Follow ESLint rules
   - Run `pnpm run lint` and fix any issues
   - Use `pnpm run lint:fix` for auto-fixes

4. **Documentation**
   - Add JSDoc comments for public APIs
   - Update relevant documentation files
   - Include code examples where appropriate

#### Testing Guidelines

1. **Unit Tests**

   - Write tests for all new functionality
   - Use Jest for unit testing
   - Aim for high code coverage
   - Test edge cases and error conditions

2. **Integration Tests**

   - Add integration tests for significant features
   - Test real-world scenarios
   - Use the blog scenario tests as reference

3. **Test Structure**

   ```typescript
   describe('FeatureName', () => {
     beforeEach(() => {
       // Setup
     });

     it('should behave correctly in normal case', () => {
       // Test implementation
     });

     it('should handle edge case', () => {
       // Edge case test
     });

     it('should throw error for invalid input', () => {
       // Error case test
     });
   });
   ```

## Documentation

### Documentation Structure

- **README.md** - Overview and quick start
- **docs/docs/intro.md** - Framework introduction
- **docs/docs/getting-started.md** - Setup guide
- **docs/docs/core-concepts/** - Architecture and concepts
- **docs/docs/api/** - API reference
- **docs/docs/examples/** - Usage examples

### Writing Documentation

1. **Use clear, concise language**
2. **Provide code examples**
3. **Include both basic and advanced usage**
4. **Keep examples up-to-date**
5. **Add diagrams where helpful**

### Building Documentation

```bash
cd docs
npm install
npm run start  # Development server
npm run build  # Production build
```

## Release Process

Releases are managed by the core team and follow semantic versioning:

- **Patch** (0.5.1): Bug fixes and small improvements
- **Minor** (0.6.0): New features, backward compatible
- **Major** (1.0.0): Breaking changes

## Community Guidelines

### Code of Conduct

We are committed to providing a welcoming and inclusive environment. Please:

- **Be respectful** and considerate
- **Use inclusive language**
- **Accept constructive feedback**
- **Focus on what's best** for the community
- **Show empathy** towards other contributors

### Getting Help

- **GitHub Issues** - For bug reports and feature requests
- **GitHub Discussions** - For questions and community discussion
- **Discord** - For real-time chat and support

## Recognition

Contributors will be recognized in:

- **CONTRIBUTORS.md** file
- **Release notes** for significant contributions
- **Documentation credits** for doc contributions

## Questions?

If you have questions about contributing, please:

1. Check existing documentation
2. Search GitHub issues
3. Ask in GitHub Discussions
4. Contact the maintainers

Thank you for contributing to DebrosFramework! 🚀
