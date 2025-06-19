// Global teardown for real integration tests
module.exports = async () => {
  console.log('🧹 Global teardown for real integration tests');
  
  // Force cleanup any remaining processes
  try {
    // Kill any orphaned processes that might be hanging around
    const { exec } = require('child_process');
    const { promisify } = require('util');
    const execAsync = promisify(exec);
    
    // Clean up any leftover IPFS processes (be careful - only test processes)
    try {
      await execAsync('pkill -f "test.*ipfs" || true');
    } catch (error) {
      // Ignore errors - processes might not exist
    }
    
    // Clean up temporary directories
    const fs = require('fs');
    const path = require('path');
    const os = require('os');
    
    const tempDir = os.tmpdir();
    const testDirs = fs.readdirSync(tempDir).filter(dir => dir.startsWith('debros-test-'));
    
    for (const dir of testDirs) {
      try {
        const fullPath = path.join(tempDir, dir);
        fs.rmSync(fullPath, { recursive: true, force: true });
        console.log(`🗑️  Cleaned up: ${fullPath}`);
      } catch (error) {
        console.warn(`⚠️  Could not clean up ${dir}:`, error.message);
      }
    }
    
  } catch (error) {
    console.warn('⚠️  Error during global teardown:', error.message);
  }
  
  console.log('✅ Global teardown complete');
};