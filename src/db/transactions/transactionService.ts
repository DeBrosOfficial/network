import { createServiceLogger } from '../../utils/logger';
import { ErrorCode } from '../types';
import { DBError } from '../core/error';

const logger = createServiceLogger('DB_TRANSACTION');

// Transaction operation type
interface TransactionOperation {
  type: 'create' | 'update' | 'delete';
  collection: string;
  id: string;
  data?: any;
}

/**
 * Transaction object for batching operations
 */
export class Transaction {
  private operations: TransactionOperation[] = [];
  private connectionId?: string;
  
  constructor(connectionId?: string) {
    this.connectionId = connectionId;
  }
  
  /**
   * Add a create operation to the transaction
   */
  create<T>(collection: string, id: string, data: T): Transaction {
    this.operations.push({
      type: 'create',
      collection,
      id,
      data
    });
    return this;
  }
  
  /**
   * Add an update operation to the transaction
   */
  update<T>(collection: string, id: string, data: Partial<T>): Transaction {
    this.operations.push({
      type: 'update',
      collection,
      id,
      data
    });
    return this;
  }
  
  /**
   * Add a delete operation to the transaction
   */
  delete(collection: string, id: string): Transaction {
    this.operations.push({
      type: 'delete',
      collection,
      id
    });
    return this;
  }
  
  /**
   * Get all operations in this transaction
   */
  getOperations(): TransactionOperation[] {
    return [...this.operations];
  }
  
  /**
   * Get connection ID for this transaction
   */
  getConnectionId(): string | undefined {
    return this.connectionId;
  }
}