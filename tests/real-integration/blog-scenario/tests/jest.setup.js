// Global test setup
console.log('🚀 Starting Blog Integration Tests');
console.log('📡 Target nodes: blog-node-1, blog-node-2, blog-node-3');
console.log('⏰ Test timeout: 120 seconds');
console.log('=====================================');

// Increase timeout for all tests
jest.setTimeout(120000);

// Global error handler
process.on('unhandledRejection', (reason, promise) => {
  console.error('Unhandled Rejection at:', promise, 'reason:', reason);
});

// Clean up console logs for better readability
const originalLog = console.log;
console.log = (...args) => {
  const timestamp = new Date().toISOString();
  originalLog(`[${timestamp}]`, ...args);
};
