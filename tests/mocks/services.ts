// Mock services factory for testing
import { MockIPFSService, MockOrbitDBService } from './ipfs';

export function createMockServices() {
  const ipfsService = new MockIPFSService();
  const orbitDBService = new MockOrbitDBService();

  return {
    ipfsService,
    orbitDBService,
    async initialize() {
      await ipfsService.init();
      await orbitDBService.init();
    },
    async cleanup() {
      await ipfsService.stop();
      await orbitDBService.stop();
    }
  };
}

// Test utilities
export function createMockDatabase() {
  const { MockDatabase } = require('./orbitdb');
  return new MockDatabase('test-db', { type: 'docstore' });
}

export function createMockRecord(overrides: any = {}) {
  return {
    id: `test-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
    createdAt: Date.now(),
    updatedAt: Date.now(),
    ...overrides
  };
}