// Test setup file
import 'reflect-metadata';

// Global test configuration
jest.setTimeout(30000);

// Mock console to reduce noise during testing
global.console = {
  ...console,
  log: jest.fn(),
  debug: jest.fn(),
  info: jest.fn(),
  warn: jest.fn(),
  error: jest.fn(),
};

// Setup global test utilities
global.beforeEach(() => {
  jest.clearAllMocks();
});

// Add custom matchers if needed
expect.extend({
  toBeValidModel(received: any) {
    const pass = received && 
                 typeof received.id === 'string' && 
                 typeof received.save === 'function' &&
                 typeof received.delete === 'function';
    
    if (pass) {
      return {
        message: () => `Expected ${received} not to be a valid model`,
        pass: true,
      };
    } else {
      return {
        message: () => `Expected ${received} to be a valid model with id, save, and delete methods`,
        pass: false,
      };
    }
  },
});

// Declare custom matcher types for TypeScript
declare global {
  namespace jest {
    interface Matchers<R> {
      toBeValidModel(): R;
    }
  }
}