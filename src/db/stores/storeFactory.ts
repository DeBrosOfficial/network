import { createServiceLogger } from '../../utils/logger';
import { StoreType, ErrorCode } from '../types';
import { DBError } from '../core/error';
import { BaseStore } from './baseStore';
import { KeyValueStore } from './keyValueStore';
import { DocStore } from './docStore';
import { FeedStore } from './feedStore';
import { CounterStore } from './counterStore';

const logger = createServiceLogger('STORE_FACTORY');

// Initialize instances for each store type
const storeInstances = new Map<StoreType, BaseStore>();

/**
 * Get a store instance by type
 */
export function getStore(type: StoreType): BaseStore {
  // Check if we already have an instance
  if (storeInstances.has(type)) {
    return storeInstances.get(type)!;
  }
  
  // Create a new instance based on type
  let store: BaseStore;
  
  switch (type) {
    case StoreType.KEYVALUE:
      store = new KeyValueStore();
      break;
      
    case StoreType.DOCSTORE:
      store = new DocStore();
      break;
      
    case StoreType.FEED:
    case StoreType.EVENTLOG: // Alias for feed
      store = new FeedStore();
      break;
      
    case StoreType.COUNTER:
      store = new CounterStore();
      break;
      
    default:
      logger.error(`Unsupported store type: ${type}`);
      throw new DBError(ErrorCode.STORE_TYPE_ERROR, `Unsupported store type: ${type}`);
  }
  
  // Cache the instance
  storeInstances.set(type, store);
  
  return store;
}