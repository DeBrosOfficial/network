import { dbEvents } from '../types';

// Event types
type DBEventType = 'document:created' | 'document:updated' | 'document:deleted';

/**
 * Subscribe to database events
 */
export const subscribe = (
  event: DBEventType,
  callback: (data: any) => void
): () => void => {
  dbEvents.on(event, callback);
  
  // Return unsubscribe function
  return () => {
    dbEvents.off(event, callback);
  };
};

/**
 * Emit an event
 */
export const emit = (
  event: DBEventType,
  data: any
): void => {
  dbEvents.emit(event, data);
};

/**
 * Remove all event listeners
 */
export const removeAllListeners = (): void => {
  dbEvents.removeAllListeners();
};