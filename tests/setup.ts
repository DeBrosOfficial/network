// Test setup file
import 'reflect-metadata';

// Global test configuration
jest.setTimeout(30000);

// Setup global test utilities
global.beforeEach(() => {
  jest.clearAllMocks();
});
