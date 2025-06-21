import { spawn, ChildProcess } from 'child_process';
import { ApiClient } from '../utils/ApiClient';
import { SyncWaiter } from '../utils/SyncWaiter';

export interface NodeConfig {
  nodeId: string;
  apiPort: number;
  ipfsPort: number;
  nodeType: string;
}

export interface DockerComposeConfig {
  composeFile: string;
  scenario: string;
  nodes: NodeConfig[];
}

export class DockerNodeManager {
  private process: ChildProcess | null = null;
  private apiClients: ApiClient[] = [];
  private syncWaiter: SyncWaiter;

  constructor(private config: DockerComposeConfig) {
    // Create API clients for each node
    this.apiClients = this.config.nodes.map(node => 
      new ApiClient(`http://localhost:${node.apiPort}`)
    );
    
    this.syncWaiter = new SyncWaiter(this.apiClients);
  }

  async startCluster(): Promise<boolean> {
    console.log(`🚀 Starting ${this.config.scenario} cluster...`);
    
    try {
      // Start docker-compose
      this.process = spawn('docker-compose', [
        '-f', this.config.composeFile,
        'up',
        '--build',
        '--force-recreate'
      ], {
        stdio: 'pipe',
        cwd: process.cwd()
      });

      // Log output
      this.process.stdout?.on('data', (data) => {
        console.log(`[DOCKER] ${data.toString().trim()}`);
      });

      this.process.stderr?.on('data', (data) => {
        console.error(`[DOCKER ERROR] ${data.toString().trim()}`);
      });

      // Wait for nodes to be ready
      const ready = await this.syncWaiter.waitForNodesReady(120000);
      if (!ready) {
        throw new Error('Nodes failed to become ready');
      }

      // Wait for peer connections
      const connected = await this.syncWaiter.waitForPeerConnections(
        this.config.nodes.length - 1, // Each node should connect to all others
        60000
      );
      
      if (!connected) {
        throw new Error('Nodes failed to establish peer connections');
      }

      console.log(`✅ ${this.config.scenario} cluster started successfully`);
      return true;

    } catch (error) {
      console.error(`❌ Failed to start cluster: ${error.message}`);
      await this.stopCluster();
      return false;
    }
  }

  async stopCluster(): Promise<void> {
    console.log(`🛑 Stopping ${this.config.scenario} cluster...`);
    
    try {
      if (this.process) {
        this.process.kill('SIGTERM');
        
        // Wait for graceful shutdown
        await new Promise((resolve) => {
          this.process?.on('exit', resolve);
          setTimeout(resolve, 10000); // Force kill after 10s
        });
      }

      // Clean up docker containers and volumes
      const cleanup = spawn('docker-compose', [
        '-f', this.config.composeFile,
        'down',
        '-v',
        '--remove-orphans'
      ], {
        stdio: 'inherit',
        cwd: process.cwd()
      });

      await new Promise((resolve) => {
        cleanup.on('exit', resolve);
      });

      console.log(`✅ ${this.config.scenario} cluster stopped`);

    } catch (error) {
      console.error(`❌ Error stopping cluster: ${error.message}`);
    }
  }

  getApiClient(nodeIndex: number): ApiClient {
    if (nodeIndex >= this.apiClients.length) {
      throw new Error(`Node index ${nodeIndex} is out of range`);
    }
    return this.apiClients[nodeIndex];
  }

  getSyncWaiter(): SyncWaiter {
    return this.syncWaiter;
  }

  async waitForSync(timeout: number = 10000): Promise<void> {
    await this.syncWaiter.waitForSync(timeout);
  }

  async getNetworkMetrics(): Promise<any> {
    const metrics = await this.syncWaiter.getSyncMetrics();
    
    return {
      totalNodes: this.config.nodes.length,
      readyNodes: metrics.length,
      averagePeers: metrics.length > 0 
        ? metrics.reduce((sum, m) => sum + m.peerCount, 0) / metrics.length 
        : 0,
      nodeMetrics: metrics
    };
  }

  async logClusterStatus(): Promise<void> {
    console.log(`\n📋 ${this.config.scenario} Cluster Status:`);
    console.log(`Nodes: ${this.config.nodes.length}`);
    
    const networkMetrics = await this.getNetworkMetrics();
    console.log(`Ready: ${networkMetrics.readyNodes}/${networkMetrics.totalNodes}`);
    console.log(`Average Peers: ${networkMetrics.averagePeers.toFixed(1)}`);
    
    await this.syncWaiter.logSyncStatus();
  }

  async healthCheck(): Promise<boolean> {
    try {
      const results = await Promise.all(
        this.apiClients.map(client => client.health())
      );
      
      return results.every(result => 
        result.status === 200 && result.data?.status === 'healthy'
      );
    } catch (error) {
      return false;
    }
  }
}