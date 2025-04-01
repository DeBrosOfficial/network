import { initIpfs, initOrbitDB, logger, createServiceLogger } from '../src/index';

const appLogger = createServiceLogger('APP');

async function startNode() {
  try {
    appLogger.info('Starting Debros node...');

    // Initialize IPFS
    const ipfs = await initIpfs();
    appLogger.info('IPFS node initialized');

    // Initialize OrbitDB
    const ipfsService = {
      getHelia: () => ipfs,
    };

    const orbitdb = await initOrbitDB(ipfsService);
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
      await initIpfs.stop();
      process.exit(0);
    });
  } catch (error) {
    appLogger.error('Failed to start node:', error);
  }
}

startNode();
