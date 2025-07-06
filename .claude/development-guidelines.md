# DebrosFramework Development Guidelines

## Code Style and Standards

### TypeScript Configuration
- **Strict Mode**: Always use strict TypeScript configuration
- **Decorators**: Enable `experimentalDecorators` and `emitDecoratorMetadata`
- **Target**: ES2020 for modern JavaScript features
- **Module System**: ES modules with CommonJS interop

### Code Formatting
- **Prettier**: Use Prettier for consistent code formatting
- **ESLint**: Follow ESLint rules for code quality
- **Indentation**: 2 spaces for all files
- **Line Length**: Max 120 characters per line
- **Semicolons**: Always use semicolons
- **Quotes**: Single quotes for strings, double quotes for JSX

### Naming Conventions
- **Classes**: PascalCase (e.g., `UserModel`, `QueryBuilder`)
- **Functions/Methods**: camelCase (e.g., `createUser`, `findById`)
- **Variables**: camelCase (e.g., `userId`, `queryResult`)
- **Constants**: UPPER_SNAKE_CASE (e.g., `DEFAULT_CACHE_SIZE`)
- **Files**: PascalCase for classes, camelCase for utilities
- **Interfaces**: PascalCase with descriptive names (e.g., `ModelConfig`)

## Framework Architecture Patterns

### Model System
```typescript
// Always extend BaseModel for data models
@Model({
  scope: 'user' | 'global',
  type: 'docstore',
  sharding: { strategy: 'hash', count: 4, key: 'fieldName' }
})
export class ModelName extends BaseModel {
  @Field({ type: 'string', required: true })
  propertyName: string;
  
  @BelongsTo(() => RelatedModel, 'foreignKey')
  relation: RelatedModel;
}
```

### Query Patterns
```typescript
// Use chainable query builder pattern
const results = await Model.query()
  .where('field', 'operator', value)
  .with(['relationship1', 'relationship2'])
  .orderBy('field', 'direction')
  .limit(count)
  .find();

// Prefer specific methods over generic ones
const user = await User.findById(id);           // Good
const user = await User.query().where('id', id).findOne(); // Less preferred
```

### Error Handling
```typescript
// Always use try-catch for async operations
try {
  const result = await someAsyncOperation();
  return result;
} catch (error) {
  console.error('Operation failed:', error);
  throw new DebrosFrameworkError('Descriptive error message', { originalError: error });
}

// Validate inputs early
if (!userId || typeof userId !== 'string') {
  throw new ValidationError('User ID must be a non-empty string');
}
```

### Service Layer Pattern
```typescript
// Encapsulate business logic in service classes
export class UserService {
  async createUser(userData: CreateUserData): Promise<User> {
    // Validation
    await this.validateUserData(userData);
    
    // Business logic
    const user = await User.create(userData);
    
    // Post-processing
    await this.sendWelcomeEmail(user);
    
    return user;
  }
  
  private async validateUserData(data: CreateUserData): Promise<void> {
    // Validation logic
  }
}
```

## Testing Standards

### Unit Tests
- **Jest**: Use Jest for all unit testing
- **Mocking**: Mock external dependencies (OrbitDB, IPFS)
- **Coverage**: Aim for >90% code coverage
- **Structure**: One test file per source file
- **Naming**: `*.test.ts` for test files

```typescript
describe('ModelName', () => {
  let model: ModelName;
  
  beforeEach(() => {
    model = new ModelName();
  });
  
  describe('methodName', () => {
    it('should handle normal case', async () => {
      // Arrange
      const input = 'test value';
      
      // Act
      const result = await model.methodName(input);
      
      // Assert
      expect(result).toBeDefined();
      expect(result.property).toBe('expected value');
    });
    
    it('should throw error for invalid input', async () => {
      // Arrange & Act & Assert
      await expect(model.methodName(null)).rejects.toThrow('Expected error message');
    });
  });
});
```

### Integration Tests
- **Docker**: Use Docker for real integration tests
- **Scenarios**: Test complete user workflows
- **Data**: Use realistic test data
- **Cleanup**: Always clean up test data

## Performance Guidelines

### Query Optimization
```typescript
// Use selective field loading when possible
const users = await User.query().select(['id', 'username']).find();

// Prefer eager loading for predictable relationships
const posts = await Post.query().with(['author', 'comments']).find();

// Use caching for expensive queries
const popularPosts = await Post.query()
  .where('likeCount', '>', 100)
  .cache(300) // Cache for 5 minutes
  .find();
```

### Memory Management
- **Lazy Loading**: Use lazy loading for large datasets
- **Pagination**: Always use pagination for large result sets
- **Cache Limits**: Set appropriate cache size limits
- **Cleanup**: Clean up resources in finally blocks

### Database Design
- **Sharding**: Design effective sharding strategies
- **Indexing**: Use indexes for frequently queried fields
- **Relationships**: Avoid deep relationship chains
- **Data Types**: Use appropriate data types for storage efficiency

