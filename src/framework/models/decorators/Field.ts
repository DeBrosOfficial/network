import { FieldConfig, ValidationError } from '../../types/models';
import { BaseModel } from '../BaseModel';

export function Field(config: FieldConfig) {
  return function (target: any, propertyKey: string) {
    // Validate field configuration
    validateFieldConfig(config);
    
    // Handle ESM case where target might be undefined
    if (!target || typeof target !== 'object') {
      // Skip the decorator if target is not available - the field will be handled later
      return;
    }
    
    // Get the constructor function - handle ESM case where constructor might be undefined
    const ctor = (target.constructor || target) as typeof BaseModel;
    
    // Initialize fields map if it doesn't exist
    if (!ctor.hasOwnProperty('fields')) {
      const parentFields = ctor.fields ? new Map(ctor.fields) : new Map();
      Object.defineProperty(ctor, 'fields', {
        value: parentFields,
        writable: true,
        enumerable: false,
        configurable: true,
      });
    }
    
    // Store field configuration
    ctor.fields.set(propertyKey, config);

    // Define property on the prototype
    Object.defineProperty(target, propertyKey, {
      get() {
        const privateKey = `_${propertyKey}`;
        return this[privateKey];
      },
      set(value) {
        const privateKey = `_${propertyKey}`;
        const ctor = this.constructor as typeof BaseModel;

        // Ensure fields map exists on the constructor
        if (!ctor.hasOwnProperty('fields')) {
          const parentFields = ctor.fields ? new Map(ctor.fields) : new Map();
          Object.defineProperty(ctor, 'fields', {
            value: parentFields,
            writable: true,
            enumerable: false,
            configurable: true,
          });
        }

        // Store field configuration if it's not already there
        if (!ctor.fields.has(propertyKey)) {
          ctor.fields.set(propertyKey, config);
        }

        // Apply transformation first
        const transformedValue = config.transform ? config.transform(value) : value;

        // Only validate non-required constraints during assignment
        const validationResult = validateFieldValueNonRequired(
          transformedValue,
          config,
          propertyKey,
        );
        if (!validationResult.valid) {
          throw new ValidationError(validationResult.errors);
        }

        // Check if value actually changed
        const oldValue = this[privateKey];
        if (oldValue !== transformedValue) {
          // Set the value and mark as dirty
          this[privateKey] = transformedValue;
          if (this._isDirty !== undefined) {
            this._isDirty = true;
          }
          // Track field modification
          if (this.markFieldAsModified && typeof this.markFieldAsModified === 'function') {
            this.markFieldAsModified(propertyKey);
          }
        }
      },
      enumerable: true,
      configurable: true,
    });
  };
}

function validateFieldConfig(config: FieldConfig): void {
  const validTypes = ['string', 'number', 'boolean', 'array', 'object', 'date'];
  if (!validTypes.includes(config.type)) {
    throw new Error(
      `Invalid field type: ${config.type}. Valid types are: ${validTypes.join(', ')}`,
    );
  }
}

function _validateFieldValue(
  value: any,
  config: FieldConfig,
  fieldName: string,
): { valid: boolean; errors: string[] } {
  const errors: string[] = [];

  // Required validation
  if (config.required && (value === undefined || value === null || value === '')) {
    errors.push(`${fieldName} is required`);
    return { valid: false, errors };
  }

  // Skip further validation if value is empty and not required
  if (value === undefined || value === null) {
    return { valid: true, errors: [] };
  }

  // Type validation
  if (!isValidType(value, config.type)) {
    errors.push(`${fieldName} must be of type ${config.type}`);
  }

  // Custom validation
  if (config.validate) {
    const customResult = config.validate(value);
    if (customResult === false) {
      errors.push(`${fieldName} failed custom validation`);
    } else if (typeof customResult === 'string') {
      errors.push(customResult);
    }
  }

  return { valid: errors.length === 0, errors };
}

function validateFieldValueNonRequired(
  value: any,
  config: FieldConfig,
  fieldName: string,
): { valid: boolean; errors: string[] } {
  const errors: string[] = [];

  // Skip required validation during assignment
  // Skip further validation if value is empty
  if (value === undefined || value === null) {
    return { valid: true, errors: [] };
  }

  // Type validation
  if (!isValidType(value, config.type)) {
    errors.push(`${fieldName} must be of type ${config.type}`);
  }

  // Custom validation
  if (config.validate) {
    const customResult = config.validate(value);
    if (customResult === false) {
      errors.push(`${fieldName} failed custom validation`);
    } else if (typeof customResult === 'string') {
      errors.push(customResult);
    }
  }

  return { valid: errors.length === 0, errors };
}

function isValidType(value: any, expectedType: FieldConfig['type']): boolean {
  switch (expectedType) {
    case 'string':
      return typeof value === 'string';
    case 'number':
      return typeof value === 'number' && !isNaN(value);
    case 'boolean':
      return typeof value === 'boolean';
    case 'array':
      return Array.isArray(value);
    case 'object':
      return typeof value === 'object' && !Array.isArray(value);
    case 'date':
      return value instanceof Date || (typeof value === 'number' && !isNaN(value));
    default:
      return true;
  }
}

// Utility function to get field configuration
export function getFieldConfig(target: any, propertyKey: string): FieldConfig | undefined {
  // Handle both class constructors and instances
  let current = target;
  if (target.constructor && target.constructor !== Function) {
    current = target.constructor;
  }

  // Walk up the prototype chain to find field configuration
  while (current && current !== Function && current !== Object) {
    if (current.fields && current.fields.has(propertyKey)) {
      return current.fields.get(propertyKey);
    }
    current = Object.getPrototypeOf(current);
    // Stop if we've reached the base class or gone too far
    if (current === Function.prototype || current === Object.prototype) {
      break;
    }
  }

  return undefined;
}

// Deferred setup function for ESM environments
function deferredFieldSetup(config: FieldConfig, propertyKey: string) {
  // Return a function that will be called when the class is properly initialized
  return function() {
    // This function will be called later when the class prototype is ready
    console.warn(`Deferred field setup not yet implemented for property ${propertyKey}`);
  };
}

// Export the decorator type for TypeScript
export type FieldDecorator = (config: FieldConfig) => (target: any, propertyKey: string) => void;
