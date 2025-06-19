// Simple debug script to test field defaults
const { execSync } = require('child_process');

// Run a small test using jest directly
const testCode = `
import { BaseModel } from './src/framework/models/BaseModel';
import { Model, Field } from './src/framework/models/decorators';

@Model({
  scope: 'global',
  type: 'docstore'
})
class TestUser extends BaseModel {
  @Field({ type: 'string', required: true })
  username: string;

  @Field({ type: 'number', required: false, default: 0 })
  score: number;

  @Field({ type: 'boolean', required: false, default: true })
  isActive: boolean;
}

// Debug the fields
console.log('TestUser.fields:', TestUser.fields);
console.log('TestUser.fields size:', TestUser.fields?.size);

if (TestUser.fields) {
  for (const [fieldName, fieldConfig] of TestUser.fields) {
    console.log(\`Field: \${fieldName}, Config:\`, fieldConfig);
  }
}

// Test instance creation
const user = new TestUser();
console.log('User instance score:', user.score);
console.log('User instance isActive:', user.isActive);

// Check private fields
console.log('User _score:', (user as any)._score);
console.log('User _isActive:', (user as any)._isActive);
`;

console.log('Test code created for debugging...');