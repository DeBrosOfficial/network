import { FieldConfig, ValidationError } from '../../types/models';

export function Field(config: FieldConfig) {
  return function (target: any, propertyKey: string) {
    // Validate field configuration
    validateFieldConfig(config);
    
    // Initialize fields map if it doesn't exist, inheriting from parent
    if (!target.constructor.hasOwnProperty('fields')) {
      // Copy fields from parent class if they exist
      const parentFields = target.constructor.fields || new Map();
      target.constructor.fields = new Map(parentFields);
    }

    // Store field configuration
    target.constructor.fields.set(propertyKey, config);

    // Create getter/setter with validation and transformation
    const privateKey = `_${propertyKey}`;

    // Store the current descriptor (if any) - for future use
    const _currentDescriptor = Object.getOwnPropertyDescriptor(target, propertyKey);

    Object.defineProperty(target, propertyKey, {
      get() {
        // Explicitly construct the private key to avoid closure issues
        const key = `_${propertyKey}`;
        return this[key];
      },
      set(value) {
        // Apply transformation first
        const transformedValue = config.transform ? config.transform(value) : value;

        // Only validate non-required constraints during assignment
        // Required field validation will happen during save()
        const validationResult = validateFieldValueNonRequired(transformedValue, config, propertyKey);
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

    // Don't set default values here - let BaseModel constructor handle it
    // This ensures proper inheritance and instance-specific defaults
  };
}

function validateFieldConfig(config: FieldConfig): void {
  const validTypes = ['string', 'number', 'boolean', 'array', 'object', 'date'];
  if (!validTypes.includes(config.type)) {
    throw new Error(`Invalid field type: ${config.type}. Valid types are: ${validTypes.join(', ')}`);
  }
}

function validateFieldValue(
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

// Export the decorator type for TypeScript
export type FieldDecorator = (config: FieldConfig) => (target: any, propertyKey: string) => void;
