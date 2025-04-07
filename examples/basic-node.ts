import { init as initIpfs, stop as stopIpfs } from '../src/ipfs/ipfsService';
import { init as initOrbitDB } from '../src/orbit/orbitDBService';
import { createServiceLogger } from '../src/utils/logger';

const appLogger = createServiceLogger('APP');

async function startNode() {
  try {
    appLogger.info('Starting Debros node...');

    // Initialize IPFS
    const ipfs = await initIpfs();
    appLogger.info('IPFS node initialized');

    // Initialize OrbitDB
    const orbitdb = await initOrbitDB();
    appLogger.info('OrbitDB initialized');

    // Create a test database
    const db = await orbitdb.open('test-db', {
      type: 'feed',
      overwrite: false,
    });
    appLogger.info(`Database opened: ${db.address.toString()}`);

    // Add some data
    const hash = await db.add('Hello from Debros Network!');
    appLogger.info(`Added entry: ${hash}`);

    // Query data
    const allEntries = db.iterator({ limit: 10 }).collect();
    appLogger.info('Database entries:', allEntries);

    // Keep the process running
    appLogger.info('Node is running. Press Ctrl+C to stop.');

    // Handle shutdown
    process.on('SIGINT', async () => {
      appLogger.info('Shutting down...');
      await orbitdb.stop();
      await stopIpfs();
      process.exit(0);
    });
  } catch (error) {
    appLogger.error('Failed to start node:', error);
  }
}

startNode();
