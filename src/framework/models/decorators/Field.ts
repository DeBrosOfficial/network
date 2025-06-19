import { FieldConfig, ValidationError } from '../../types/models';

export function Field(config: FieldConfig) {
  return function (target: any, propertyKey: string) {
    // Initialize fields map if it doesn't exist
    if (!target.constructor.fields) {
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

        // Validate the field value
        const validationResult = validateFieldValue(transformedValue, config, propertyKey);
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
  if (!target.constructor.fields) {
    return undefined;
  }
  return target.constructor.fields.get(propertyKey);
}

// Export the decorator type for TypeScript
export type FieldDecorator = (config: FieldConfig) => (target: any, propertyKey: string) => void;
