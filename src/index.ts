// Config exports
import { config, defaultConfig, type DebrosConfig } from './config';
import { validateConfig, type ValidationResult } from './ipfs/config/configValidator';

// Database service exports (new abstracted layer)
import {
  init as initDB,
  create,
  get,
  update,
  remove,
  list,
  query,
  createIndex,
  createTransaction,
  commitTransaction,
  subscribe,
  uploadFile,
  getFile,
  deleteFile,
  defineSchema,
  closeConnection,
  stop as stopDB,
} from './db/dbService';
import { ErrorCode, StoreType } from './db/types';

// Import types
import type {
  Transaction,
  CreateResult,
  UpdateResult,
  PaginatedResult,
  ListOptions,
  QueryOptions,
  FileUploadResult,
  FileResult,
  CollectionSchema,
  SchemaDefinition,
  Metrics,
} from './db/types';

import { DBError } from './db/core/error';

// Legacy exports (internal use only, not exposed in default export)
import { getConnectedPeers, logPeersStatus } from './ipfs/ipfsService';

// Load balancer exports
import loadBalancerController from './ipfs/loadBalancerController';

// Logger exports
import logger, {
  createServiceLogger,
  createDebrosLogger,
  type LoggerOptions,
} from './utils/logger';

// Export public API
export {
  // Configuration
  config,
  defaultConfig,
  validateConfig,
  type DebrosConfig,
  type ValidationResult,

  // Database Service (Main public API)
  initDB,
  create,
  get,
  update,
  remove,
  list,
  query,
  createIndex,
  createTransaction,
  commitTransaction,
  subscribe,
  uploadFile,
  getFile,
  deleteFile,
  defineSchema,
  closeConnection,
  stopDB,
  ErrorCode,
  StoreType,

  // Load Balancer
  loadBalancerController,
  getConnectedPeers,
  logPeersStatus,

  // Types
  type Transaction,
  type DBError,
  type CollectionSchema,
  type SchemaDefinition,
  type CreateResult,
  type UpdateResult,
  type PaginatedResult,
  type ListOptions,
  type QueryOptions,
  type FileUploadResult,
  type FileResult,
  type Metrics,

  // Logger
  logger,
  createServiceLogger,
  createDebrosLogger,
  type LoggerOptions,
};

// Default export for convenience
export default {
  config,
  validateConfig,
  // Database Service as main interface
  db: {
    init: initDB,
    create,
    get,
    update,
    remove,
    list,
    query,
    createIndex,
    createTransaction,
    commitTransaction,
    subscribe,
    uploadFile,
    getFile,
    deleteFile,
    defineSchema,
    closeConnection,
    stop: stopDB,
    ErrorCode,
    StoreType,
  },
  loadBalancerController,
  logPeersStatus,
  getConnectedPeers,
  logger,
  createServiceLogger,
};
