// Example of how to integrate the new @debros/node-core package into an application

import express from 'express';
import { initIpfs, initOrbitDB, createServiceLogger, getConnectedPeers } from '@debros/node-core'; // This would be the import path when installed from npm

// Create service-specific loggers
const apiLogger = createServiceLogger('API');
const networkLogger = createServiceLogger('NETWORK');

// Initialize Express app
const app = express();
app.use(express.json());

// Network state
let ipfsNode: any;
let orbitInstance: any;
let messageDB: any;

// Initialize network components
async function initializeNetwork() {
  try {
    // Initialize IPFS
    networkLogger.info('Initializing IPFS node...');
    ipfsNode = await initIpfs();

    // Initialize OrbitDB
    networkLogger.info('Initializing OrbitDB...');
    orbitInstance = await initOrbitDB({
      getHelia: () => ipfsNode,
    });

    // Open message database
    messageDB = await orbitInstance.open('messages', {
      type: 'feed',
    });

    networkLogger.info('Network components initialized successfully');

    // Log connected peers every minute
    setInterval(() => {
      const peers = getConnectedPeers();
      networkLogger.info(`Connected to ${peers.size} peers`);
    }, 60000);

    return true;
  } catch (error) {
    networkLogger.error('Failed to initialize network:', error);
    return false;
  }
}

// API routes
app.get('/api/peers', (req, res) => {
  const peers = getConnectedPeers();
  const peerList: any[] = [];

  peers.forEach((data, peerId) => {
    peerList.push({
      id: peerId,
      load: data.load,
      address: data.publicAddress,
      lastSeen: data.lastSeen,
    });
  });

  res.json({ peers: peerList });
});

app.get('/api/messages', async (req, res) => {
  try {
    const messages = messageDB.iterator({ limit: 100 }).collect();
    res.json({ messages });
  } catch (error) {
    apiLogger.error('Error fetching messages:', error);
    res.status(500).json({ error: 'Failed to fetch messages' });
  }
});

app.post('/api/messages', async (req, res) => {
  try {
    const { content } = req.body;
    if (!content) {
      return res.status(400).json({ error: 'Content is required' });
    }

    const entry = await messageDB.add({
      content,
      timestamp: Date.now(),
    });

    res.status(201).json({ id: entry });
  } catch (error) {
    apiLogger.error('Error creating message:', error);
    res.status(500).json({ error: 'Failed to create message' });
  }
});

// Start the application
async function startApp() {
  const networkInitialized = await initializeNetwork();

  if (networkInitialized) {
    const port = config.env.port;
    app.listen(port, () => {
      apiLogger.info(`Server listening on port ${port}`);
    });
  } else {
    apiLogger.error('Cannot start application: Network initialization failed');
    process.exit(1);
  }
}

// Shutdown handler
process.on('SIGINT', async () => {
  networkLogger.info('Application shutting down...');
  if (orbitInstance) {
    await orbitInstance.stop();
  }
  if (ipfsNode) {
    await initIpfs.stop();
  }
  process.exit(0);
});

// Start the application
startApp();
