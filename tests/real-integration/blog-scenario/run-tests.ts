#!/usr/bin/env node

import { spawn, ChildProcess } from 'child_process';
import path from 'path';
import fs from 'fs';

interface TestConfig {
  scenario: string;
  composeFile: string;
  testCommand: string;
  timeout: number;
}

class BlogIntegrationTestRunner {
  private dockerProcess: ChildProcess | null = null;
  private isShuttingDown = false;

  constructor(private config: TestConfig) {
    // Handle graceful shutdown
    process.on('SIGINT', () => this.shutdown());
    process.on('SIGTERM', () => this.shutdown());
    process.on('exit', () => this.shutdown());
  }

  async run(): Promise<boolean> {
    console.log(`🚀 Starting ${this.config.scenario} integration tests...`);
    console.log(`Using compose file: ${this.config.composeFile}`);
    
    try {
      // Verify compose file exists
      if (!fs.existsSync(this.config.composeFile)) {
        throw new Error(`Docker compose file not found: ${this.config.composeFile}`);
      }

      // Clean up any existing containers
      await this.cleanup();

      // Start Docker services
      const success = await this.startServices();
      if (!success) {
        throw new Error('Failed to start Docker services');
      }

      // Wait for services to be healthy
      const healthy = await this.waitForHealthy();
      if (!healthy) {
        throw new Error('Services failed to become healthy');
      }

      // Run tests
      const testResult = await this.runTests();
      
      // Cleanup
      await this.cleanup();
      
      return testResult;

    } catch (error) {
      console.error('❌ Test execution failed:', error.message);
      await this.cleanup();
      return false;
    }
  }

  private async startServices(): Promise<boolean> {
    console.log('🔧 Starting Docker services...');
    
    return new Promise((resolve) => {
      this.dockerProcess = spawn('docker-compose', [
        '-f', this.config.composeFile,
        'up',
        '--build',
        '--abort-on-container-exit'
      ], {
        stdio: 'pipe',
        cwd: path.dirname(this.config.composeFile)
      });

      let servicesStarted = false;
      let testRunnerFinished = false;

      this.dockerProcess.stdout?.on('data', (data) => {
        const output = data.toString();
        console.log('[DOCKER]', output.trim());
        
        // Check if all services are up
        if (output.includes('blog-node-3') && output.includes('healthy')) {
          servicesStarted = true;
        }
        
        // Check if test runner has finished
        if (output.includes('blog-test-runner') && (output.includes('exited') || output.includes('done'))) {
          testRunnerFinished = true;
        }
      });

      this.dockerProcess.stderr?.on('data', (data) => {
        console.error('[DOCKER ERROR]', data.toString().trim());
      });

      this.dockerProcess.on('exit', (code) => {
        console.log(`Docker process exited with code: ${code}`);
        resolve(code === 0 && testRunnerFinished);
      });

      // Timeout after specified time
      setTimeout(() => {
        if (!testRunnerFinished) {
          console.log('❌ Test execution timed out');
          resolve(false);
        }
      }, this.config.timeout);
    });
  }

  private async waitForHealthy(): Promise<boolean> {
    console.log('🔧 Waiting for services to be healthy...');
    
    // Wait for health checks to pass
    for (let attempt = 0; attempt < 30; attempt++) {
      try {
        const result = await this.checkHealth();
        if (result) {
          console.log('✅ All services are healthy');
          return true;
        }
      } catch (error) {
        // Continue waiting
      }
      
      await new Promise(resolve => setTimeout(resolve, 5000));
    }
    
    console.log('❌ Services failed to become healthy within timeout');
    return false;
  }

  private async checkHealth(): Promise<boolean> {
    return new Promise((resolve) => {
      const healthCheck = spawn('docker-compose', [
        '-f', this.config.composeFile,
        'ps'
      ], {
        stdio: 'pipe',
        cwd: path.dirname(this.config.composeFile)
      });

      let output = '';
      healthCheck.stdout?.on('data', (data) => {
        output += data.toString();
      });

      healthCheck.on('exit', () => {
        // Check if all required services are healthy
        const requiredServices = ['blog-node-1', 'blog-node-2', 'blog-node-3'];
        const allHealthy = requiredServices.every(service => 
          output.includes(service) && output.includes('Up') && output.includes('healthy')
        );
        
        resolve(allHealthy);
      });
    });
  }

  private async runTests(): Promise<boolean> {
    console.log('🧪 Running integration tests...');
    
    // Tests are run as part of the Docker composition
    // We just need to wait for the test runner container to complete
    return true;
  }

  private async cleanup(): Promise<void> {
    if (this.isShuttingDown) return;
    this.isShuttingDown = true;
    
    console.log('🧹 Cleaning up Docker resources...');
    
    try {
      // Stop and remove containers
      const cleanup = spawn('docker-compose', [
        '-f', this.config.composeFile,
        'down',
        '-v',
        '--remove-orphans'
      ], {
        stdio: 'inherit',
        cwd: path.dirname(this.config.composeFile)
      });

      await new Promise((resolve) => {
        cleanup.on('exit', resolve);
        setTimeout(resolve, 10000); // Force cleanup after 10s
      });

      console.log('✅ Cleanup completed');
    } catch (error) {
      console.warn('⚠️ Cleanup warning:', error.message);
    }
  }

  private async shutdown(): Promise<void> {
    console.log('\n🛑 Shutting down...');
    
    if (this.dockerProcess && !this.dockerProcess.killed) {
      this.dockerProcess.kill('SIGTERM');
    }
    
    await this.cleanup();
    process.exit(0);
  }
}

// Main execution
async function main() {
  const config: TestConfig = {
    scenario: 'blog',
    composeFile: path.join(__dirname, 'docker', 'docker-compose.blog.yml'),
    testCommand: 'npm run test:blog-integration',
    timeout: 600000 // 10 minutes
  };

  const runner = new BlogIntegrationTestRunner(config);
  const success = await runner.run();
  
  if (success) {
    console.log('🎉 Blog integration tests completed successfully!');
    process.exit(0);
  } else {
    console.log('❌ Blog integration tests failed!');
    process.exit(1);
  }
}

// Run if called directly
if (require.main === module) {
  main().catch((error) => {
    console.error('💥 Unexpected error:', error);
    process.exit(1);
  });
}

export { BlogIntegrationTestRunner };