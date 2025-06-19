import { FieldConfig, ValidationError } from '../../types/models';

export function Field(config: FieldConfig) {
  return function (target: any, propertyKey: string) {
    // Validate field configuration
    validateFieldConfig(config);
    
    // Initialize fields map if it doesn't exist on this specific constructor
    if (!target.constructor.hasOwnProperty('fields')) {
      target.constructor.fields = new Map();
    }

    // Store field configuration
    target.constructor.fields.set(propertyKey, config);

    // Create getter/setter with validation and transformation
    const privateKey = `_${propertyKey}`;

    // Store the current descriptor (if any) - for future use
    const _currentDescriptor = Object.getOwnPropertyDescriptor(target, propertyKey);

    Object.defineProperty(target, propertyKey, {
      get() {
        return this[privateKey];
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

        // Set the value and mark as dirty
        this[privateKey] = transformedValue;
        if (this._isDirty !== undefined) {
          this._isDirty = true;
        }
      },
      enumerable: true,
      configurable: true,
    });

    // Set default value if provided
    if (config.default !== undefined) {
      Object.defineProperty(target, privateKey, {
        value: config.default,
        writable: true,
        enumerable: false,
        configurable: true,
      });
    }
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
  const fields = target.fields || (target.constructor && target.constructor.fields);
  if (!fields) {
    return undefined;
  }
  return fields.get(propertyKey);
}

// Export the decorator type for TypeScript
export type FieldDecorator = (config: FieldConfig) => (target: any, propertyKey: string) => void;
