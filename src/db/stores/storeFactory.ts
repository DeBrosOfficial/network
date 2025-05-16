import { createServiceLogger } from '../../utils/logger';
import { StoreType, ErrorCode } from '../types';
import { DBError } from '../core/error';
import { BaseStore } from './baseStore';
import { KeyValueStore } from './keyValueStore';
import { DocStore } from './docStore';
import { FeedStore } from './feedStore';
import { CounterStore } from './counterStore';

const logger = createServiceLogger('STORE_FACTORY');

// Initialize instances for each store type - singleton pattern
const storeInstances = new Map<StoreType, BaseStore>();

// Store type mapping to implementations
const storeImplementations = {
  [StoreType.KEYVALUE]: KeyValueStore,
  [StoreType.DOCSTORE]: DocStore,
  [StoreType.FEED]: FeedStore,
  [StoreType.EVENTLOG]: FeedStore, // Alias for feed
  [StoreType.COUNTER]: CounterStore,
};

/**
 * Get a store instance by type (factory and singleton pattern)
 */
export function getStore(type: StoreType): BaseStore {
  // Return cached instance if available (singleton pattern)
  if (storeInstances.has(type)) {
    return storeInstances.get(type)!;
  }

  // Get the store implementation class
  const StoreClass = storeImplementations[type];

  if (!StoreClass) {
    logger.error(`Unsupported store type: ${type}`);
    throw new DBError(ErrorCode.STORE_TYPE_ERROR, `Unsupported store type: ${type}`);
  }

  // Create a new instance of the store
  const store = new StoreClass();

  // Cache the instance for future use
  storeInstances.set(type, store);

  return store;
}
