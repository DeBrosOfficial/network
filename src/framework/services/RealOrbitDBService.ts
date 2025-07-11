import { createOrbitDB } from '@orbitdb/core';

export class OrbitDBService {
  private orbitdb: any;
  private ipfsService: any;

  constructor(ipfsService: any) {
    this.ipfsService = ipfsService;
  }

  async init(): Promise<void> {
    if (!this.ipfsService) {
      throw new Error('IPFS service is required for OrbitDB');
    }

    this.orbitdb = await createOrbitDB({
      ipfs: this.ipfsService.getHelia(),
      directory: './orbitdb'
    });

    console.log('OrbitDB Service initialized');
  }

  async stop(): Promise<void> {
    if (this.orbitdb) {
      await this.orbitdb.stop();
    }
  }

  async openDB(name: string, type: string): Promise<any> {
    if (!this.orbitdb) {
      throw new Error('OrbitDB not initialized');
    }

    // Map framework types to OrbitDB v2 types
    const orbitDBType = this.mapFrameworkTypeToOrbitDB(type);

    return await this.orbitdb.open(name, { 
      type: orbitDBType,
      AccessController: this.orbitdb.AccessController
    });
  }

  private mapFrameworkTypeToOrbitDB(frameworkType: string): string {
    const typeMapping: { [key: string]: string } = {
      'docstore': 'documents',
      'keyvalue': 'keyvalue',
      'eventlog': 'eventlog'
    };

    return typeMapping[frameworkType] || frameworkType;
  }

  getOrbitDB(): any {
    return this.orbitdb;
  }
}