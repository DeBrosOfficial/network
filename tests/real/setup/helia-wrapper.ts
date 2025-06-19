// Manual wrapper for ES modules to work with Jest
// This file provides CommonJS-compatible interfaces for pure ES modules

// Synchronous wrappers that use dynamic imports with await
export async function loadModules() {
  const [
    heliaModule,
    libp2pModule,
    tcpModule,
    noiseModule,
    yamuxModule,
    gossipsubModule,
    identifyModule,
  ] = await Promise.all([
    import('helia'),
    import('libp2p'),
    import('@libp2p/tcp'),
    import('@chainsafe/libp2p-noise'),
    import('@chainsafe/libp2p-yamux'),
    import('@chainsafe/libp2p-gossipsub'),
    import('@libp2p/identify'),
  ]);

  return {
    createHelia: heliaModule.createHelia,
    createLibp2p: libp2pModule.createLibp2p,
    tcp: tcpModule.tcp,
    noise: noiseModule.noise,
    yamux: yamuxModule.yamux,
    gossipsub: gossipsubModule.gossipsub,
    identify: identifyModule.identify,
  };
}

// Separate async loader for OrbitDB
export async function loadOrbitDBModules() {
  const orbitdbModule = await import('@orbitdb/core');

  return {
    createOrbitDB: orbitdbModule.createOrbitDB,
  };
}

// Separate async loaders for datastore modules that might have different import patterns
export async function loadDatastoreModules() {
  try {
    const [blockstoreModule, datastoreModule] = await Promise.all([
      import('blockstore-fs'),
      import('datastore-fs'),
    ]);

    return {
      FsBlockstore: blockstoreModule.FsBlockstore,
      FsDatastore: datastoreModule.FsDatastore,
    };
  } catch (_error) {
    // Fallback to require() for modules that might not be pure ES modules
    const FsBlockstore = require('blockstore-fs').FsBlockstore;
    const FsDatastore = require('datastore-fs').FsDatastore;

    return {
      FsBlockstore,
      FsDatastore,
    };
  }
}
