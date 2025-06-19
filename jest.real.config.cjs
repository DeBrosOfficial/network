module.exports = {
  preset: 'ts-jest/presets/default-esm',
  testEnvironment: 'node',
  roots: ['<rootDir>/tests/real'],
  testMatch: ['**/real/**/*.test.ts'],
  
  // ES Module configuration
  extensionsToTreatAsEsm: ['.ts'],
  
  transform: {
    '^.+\\.ts$': [
      'ts-jest',
      {
        useESM: true,
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

  // Disable watch mode (real tests are too slow)
  watchman: false,

  // Clear mocks between tests
  clearMocks: true,
  restoreMocks: true,

  // Verbose output for debugging
  verbose: true,

  // Fail fast on first error (saves time with slow tests)
  bail: 1,

  // ES Module support
  extensionsToTreatAsEsm: ['.ts'],

  // Transform ES modules - more comprehensive pattern
  transformIgnorePatterns: [
    'node_modules/(?!(helia|@helia|@orbitdb|@libp2p|@chainsafe|@multiformats|multiformats|datastore-fs|blockstore-fs|libp2p)/)',
  ],

  // Module resolution for ES modules
  resolver: undefined,
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx', 'json', 'node'],  // Module name mapping to handle ES modules
  moduleNameMapper: {
    '^(\\.{1,2}/.*)\\.js$': '$1',
  },
};
