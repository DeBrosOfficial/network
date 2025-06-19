// Global setup for real integration tests
module.exports = async () => {
  console.log('🚀 Global setup for real integration tests');
  
  // Set environment variables
  process.env.NODE_ENV = 'test';
  process.env.DEBROS_TEST_MODE = 'real';
  
  // Check for required dependencies
  try {
    require('helia');
    require('@orbitdb/core');
    console.log('✅ Required dependencies available');
  } catch (error) {
    console.error('❌ Missing required dependencies for real tests:', error.message);
    process.exit(1);
  }
  
  // Validate environment
  const nodeVersion = process.version;
  console.log(`📋 Node.js version: ${nodeVersion}`);
  
  if (parseInt(nodeVersion.slice(1)) < 18) {
    console.error('❌ Node.js 18+ required for real tests');
    process.exit(1);
  }
  
  // Check available ports (basic check)
  const net = require('net');
  const checkPort = (port) => {
    return new Promise((resolve) => {
      const server = net.createServer();
      server.listen(port, () => {
        server.close(() => resolve(true));
      });
      server.on('error', () => resolve(false));
    });
  };
  
  const basePort = 40000;
  const portAvailable = await checkPort(basePort);
  if (!portAvailable) {
    console.warn(`⚠️  Port ${basePort} not available, tests will use dynamic ports`);
  }
  
  console.log('✅ Global setup complete');
};