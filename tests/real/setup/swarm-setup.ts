import { randomBytes } from 'crypto';
import { writeFileSync, mkdirSync, rmSync, existsSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';

export interface SwarmConfig {
  swarmKey: string;
  nodeCount: number;
  basePort: number;
  dataDir: string;
  bootstrapAddrs: string[];
}

export class PrivateSwarmSetup {
  private config: SwarmConfig;
  private swarmKeyPath: string;

  constructor(nodeCount: number = 3) {
    const testId = Date.now().toString(36);
    const basePort = 40000 + Math.floor(Math.random() * 10000);
    
    this.config = {
      swarmKey: this.generateSwarmKey(),
      nodeCount,
      basePort,
      dataDir: join(tmpdir(), `debros-test-${testId}`),
      bootstrapAddrs: []
    };

    this.swarmKeyPath = join(this.config.dataDir, 'swarm.key');
    this.setupSwarmKey();
    this.generateBootstrapAddrs();
  }

  private generateSwarmKey(): string {
    // Generate a private swarm key (64 bytes of random data)
    const key = randomBytes(32).toString('hex');
    return `/key/swarm/psk/1.0.0/\n/base16/\n${key}`;
  }

  private setupSwarmKey(): void {
    // Create data directory
    mkdirSync(this.config.dataDir, { recursive: true });
    
    // Write swarm key file
    writeFileSync(this.swarmKeyPath, this.config.swarmKey);
  }

  private generateBootstrapAddrs(): void {
    // Generate bootstrap addresses for private network
    // First node will be the bootstrap node
    const bootstrapPort = this.config.basePort;
    this.config.bootstrapAddrs = [
      `/ip4/127.0.0.1/tcp/${bootstrapPort}/p2p/12D3KooWBootstrapNodeId` // Placeholder - will be replaced with actual peer ID
    ];
  }

  getConfig(): SwarmConfig {
    return { ...this.config };
  }

  getNodeDataDir(nodeIndex: number): string {
    const nodeDir = join(this.config.dataDir, `node-${nodeIndex}`);
    mkdirSync(nodeDir, { recursive: true });
    return nodeDir;
  }

  getNodePort(nodeIndex: number): number {
    return this.config.basePort + nodeIndex;
  }

  getSwarmKeyPath(): string {
    return this.swarmKeyPath;
  }

  cleanup(): void {
    try {
      if (existsSync(this.config.dataDir)) {
        rmSync(this.config.dataDir, { recursive: true, force: true });
        console.log(`🧹 Cleaned up test data directory: ${this.config.dataDir}`);
      }
    } catch (error) {
      console.warn(`Warning: Could not cleanup test directory: ${error}`);
    }
  }

  // Get libp2p configuration for a node
  getLibp2pConfig(nodeIndex: number, isBootstrap: boolean = false) {
    const port = this.getNodePort(nodeIndex);
    
    return {
      addresses: {
        listen: [`/ip4/127.0.0.1/tcp/${port}`]
      },
      connectionManager: {
        minConnections: 1,
        maxConnections: 10,
        dialTimeout: 30000
      },
      // For private networks, we'll configure bootstrap after peer IDs are known
      bootstrap: isBootstrap ? [] : [], // Will be populated with actual bootstrap addresses
      datastore: undefined, // Will be set by the node setup
      keychain: {
        pass: 'test-passphrase'
      }
    };
  }
}

// Test utilities
export async function waitForPeerConnections(
  nodes: any[], 
  expectedConnections: number, 
  timeout: number = 30000
): Promise<boolean> {
  const startTime = Date.now();
  
  while (Date.now() - startTime < timeout) {
    let allConnected = true;
    
    for (const node of nodes) {
      const peers = node.libp2p.getPeers();
      if (peers.length < expectedConnections) {
        allConnected = false;
        break;
      }
    }
    
    if (allConnected) {
      console.log(`✅ All nodes connected with ${expectedConnections} peers each`);
      return true;
    }
    
    // Wait 100ms before checking again
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  
  console.log(`⚠️ Timeout waiting for peer connections after ${timeout}ms`);
  return false;
}

export async function waitForNetworkReady(nodes: any[], timeout: number = 30000): Promise<boolean> {
  // Wait for at least one connection between any nodes
  const startTime = Date.now();
  
  while (Date.now() - startTime < timeout) {
    let hasConnections = false;
    
    for (const node of nodes) {
      const peers = node.libp2p.getPeers();
      if (peers.length > 0) {
        hasConnections = true;
        break;
      }
    }
    
    if (hasConnections) {
      console.log(`🌐 Private network is ready with ${nodes.length} nodes`);
      return true;
    }
    
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  
  console.log(`⚠️ Timeout waiting for network to be ready after ${timeout}ms`);
  return false;
}