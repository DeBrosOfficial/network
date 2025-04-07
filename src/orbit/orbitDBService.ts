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
  return `${baseDir}-${fingerprint}`;
};

const ORBITDB_DIR = getOrbitDBDir();

export const getDBAddress = (name: string): string | null => {
  const addressFile = path.join(ORBITDB_DIR, `${name}.address`);
  if (fs.existsSync(addressFile)) {
    return fs.readFileSync(addressFile, 'utf-8').trim();
  }
  return null;
};

export const saveDBAddress = (name: string, address: string) => {
  const addressFile = path.join(ORBITDB_DIR, `${name}.address`);
  fs.writeFileSync(addressFile, address);
};

export const init = async () => {
  try {
    // Create directory if it doesn't exist
    if (!fs.existsSync(ORBITDB_DIR)) {
      fs.mkdirSync(ORBITDB_DIR, { recursive: true });
      logger.info(`Created OrbitDB directory: ${ORBITDB_DIR}`);
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
  } catch (e) {
    logger.error('Failed to initialize OrbitDB:', e);
    throw e;
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
      AccessController: IPFSAccessController({ write: ['*'] }),
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
