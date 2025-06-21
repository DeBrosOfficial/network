import { ApiClient } from './ApiClient';

export interface SyncMetrics {
  nodeId: string;
  peerCount: number;
  dataCount: {
    users: number;
    posts: number;
    comments: number;
    categories: number;
  };
}

export class SyncWaiter {
  constructor(private apiClients: ApiClient[]) {}

  async waitForSync(timeout: number = 10000): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, timeout));
  }

  async waitForPeerConnections(minPeers: number = 2, timeout: number = 30000): Promise<boolean> {
    const startTime = Date.now();
    
    console.log(`Waiting for nodes to connect to at least ${minPeers} peers...`);
    
    while (Date.now() - startTime < timeout) {
      let allConnected = true;
      
      for (const client of this.apiClients) {
        try {
          const health = await client.health();
          if (!health.data || health.data.peers < minPeers) {
            allConnected = false;
            break;
          }
        } catch (error) {
          allConnected = false;
          break;
        }
      }
      
      if (allConnected) {
        console.log('✅ All nodes have sufficient peer connections');
        return true;
      }
      
      await new Promise(resolve => setTimeout(resolve, 2000));
    }
    
    console.log('❌ Timeout waiting for peer connections');
    return false;
  }

  async waitForDataConsistency(
    dataType: 'users' | 'posts' | 'comments' | 'categories',
    expectedCount: number,
    timeout: number = 15000,
    tolerance: number = 0
  ): Promise<boolean> {
    const startTime = Date.now();
    
    console.log(`Waiting for ${dataType} count to reach ${expectedCount} across all nodes...`);
    
    while (Date.now() - startTime < timeout) {
      let isConsistent = true;
      const counts: number[] = [];
      
      for (const client of this.apiClients) {
        try {
          const response = await client.get('/api/metrics/data');
          if (response.data && response.data.counts) {
            const count = response.data.counts[dataType];
            counts.push(count);
            
            if (Math.abs(count - expectedCount) > tolerance) {
              isConsistent = false;
            }
          } else {
            isConsistent = false;
            break;
          }
        } catch (error) {
          isConsistent = false;
          break;
        }
      }
      
      if (isConsistent) {
        console.log(`✅ Data consistency achieved: ${dataType} = ${expectedCount} across all nodes`);
        return true;
      }
      
      console.log(`Data counts: ${counts.join(', ')}, expected: ${expectedCount}`);
      await new Promise(resolve => setTimeout(resolve, 2000));
    }
    
    console.log(`❌ Timeout waiting for data consistency: ${dataType}`);
    return false;
  }

  async getSyncMetrics(): Promise<SyncMetrics[]> {
    const metrics: SyncMetrics[] = [];
    
    for (const client of this.apiClients) {
      try {
        const [healthResponse, dataResponse] = await Promise.all([
          client.health(),
          client.get('/api/metrics/data')
        ]);
        
        if (healthResponse.data && dataResponse.data) {
          metrics.push({
            nodeId: healthResponse.data.nodeId,
            peerCount: healthResponse.data.peers,
            dataCount: dataResponse.data.counts
          });
        }
      } catch (error) {
        console.warn(`Failed to get metrics from node: ${error.message}`);
      }
    }
    
    return metrics;
  }

  async logSyncStatus(): Promise<void> {
    console.log('\n📊 Current Sync Status:');
    const metrics = await this.getSyncMetrics();
    
    metrics.forEach(metric => {
      console.log(`Node: ${metric.nodeId}`);
      console.log(`  Peers: ${metric.peerCount}`);
      console.log(`  Data: Users=${metric.dataCount.users}, Posts=${metric.dataCount.posts}, Comments=${metric.dataCount.comments}, Categories=${metric.dataCount.categories}`);
    });
    console.log('');
  }

  async waitForNodesReady(timeout: number = 60000): Promise<boolean> {
    const startTime = Date.now();
    
    console.log('Waiting for all nodes to be ready...');
    
    while (Date.now() - startTime < timeout) {
      let allReady = true;
      
      for (let i = 0; i < this.apiClients.length; i++) {
        const isReady = await this.apiClients[i].waitForHealth(5000);
        if (!isReady) {
          console.log(`Node ${i} not ready yet...`);
          allReady = false;
          break;
        }
      }
      
      if (allReady) {
        console.log('✅ All nodes are ready');
        return true;
      }
      
      await new Promise(resolve => setTimeout(resolve, 3000));
    }
    
    console.log('❌ Timeout waiting for nodes to be ready');
    return false;
  }
}