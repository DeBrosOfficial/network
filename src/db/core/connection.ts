import { createServiceLogger } from '../../utils/logger';
import { init as initIpfs, stop as stopIpfs } from '../../ipfs/ipfsService';
import { init as initOrbitDB } from '../../orbit/orbitDBService';
import { DBConnection, ErrorCode } from '../types';
import { DBError } from './error';

const logger = createServiceLogger('DB_CONNECTION');

// Connection pool of database instances
const connections = new Map<string, DBConnection>();
let defaultConnectionId: string | null = null;

/**
 * Initialize the database service
 * This abstracts away OrbitDB and IPFS from the end user
 */
export const init = async (connectionId?: string): Promise<string> => {
  try {
    const connId = connectionId || `conn_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    logger.info(`Initializing DB service with connection ID: ${connId}`);
    
    // Initialize IPFS
    const ipfsInstance = await initIpfs();
    
    // Initialize OrbitDB
    const orbitdbInstance = await initOrbitDB();
    
    // Store connection in pool
    connections.set(connId, {
      ipfs: ipfsInstance,
      orbitdb: orbitdbInstance,
      timestamp: Date.now(),
      isActive: true,
    });
    
    // Set as default if no default exists
    if (!defaultConnectionId) {
      defaultConnectionId = connId;
    }
    
    logger.info(`DB service initialized successfully with connection ID: ${connId}`);
    return connId;
  } catch (error) {
    logger.error('Failed to initialize DB service:', error);
    throw new DBError(ErrorCode.INITIALIZATION_FAILED, 'Failed to initialize database service', error);
  }
};

/**
 * Get the active connection
 */
export const getConnection = (connectionId?: string): DBConnection => {
  const connId = connectionId || defaultConnectionId;
  
  if (!connId || !connections.has(connId)) {
    throw new DBError(
      ErrorCode.NOT_INITIALIZED, 
      `No active database connection found${connectionId ? ` for ID: ${connectionId}` : ''}`
    );
  }
  
  const connection = connections.get(connId)!;
  
  if (!connection.isActive) {
    throw new DBError(
      ErrorCode.CONNECTION_ERROR, 
      `Connection ${connId} is no longer active`
    );
  }
  
  return connection;
};

/**
 * Close a specific database connection
 */
export const closeConnection = async (connectionId: string): Promise<boolean> => {
  if (!connections.has(connectionId)) {
    return false;
  }
  
  try {
    const connection = connections.get(connectionId)!;
    
    // Stop OrbitDB
    if (connection.orbitdb) {
      await connection.orbitdb.stop();
    }
    
    // Mark connection as inactive
    connection.isActive = false;
    
    // If this was the default connection, clear it
    if (defaultConnectionId === connectionId) {
      defaultConnectionId = null;
      
      // Try to find another active connection to be the default
      for (const [id, conn] of connections.entries()) {
        if (conn.isActive) {
          defaultConnectionId = id;
          break;
        }
      }
    }
    
    logger.info(`Closed database connection: ${connectionId}`);
    return true;
  } catch (error) {
    logger.error(`Error closing connection ${connectionId}:`, error);
    return false;
  }
};

/**
 * Stop all database connections
 */
export const stop = async (): Promise<void> => {
  try {
    // Close all connections
    for (const [id, connection] of connections.entries()) {
      if (connection.isActive) {
        await closeConnection(id);
      }
    }
    
    // Stop IPFS if needed
    const ipfs = connections.get(defaultConnectionId || '')?.ipfs;
    if (ipfs) {
      await stopIpfs();
    }
    
    defaultConnectionId = null;
    logger.info('All DB connections stopped successfully');
  } catch (error) {
    logger.error('Error stopping DB connections:', error);
    throw error;
  }
};