import { createServiceLogger } from '../../utils/logger';
import { init as initIpfs, stop as stopIpfs } from '../../ipfs/ipfsService';
import { init as initOrbitDB } from '../../orbit/orbitDBService';
import { DBConnection, ErrorCode } from '../types';
import { DBError } from './error';

const logger = createServiceLogger('DB_CONNECTION');

// Connection pool of database instances
const connections = new Map<string, DBConnection>();
let defaultConnectionId: string | null = null;
let cleanupInterval: NodeJS.Timeout | null = null;

// Configuration
const CONNECTION_TIMEOUT = 3600000; // 1 hour in milliseconds
const CLEANUP_INTERVAL = 300000; // 5 minutes in milliseconds
const MAX_RETRY_ATTEMPTS = 3;
const RETRY_DELAY = 2000; // 2 seconds

/**
 * Initialize the database service
 * This abstracts away OrbitDB and IPFS from the end user
 */
export const init = async (connectionId?: string): Promise<string> => {
  // Start connection cleanup interval if not already running
  if (!cleanupInterval) {
    cleanupInterval = setInterval(cleanupStaleConnections, CLEANUP_INTERVAL);
    logger.info(`Connection cleanup scheduled every ${CLEANUP_INTERVAL / 60000} minutes`);
  }

  const connId = connectionId || `conn_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

  // Check if connection already exists
  if (connections.has(connId)) {
    const existingConnection = connections.get(connId)!;
    if (existingConnection.isActive) {
      logger.info(`Using existing active connection: ${connId}`);
      return connId;
    }
  }

  logger.info(`Initializing DB service with connection ID: ${connId}`);

  let attempts = 0;
  let lastError: any = null;

  // Retry initialization with exponential backoff
  while (attempts < MAX_RETRY_ATTEMPTS) {
    try {
      // Initialize IPFS with retry logic
      const ipfsInstance = await initIpfs().catch((error) => {
        logger.error(
          `IPFS initialization failed (attempt ${attempts + 1}/${MAX_RETRY_ATTEMPTS}):`,
          error,
        );
        throw error;
      });

      // Initialize OrbitDB
      const orbitdbInstance = await initOrbitDB().catch((error) => {
        logger.error(
          `OrbitDB initialization failed (attempt ${attempts + 1}/${MAX_RETRY_ATTEMPTS}):`,
          error,
        );
        throw error;
      });

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
      lastError = error;
      attempts++;

      if (attempts >= MAX_RETRY_ATTEMPTS) {
        logger.error(
          `Failed to initialize DB service after ${MAX_RETRY_ATTEMPTS} attempts:`,
          error,
        );
        break;
      }

      // Wait before retrying with exponential backoff
      const delay = RETRY_DELAY * Math.pow(2, attempts - 1);
      logger.info(
        `Retrying initialization in ${delay}ms (attempt ${attempts + 1}/${MAX_RETRY_ATTEMPTS})...`,
      );

      // Clean up any partial initialization before retrying
      try {
        await stopIpfs();
      } catch (cleanupError) {
        logger.warn('Error during cleanup before retry:', cleanupError);
      }

      await new Promise((resolve) => setTimeout(resolve, delay));
    }
  }

  throw new DBError(
    ErrorCode.INITIALIZATION_FAILED,
    `Failed to initialize database service after ${MAX_RETRY_ATTEMPTS} attempts`,
    lastError,
  );
};

/**
 * Get the active connection
 */
export const getConnection = (connectionId?: string): DBConnection => {
  const connId = connectionId || defaultConnectionId;

  if (!connId || !connections.has(connId)) {
    throw new DBError(
      ErrorCode.NOT_INITIALIZED,
      `No active database connection found${connectionId ? ` for ID: ${connectionId}` : ''}`,
    );
  }

  const connection = connections.get(connId)!;

  if (!connection.isActive) {
    throw new DBError(ErrorCode.CONNECTION_ERROR, `Connection ${connId} is no longer active`);
  }

  // Update the timestamp to mark connection as recently used
  connection.timestamp = Date.now();

  return connection;
};

/**
 * Cleanup stale connections to prevent memory leaks
 */
export const cleanupStaleConnections = (): void => {
  try {
    const now = Date.now();
    let removedCount = 0;

    // Identify stale connections (older than CONNECTION_TIMEOUT)
    for (const [id, connection] of connections.entries()) {
      if (connection.isActive && now - connection.timestamp > CONNECTION_TIMEOUT) {
        logger.info(
          `Closing stale connection: ${id} (inactive for ${(now - connection.timestamp) / 60000} minutes)`,
        );

        // Close connection asynchronously (don't await to avoid blocking)
        closeConnection(id)
          .then((success) => {
            if (success) {
              logger.info(`Successfully closed stale connection: ${id}`);
            } else {
              logger.warn(`Failed to close stale connection: ${id}`);
            }
          })
          .catch((error) => {
            logger.error(`Error closing stale connection ${id}:`, error);
          });

        removedCount++;
      } else if (!connection.isActive) {
        // Remove inactive connections from the map
        connections.delete(id);
        removedCount++;
      }
    }

    if (removedCount > 0) {
      logger.info(`Cleaned up ${removedCount} stale or inactive connections`);
    }
  } catch (error) {
    logger.error('Error during connection cleanup:', error);
  }
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

    // Remove the connection from the pool
    connections.delete(connectionId);

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
    // Stop the cleanup interval
    if (cleanupInterval) {
      clearInterval(cleanupInterval);
      cleanupInterval = null;
    }

    // Close all connections
    const promises: Promise<boolean>[] = [];
    for (const [id, connection] of connections.entries()) {
      if (connection.isActive) {
        promises.push(closeConnection(id));
      }
    }

    // Wait for all connections to close
    await Promise.allSettled(promises);

    // Stop IPFS if needed
    const ipfs = connections.get(defaultConnectionId || '')?.ipfs;
    if (ipfs) {
      await stopIpfs();
    }

    // Clear all connections
    connections.clear();
    defaultConnectionId = null;

    logger.info('All DB connections stopped successfully');
  } catch (error: any) {
    logger.error('Error stopping DB connections:', error);
    throw new Error(`Failed to stop database connections: ${error.message}`);
  }
};
