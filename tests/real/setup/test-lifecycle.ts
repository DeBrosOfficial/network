import { RealIPFSService, createIPFSNetwork, shutdownIPFSNetwork } from './ipfs-setup';
import { RealOrbitDBService, createOrbitDBNetwork, shutdownOrbitDBNetwork } from './orbitdb-setup';
import { PrivateSwarmSetup, waitForNetworkReady } from './swarm-setup';

export interface RealTestNetwork {
  ipfsNodes: RealIPFSService[];
  orbitdbNodes: RealOrbitDBService[];
  swarmSetup: PrivateSwarmSetup;
}

export interface RealTestConfig {
  nodeCount: number;
  timeout: number;
  enableDebugLogs: boolean;
}

export class RealTestManager {
  private network: RealTestNetwork | null = null;
  private config: RealTestConfig;

  constructor(config: Partial<RealTestConfig> = {}) {
    this.config = {
      nodeCount: 3,
      timeout: 60000, // 60 seconds
      enableDebugLogs: false,
      ...config
    };
  }

  async setup(): Promise<RealTestNetwork> {
    console.log(`🚀 Setting up real test network with ${this.config.nodeCount} nodes...`);
    
    try {
      // Create IPFS network
      const { nodes: ipfsNodes, swarmSetup } = await createIPFSNetwork(this.config.nodeCount);

      // Wait for network to be ready
      const networkReady = await waitForNetworkReady(ipfsNodes.map(n => n.getHelia()), this.config.timeout);
      if (!networkReady) {
        throw new Error('Network failed to become ready within timeout');
      }

      // Create OrbitDB network
      const orbitdbNodes = await createOrbitDBNetwork(ipfsNodes);

      this.network = {
        ipfsNodes,
        orbitdbNodes,
        swarmSetup
      };

      console.log(`✅ Real test network setup complete`);
      this.logNetworkStatus();

      return this.network;
    } catch (error) {
      console.error(`❌ Failed to setup real test network:`, error);
      await this.cleanup();
      throw error;
    }
  }

  async cleanup(): Promise<void> {
    if (!this.network) {
      return;
    }

    console.log(`🧹 Cleaning up real test network...`);

    try {
      // Shutdown OrbitDB network first
      await shutdownOrbitDBNetwork(this.network.orbitdbNodes);
      
      // Shutdown IPFS network
      await shutdownIPFSNetwork(this.network.ipfsNodes, this.network.swarmSetup);

      this.network = null;
      console.log(`✅ Real test network cleanup complete`);
    } catch (error) {
      console.error(`❌ Error during cleanup:`, error);
      // Continue with cleanup even if there are errors
    }
  }

  getNetwork(): RealTestNetwork {
    if (!this.network) {
      throw new Error('Network not initialized. Call setup() first.');
    }
    return this.network;
  }

  // Get a single node for simple tests
  getPrimaryNode(): { ipfs: RealIPFSService; orbitdb: RealOrbitDBService } {
    const network = this.getNetwork();
    return {
      ipfs: network.ipfsNodes[0],
      orbitdb: network.orbitdbNodes[0]
    };
  }

  // Get multiple nodes for P2P tests
  getMultipleNodes(count?: number): Array<{ ipfs: RealIPFSService; orbitdb: RealOrbitDBService }> {
    const network = this.getNetwork();
    const nodeCount = count || network.ipfsNodes.length;
    
    return Array.from({ length: Math.min(nodeCount, network.ipfsNodes.length) }, (_, i) => ({
      ipfs: network.ipfsNodes[i],
      orbitdb: network.orbitdbNodes[i]
    }));
  }

  private logNetworkStatus(): void {
    if (!this.network || !this.config.enableDebugLogs) {
      return;
    }

    console.log(`📊 Network Status:`);
    console.log(`  Nodes: ${this.network.ipfsNodes.length}`);
    
    for (let i = 0; i < this.network.ipfsNodes.length; i++) {
      const ipfsNode = this.network.ipfsNodes[i];
      const peers = ipfsNode.getConnectedPeers();
      console.log(`  Node ${i}:`);
      console.log(`    Peer ID: ${ipfsNode.getPeerId()}`);
      console.log(`    Connected Peers: ${peers.length}`);
      console.log(`    Addresses: ${ipfsNode.getMultiaddrs().join(', ')}`);
    }
  }

  // Test utilities
  async waitForNetworkStabilization(timeout: number = 10000): Promise<void> {
    console.log(`⏳ Waiting for network stabilization...`);
    
    // Wait for connections to stabilize
    await new Promise(resolve => setTimeout(resolve, timeout));
    
    if (this.config.enableDebugLogs) {
      this.logNetworkStatus();
    }
  }

  async verifyNetworkConnectivity(): Promise<boolean> {
    const network = this.getNetwork();
    
    // Check if all nodes have at least one connection
    for (const node of network.ipfsNodes) {
      const peers = node.getConnectedPeers();
      if (peers.length === 0) {
        console.log(`❌ Node ${node.nodeIndex} has no peer connections`);
        return false;
      }
    }
    
    console.log(`✅ All nodes have peer connections`);
    return true;
  }
}

// Global test manager for Jest lifecycle
let globalTestManager: RealTestManager | null = null;

export async function setupGlobalTestNetwork(config: Partial<RealTestConfig> = {}): Promise<RealTestNetwork> {
  if (globalTestManager) {
    throw new Error('Global test network already setup. Call cleanupGlobalTestNetwork() first.');
  }

  globalTestManager = new RealTestManager(config);
  return await globalTestManager.setup();
}

export async function cleanupGlobalTestNetwork(): Promise<void> {
  if (globalTestManager) {
    await globalTestManager.cleanup();
    globalTestManager = null;
  }
}

export function getGlobalTestNetwork(): RealTestNetwork {
  if (!globalTestManager) {
    throw new Error('Global test network not setup. Call setupGlobalTestNetwork() first.');
  }
  return globalTestManager.getNetwork();
}

export function getGlobalTestManager(): RealTestManager {
  if (!globalTestManager) {
    throw new Error('Global test manager not setup. Call setupGlobalTestNetwork() first.');
  }
  return globalTestManager;
}

// Jest helper functions
export const realTestHelpers = {
  setupAll: setupGlobalTestNetwork,
  cleanupAll: cleanupGlobalTestNetwork,
  getNetwork: getGlobalTestNetwork,
  getManager: getGlobalTestManager
};