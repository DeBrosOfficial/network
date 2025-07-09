module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  roots: ['<rootDir>/tests/real-integration'],
  testMatch: ['**/tests/**/*.test.ts'],
  transform: {
    '^.+\\.ts$': [
      'ts-jest',
      {
        isolatedModules: true,
      },
    ],
  },
  testTimeout: 120000, // 2 minutes for integration tests
  verbose: true,
  setupFilesAfterEnv: ['<rootDir>/tests/real-integration/blog-scenario/tests/setup.ts'],
  maxWorkers: 1, // Run tests sequentially for integration tests
  collectCoverage: false, // Skip coverage for integration tests
};
