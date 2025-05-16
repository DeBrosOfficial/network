import fs from 'fs';
import path from 'path';
import { createOrbitDB, IPFSAccessController } from '@orbitdb/core';
import { registerFeed } from '@orbitdb/feed-db';
import { config } from '../config';
import { createServiceLogger } from '../utils/logger';
import { getHelia } from '../ipfs/ipfsService';

const logger = createServiceLogger('ORBITDB');

let orbitdb: any;

// Create a node-specific directory based on fingerprint to avoid lock conflicts
export const getOrbitDBDir = (): string => {
  const baseDir = config.orbitdb.directory;
  const fingerprint = config.env.fingerprint;
  // Use path.join for proper cross-platform path handling
  return path.join(baseDir, `debros-${fingerprint}`);
};

const ORBITDB_DIR = getOrbitDBDir();
const ADDRESS_DIR = path.join(ORBITDB_DIR, 'addresses');

export const getDBAddress = (name: string): string | null => {
  try {
    const addressFile = path.join(ADDRESS_DIR, `${name}.address`);
    if (fs.existsSync(addressFile)) {
      return fs.readFileSync(addressFile, 'utf-8').trim();
    }
  } catch (error) {
    logger.error(`Error reading DB address for ${name}:`, error);
  }
  return null;
};

export const saveDBAddress = (name: string, address: string): boolean => {
  try {
    // Ensure the address directory exists
    if (!fs.existsSync(ADDRESS_DIR)) {
      fs.mkdirSync(ADDRESS_DIR, { recursive: true, mode: 0o755 });
    }

    const addressFile = path.join(ADDRESS_DIR, `${name}.address`);
    fs.writeFileSync(addressFile, address, { mode: 0o644 });
    logger.info(`Saved DB address for ${name} at ${addressFile}`);
    return true;
  } catch (error) {
    logger.error(`Failed to save DB address for ${name}:`, error);
    return false;
  }
};

export const init = async () => {
  try {
    // Create directory with proper permissions if it doesn't exist
    try {
      if (!fs.existsSync(ORBITDB_DIR)) {
        fs.mkdirSync(ORBITDB_DIR, { recursive: true, mode: 0o755 });
        logger.info(`Created OrbitDB directory: ${ORBITDB_DIR}`);
      }

      // Check write permissions
      fs.accessSync(ORBITDB_DIR, fs.constants.W_OK);
    } catch (permError: any) {
      logger.error(`Permission error with OrbitDB directory: ${ORBITDB_DIR}`, permError);
      throw new Error(`Cannot access or write to OrbitDB directory: ${permError.message}`);
    }

    // Create the addresses directory
    try {
      if (!fs.existsSync(ADDRESS_DIR)) {
        fs.mkdirSync(ADDRESS_DIR, { recursive: true, mode: 0o755 });
        logger.info(`Created OrbitDB addresses directory: ${ADDRESS_DIR}`);
      }
    } catch (dirError) {
      logger.error(`Error creating addresses directory: ${ADDRESS_DIR}`, dirError);
      // Continue anyway, we'll handle failures when saving addresses
    }

    registerFeed();

    const ipfs = getHelia();
    if (!ipfs) {
      throw new Error('IPFS instance is not initialized.');
    }

    logger.info(`Initializing OrbitDB with directory: ${ORBITDB_DIR}`);

    orbitdb = await createOrbitDB({
      ipfs,
      directory: ORBITDB_DIR,
    });

    logger.info('OrbitDB initialized successfully.');
    return orbitdb;
  } catch (e: any) {
    logger.error('Failed to initialize OrbitDB:', e);
    throw new Error(`OrbitDB initialization failed: ${e.message}`);
  }
};

export const openDB = async (name: string, type: string) => {
  if (!orbitdb) {
    throw new Error('OrbitDB not initialized. Call init() first.');
  }

  const existingAddress = getDBAddress(name);
  let db;

  try {
    const dbOptions = {
      type,
      overwrite: false,
      AccessController: IPFSAccessController({
        write: ['*'],
      }),
    };

    if (existingAddress) {
      logger.info(`Loading existing database with address: ${existingAddress}`);
      db = await orbitdb.open(existingAddress, dbOptions);
    } else {
      logger.info(`Creating new database: ${name}`);
      db = await orbitdb.open(name, dbOptions);
      saveDBAddress(name, db.address.toString());
    }

    // Log the access controller type to verify
    logger.info('Access Controller Type:', db.access.type);
    return db;
  } catch (error) {
    logger.error(`Error opening database '${name}':`, error);
    throw error;
  }
};

export const getOrbitDB = () => {
  return orbitdb;
};

export const db = async (dbName: string, type: string) => {
  try {
    if (!orbitdb) {
      throw new Error('OrbitDB not initialized. Call init() first.');
    }

    return await openDB(dbName, type);
  } catch (error: any) {
    logger.error(`Error accessing database '${dbName}':`, error);
    throw new Error(`Database error: ${error.message}`);
  }
};

export default {
  init,
  openDB,
  getOrbitDB,
  db,
};
