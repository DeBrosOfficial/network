// Global setup for real integration tests
module.exports = async () => {
  console.log('🚀 Global setup for real integration tests');
  
  // Set environment variables
  process.env.NODE_ENV = 'test';
  process.env.DEBROS_TEST_MODE = 'real';
  
  // Check for required dependencies - skip for ES module packages
  try {
    // Just check if the packages exist without importing them
    const fs = require('fs');
    const path = require('path');
    
    const heliaPath = path.join(__dirname, '../../node_modules/helia');
    const orbitdbPath = path.join(__dirname, '../../node_modules/@orbitdb/core');
    
    if (fs.existsSync(heliaPath) && fs.existsSync(orbitdbPath)) {
      console.log('✅ Required dependencies available');
    } else {
      throw new Error('Required packages not found');
    }
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