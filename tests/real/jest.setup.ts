// Jest setup for real integration tests
import { jest } from '@jest/globals';

// Increase timeout for all tests
jest.setTimeout(180000); // 3 minutes

// Disable console logs in tests unless in debug mode
const originalConsole = console;
const debugMode = process.env.REAL_TEST_DEBUG === 'true';

if (!debugMode) {
  // Silence routine logs but keep errors and important messages
  console.log = (...args: any[]) => {
    const message = args.join(' ');
    if (message.includes('❌') || message.includes('✅') || message.includes('🚀') || message.includes('🧹')) {
      originalConsole.log(...args);
    }
  };
  
  console.info = () => {}; // Silence info
  console.debug = () => {}; // Silence debug
  
  // Keep warnings and errors
  console.warn = originalConsole.warn;
  console.error = originalConsole.error;
}

// Global error handlers
process.on('unhandledRejection', (reason, promise) => {
  console.error('❌ Unhandled Rejection at:', promise, 'reason:', reason);
});

process.on('uncaughtException', (error) => {
  console.error('❌ Uncaught Exception:', error);
});

// Environment setup
process.env.NODE_ENV = 'test';
process.env.DEBROS_TEST_MODE = 'real';

// Global test utilities
declare global {
  namespace NodeJS {
    interface Global {
      REAL_TEST_CONFIG: {
        timeout: number;
        nodeCount: number;
        debugMode: boolean;
      };
    }
  }
}

(global as any).REAL_TEST_CONFIG = {
  timeout: 180000,
  nodeCount: parseInt(process.env.REAL_TEST_NODE_COUNT || '3'),
  debugMode: debugMode
};

console.log('🔧 Real test environment configured');
console.log(`  Debug mode: ${debugMode}`);
console.log(`  Node count: ${(global as any).REAL_TEST_CONFIG.nodeCount}`);
console.log(`  Timeout: ${(global as any).REAL_TEST_CONFIG.timeout}ms`);