## Security Considerations

### Input Validation
```typescript
// Always validate and sanitize inputs
@Field({
  type: 'string',
  required: true,
  validate: (value: string) => value.length >= 3 && value.length <= 50,
  transform: (value: string) => value.trim().toLowerCase()
})
username: string;
```

### Access Control
- **User Scoping**: Use user-scoped models for private data
- **Validation**: Validate user permissions before operations
- **Sanitization**: Sanitize all user inputs
- **Encryption**: Use encryption for sensitive data

### Error Messages
- **Security**: Don't expose internal system details in error messages
- **Logging**: Log security-relevant events
- **Validation**: Provide clear validation error messages

## Documentation Standards

### Code Documentation
```typescript
/**
 * Creates a new user with the provided data.
 * 
 * @param userData - The user data to create
 * @param options - Additional creation options
 * @returns Promise resolving to the created user
 * @throws ValidationError if user data is invalid
 * @throws DuplicateError if username or email already exists
 * 
 * @example
 * ```typescript
 * const user = await userService.createUser({
 *   username: 'alice',
 *   email: 'alice@example.com'
 * });
 * ```
 */
async createUser(userData: CreateUserData, options?: CreateOptions): Promise<User> {
  // Implementation
}
```

### README Updates
- Keep README.md up to date with latest features
- Include practical examples
- Document breaking changes
- Provide migration guides

### API Documentation
- Document all public APIs
- Include parameter types and return types
- Provide usage examples
- Document error conditions

## Common Patterns and Anti-Patterns

### ✅ Good Patterns

#### Model Definition
```typescript
@Model({
  scope: 'user',
  type: 'docstore',
  sharding: { strategy: 'hash', count: 4, key: 'userId' }
})
export class Post extends BaseModel {
  @Field({ type: 'string', required: true, maxLength: 200 })
  title: string;
  
  @Field({ type: 'string', required: true })
  content: string;
  
  @BeforeCreate()
  setDefaults() {
    this.createdAt = Date.now();
    this.updatedAt = Date.now();
  }
}
```

#### Service Methods
```typescript
async createPost(authorId: string, postData: CreatePostData): Promise<Post> {
  // Validate input
  if (!authorId) throw new ValidationError('Author ID is required');
  
  // Check permissions
  const author = await User.findById(authorId);
  if (!author) throw new NotFoundError('Author not found');
  
  // Create post
  const post = await Post.create({
    ...postData,
    authorId,
  });
  
  // Post-processing
  await this.notifyFollowers(author, post);
  
  return post;
}
```

### ❌ Anti-Patterns

#### Avoid Direct Database Access
```typescript
// Bad: Direct database manipulation
const db = await orbitdb.open('posts');
await db.put('key', data);

// Good: Use model methods
const post = await Post.create(data);
```

#### Avoid Synchronous Operations
```typescript
// Bad: Synchronous file operations
const data = fs.readFileSync('file.json');

// Good: Async operations
const data = await fs.promises.readFile('file.json');
```

#### Avoid Deep Relationship Chains
```typescript
// Bad: Deep relationship loading
const posts = await Post.query()
  .with(['author.profile.settings.preferences'])
  .find();

// Good: Load only what you need
const posts = await Post.query()
  .with(['author'])
  .find();
```

## Migration Guidelines

### Schema Changes
```typescript
// Create migration for schema changes
const migration = createMigration('add_user_avatar', '1.1.0')
  .addField('User', 'avatarUrl', {
    type: 'string',
    required: false
  })
  .transformData('User', (user) => ({
    ...user,
    avatarUrl: user.avatarUrl || null
  }))
  .build();
```

### Backwards Compatibility
- Always maintain backwards compatibility in minor versions
- Deprecate features before removing them
- Provide migration paths for breaking changes
- Document all changes in CHANGELOG.md

## Deployment Considerations

### Environment Configuration
- Use environment variables for configuration
- Provide default configurations for development
- Validate configuration on startup
- Document required environment variables

### Production Readiness
- Enable production optimizations
- Configure appropriate cache sizes
- Set up monitoring and logging
- Implement health checks

### Performance Monitoring
- Monitor query performance
- Track cache hit rates
- Monitor memory usage
- Set up alerts for errors

## Contributing Guidelines

### Pull Request Process
1. Fork the repository
2. Create feature branch from main
3. Implement changes with tests
4. Update documentation
5. Submit pull request with description

### Code Review Checklist
- [ ] Code follows style guidelines
- [ ] Tests are included and passing
- [ ] Documentation is updated
- [ ] No breaking changes (or properly documented)
- [ ] Performance implications considered
- [ ] Security implications reviewed

### Commit Message Format
```
type(scope): description

body (optional)

footer (optional)
```

Types: feat, fix, docs, style, refactor, test, chore

These guidelines ensure consistent, maintainable, and high-quality code throughout the DebrosFramework project.
