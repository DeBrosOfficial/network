import { ErrorCode } from '../types';

// Re-export error code for easier access
export { ErrorCode };


// Custom error class with error codes
export class DBError extends Error {
  code: ErrorCode;
  details?: any;

  constructor(code: ErrorCode, message: string, details?: any) {
    super(message);
    this.name = 'DBError';
    this.code = code;
    this.details = details;
  }
}