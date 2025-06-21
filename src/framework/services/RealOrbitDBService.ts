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

    return await this.orbitdb.open(name, { 
      type,
      AccessController: this.orbitdb.AccessController
    });
  }

  getOrbitDB(): any {
    return this.orbitdb;
  }
}