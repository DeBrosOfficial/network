import { createServiceLogger } from '../../utils/logger';
import { CollectionSchema, ErrorCode } from '../types';
import { DBError } from '../core/error';

const logger = createServiceLogger('DB_SCHEMA');

// Store collection schemas
const schemas = new Map<string, CollectionSchema>();

/**
 * Define a schema for a collection
 */
export const defineSchema = (collection: string, schema: CollectionSchema): void => {
  schemas.set(collection, schema);
  logger.info(`Schema defined for collection: ${collection}`);
};

/**
 * Validate a document against its schema
 */
export const validateDocument = (collection: string, document: any): boolean => {
  const schema = schemas.get(collection);
  
  if (!schema) {
    return true; // No schema defined, so validation passes
  }
  
  // Check required fields
  if (schema.required) {
    for (const field of schema.required) {
      if (document[field] === undefined) {
        throw new DBError(
          ErrorCode.INVALID_SCHEMA,
          `Required field '${field}' is missing`,
          { collection, document }
        );
      }
    }
  }
  
  // Validate properties
  for (const [field, definition] of Object.entries(schema.properties)) {
    const value = document[field];
    
    // Skip undefined optional fields
    if (value === undefined) {
      if (definition.required) {
        throw new DBError(
          ErrorCode.INVALID_SCHEMA,
          `Required field '${field}' is missing`,
          { collection, document }
        );
      }
      continue;
    }
    
    // Type validation
    switch (definition.type) {
      case 'string':
        if (typeof value !== 'string') {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must be a string`,
            { collection, field, value }
          );
        }
        
        // Pattern validation
        if (definition.pattern && !new RegExp(definition.pattern).test(value)) {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' does not match pattern: ${definition.pattern}`,
            { collection, field, value }
          );
        }
        
        // Length validation
        if (definition.min !== undefined && value.length < definition.min) {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must have at least ${definition.min} characters`,
            { collection, field, value }
          );
        }
        
        if (definition.max !== undefined && value.length > definition.max) {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must have at most ${definition.max} characters`,
            { collection, field, value }
          );
        }
        break;
        
      case 'number':
        if (typeof value !== 'number') {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must be a number`,
            { collection, field, value }
          );
        }
        
        // Range validation
        if (definition.min !== undefined && value < definition.min) {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must be at least ${definition.min}`,
            { collection, field, value }
          );
        }
        
        if (definition.max !== undefined && value > definition.max) {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must be at most ${definition.max}`,
            { collection, field, value }
          );
        }
        break;
        
      case 'boolean':
        if (typeof value !== 'boolean') {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must be a boolean`,
            { collection, field, value }
          );
        }
        break;
        
      case 'array':
        if (!Array.isArray(value)) {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must be an array`,
            { collection, field, value }
          );
        }
        
        // Length validation
        if (definition.min !== undefined && value.length < definition.min) {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must have at least ${definition.min} items`,
            { collection, field, value }
          );
        }
        
        if (definition.max !== undefined && value.length > definition.max) {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must have at most ${definition.max} items`,
            { collection, field, value }
          );
        }
        
        // Validate array items if item schema is defined
        if (definition.items && value.length > 0) {
          for (let i = 0; i < value.length; i++) {
            const item = value[i];
            
            // This is a simplified item validation
            // In a real implementation, this would recursively validate complex objects
            switch (definition.items.type) {
              case 'string':
                if (typeof item !== 'string') {
                  throw new DBError(
                    ErrorCode.INVALID_SCHEMA,
                    `Item at index ${i} in field '${field}' must be a string`,
                    { collection, field, item }
                  );
                }
                break;
                
              case 'number':
                if (typeof item !== 'number') {
                  throw new DBError(
                    ErrorCode.INVALID_SCHEMA,
                    `Item at index ${i} in field '${field}' must be a number`,
                    { collection, field, item }
                  );
                }
                break;
                
              case 'boolean':
                if (typeof item !== 'boolean') {
                  throw new DBError(
                    ErrorCode.INVALID_SCHEMA,
                    `Item at index ${i} in field '${field}' must be a boolean`,
                    { collection, field, item }
                  );
                }
                break;
            }
          }
        }
        break;
        
      case 'object':
        if (typeof value !== 'object' || value === null || Array.isArray(value)) {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must be an object`,
            { collection, field, value }
          );
        }
        
        // Nested object validation would go here in a real implementation
        break;
        
      case 'enum':
        if (definition.enum && !definition.enum.includes(value)) {
          throw new DBError(
            ErrorCode.INVALID_SCHEMA,
            `Field '${field}' must be one of: ${definition.enum.join(', ')}`,
            { collection, field, value }
          );
        }
        break;
    }
  }
  
  return true;
};