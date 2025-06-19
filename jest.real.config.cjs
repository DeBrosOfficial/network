module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  roots: ['<rootDir>/tests/real'],
  testMatch: ['**/real/**/*.test.ts'],
  transform: {
    '^.+\\.ts$': [
      'ts-jest',
      {
        isolatedModules: true,
      },
    ],
  },
  collectCoverageFrom: ['src/**/*.ts', '!src/**/*.d.ts', '!src/**/index.ts', '!src/examples/**'],
  coverageDirectory: 'coverage-real',
  coverageReporters: ['text', 'lcov', 'html'],
  
  // Extended timeouts for real network operations
  testTimeout: 180000, // 3 minutes per test
  
  // Run tests serially to avoid port conflicts and resource contention
  maxWorkers: 1,
  
  // Setup and teardown
  globalSetup: '<rootDir>/tests/real/jest.global-setup.cjs',
  globalTeardown: '<rootDir>/tests/real/jest.global-teardown.cjs',
  
  // Environment variables for real tests
  setupFilesAfterEnv: ['<rootDir>/tests/real/jest.setup.ts'],
  
  // Longer timeout for setup/teardown
  setupFilesTimeout: 120000,
  
  // Disable watch mode (real tests are too slow)
  watchman: false,
  
  // Clear mocks between tests
  clearMocks: true,
  restoreMocks: true,
  
  // Verbose output for debugging
  verbose: true,
  
  // Fail fast on first error (saves time with slow tests)
  bail: 1,
  
  // Module path mapping
  moduleNameMapping: {
    '^@/(.*)$': '<rootDir>/src/$1',
    '^@tests/(.*)$': '<rootDir>/tests/$1'
  }
};