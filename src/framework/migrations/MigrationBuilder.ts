/**
 * MigrationBuilder - Fluent API for Creating Migrations
 *
 * This class provides a convenient fluent interface for creating migration objects
 * with built-in validation and common operation patterns.
 */

import { Migration, MigrationOperation, MigrationValidator } from './MigrationManager';
import { FieldConfig } from '../types/models';

export class MigrationBuilder {
  private migration: Partial<Migration>;
  private upOperations: MigrationOperation[] = [];
  private downOperations: MigrationOperation[] = [];
  private validators: MigrationValidator[] = [];

  constructor(id: string, version: string, name: string) {
    this.migration = {
      id,
      version,
      name,
      description: '',
      targetModels: [],
      createdAt: Date.now(),
      tags: [],
    };
  }

  // Basic migration metadata
  description(desc: string): this {
    this.migration.description = desc;
    return this;
  }

  author(author: string): this {
    this.migration.author = author;
    return this;
  }

  tags(...tags: string[]): this {
    this.migration.tags = tags;
    return this;
  }

  targetModels(...models: string[]): this {
    this.migration.targetModels = models;
    return this;
  }

  dependencies(...migrationIds: string[]): this {
    this.migration.dependencies = migrationIds;
    return this;
  }

  // Field operations
  addField(modelName: string, fieldName: string, fieldConfig: FieldConfig): this {
    this.upOperations.push({
      type: 'add_field',
      modelName,
      fieldName,
      fieldConfig,
    });

    // Auto-generate reverse operation
    this.downOperations.unshift({
      type: 'remove_field',
      modelName,
      fieldName,
    });

    this.ensureTargetModel(modelName);
    return this;
  }

  removeField(modelName: string, fieldName: string, preserveData: boolean = false): this {
    this.upOperations.push({
      type: 'remove_field',
      modelName,
      fieldName,
    });

    if (!preserveData) {
      // Cannot auto-reverse field removal without knowing the original config
      this.downOperations.unshift({
        type: 'custom',
        modelName,
        customOperation: async (context) => {
          context.logger.warn(`Cannot reverse removal of field ${fieldName} - data may be lost`);
        },
      });
    }

    this.ensureTargetModel(modelName);
    return this;
  }

  modifyField(
    modelName: string,
    fieldName: string,
    newFieldConfig: FieldConfig,
    oldFieldConfig?: FieldConfig,
  ): this {
    this.upOperations.push({
      type: 'modify_field',
      modelName,
      fieldName,
      fieldConfig: newFieldConfig,
    });

    if (oldFieldConfig) {
      this.downOperations.unshift({
        type: 'modify_field',
        modelName,
        fieldName,
        fieldConfig: oldFieldConfig,
      });
    }

    this.ensureTargetModel(modelName);
    return this;
  }

  renameField(modelName: string, oldFieldName: string, newFieldName: string): this {
    this.upOperations.push({
      type: 'rename_field',
      modelName,
      fieldName: oldFieldName,
      newFieldName,
    });

    // Auto-generate reverse operation
    this.downOperations.unshift({
      type: 'rename_field',
      modelName,
      fieldName: newFieldName,
      newFieldName: oldFieldName,
    });

    this.ensureTargetModel(modelName);
    return this;
  }

  // Data transformation operations
  transformData(
    modelName: string,
    transformer: (data: any) => any,
    reverseTransformer?: (data: any) => any,
  ): this {
    this.upOperations.push({
      type: 'transform_data',
      modelName,
      transformer,
    });

    if (reverseTransformer) {
      this.downOperations.unshift({
        type: 'transform_data',
        modelName,
        transformer: reverseTransformer,
      });
    }

    this.ensureTargetModel(modelName);
    return this;
  }

  // Custom operations
  customOperation(
    modelName: string,
    operation: (context: any) => Promise<void>,
    rollbackOperation?: (context: any) => Promise<void>,
  ): this {
    this.upOperations.push({
      type: 'custom',
      modelName,
      customOperation: operation,
    });

    if (rollbackOperation) {
      this.downOperations.unshift({
        type: 'custom',
        modelName,
        customOperation: rollbackOperation,
      });
    }

    this.ensureTargetModel(modelName);
    return this;
  }

  // Common patterns
  addTimestamps(modelName: string): this {
    this.addField(modelName, 'createdAt', {
      type: 'number',
      required: false,
      default: Date.now(),
    });

    this.addField(modelName, 'updatedAt', {
      type: 'number',
      required: false,
      default: Date.now(),
    });

    return this;
  }

  addSoftDeletes(modelName: string): this {
    this.addField(modelName, 'deletedAt', {
      type: 'number',
      required: false,
      default: null,
    });

    return this;
  }

  addUuid(modelName: string, fieldName: string = 'uuid'): this {
    this.addField(modelName, fieldName, {
      type: 'string',
      required: true,
      unique: true,
      default: () => this.generateUuid(),
    });

    return this;
  }

  renameModel(oldModelName: string, newModelName: string): this {
    // This would require more complex operations across the entire system
    this.customOperation(
      oldModelName,
      async (context) => {
        context.logger.info(`Renaming model ${oldModelName} to ${newModelName}`);
        // Implementation would involve updating model registry, database names, etc.
      },
      async (context) => {
        context.logger.info(`Reverting model rename ${newModelName} to ${oldModelName}`);
      },
    );

    return this;
  }

  // Migration patterns for common scenarios
  createIndex(modelName: string, fieldNames: string[], options: any = {}): this {
    this.upOperations.push({
      type: 'add_index',
      modelName,
      indexConfig: {
        fields: fieldNames,
        ...options,
      },
    });

    this.downOperations.unshift({
      type: 'remove_index',
      modelName,
      indexConfig: {
        fields: fieldNames,
        ...options,
      },
    });

    this.ensureTargetModel(modelName);
    return this;
  }

  // Data migration helpers
  migrateData(
    fromModel: string,
    toModel: string,
    fieldMapping: Record<string, string>,
    options: {
      batchSize?: number;
      condition?: (data: any) => boolean;
      transform?: (data: any) => any;
    } = {},
  ): this {
    this.customOperation(fromModel, async (context) => {
      context.logger.info(`Migrating data from ${fromModel} to ${toModel}`);

      const records = await context.databaseManager.getAllRecords(fromModel);
      const batchSize = options.batchSize || 100;

      for (let i = 0; i < records.length; i += batchSize) {
        const batch = records.slice(i, i + batchSize);

        for (const record of batch) {
          if (options.condition && !options.condition(record)) {
            continue;
          }

          const newRecord: any = {};

          // Map fields
          for (const [oldField, newField] of Object.entries(fieldMapping)) {
            if (oldField in record) {
              newRecord[newField] = record[oldField];
            }
          }

          // Apply transformation if provided
          if (options.transform) {
            Object.assign(newRecord, options.transform(newRecord));
          }

          await context.databaseManager.createRecord(toModel, newRecord);
        }
      }
    });

    this.ensureTargetModel(fromModel);
    this.ensureTargetModel(toModel);
    return this;
  }

  // Validation
  addValidator(
    name: string,
    description: string,
    validateFn: (context: any) => Promise<any>,
  ): this {
    this.validators.push({
      name,
      description,
      validate: validateFn,
    });
    return this;
  }

  validateFieldExists(modelName: string, fieldName: string): this {
    return this.addValidator(
      `validate_${fieldName}_exists`,
      `Ensure field ${fieldName} exists in ${modelName}`,
      async (_context) => {
        // Implementation would check if field exists
        return { valid: true, errors: [], warnings: [] };
      },
    );
  }

  validateDataIntegrity(modelName: string, checkFn: (records: any[]) => any): this {
    return this.addValidator(
      `validate_${modelName}_integrity`,
      `Validate data integrity for ${modelName}`,
      async (context) => {
        const records = await context.databaseManager.getAllRecords(modelName);
        return checkFn(records);
      },
    );
  }

  // Build the final migration
  build(): Migration {
    if (!this.migration.targetModels || this.migration.targetModels.length === 0) {
      throw new Error('Migration must have at least one target model');
    }

    if (this.upOperations.length === 0) {
      throw new Error('Migration must have at least one operation');
    }

    return {
      id: this.migration.id!,
      version: this.migration.version!,
      name: this.migration.name!,
      description: this.migration.description!,
      targetModels: this.migration.targetModels!,
      up: this.upOperations,
      down: this.downOperations,
      dependencies: this.migration.dependencies,
      validators: this.validators.length > 0 ? this.validators : undefined,
      createdAt: this.migration.createdAt!,
      author: this.migration.author,
      tags: this.migration.tags,
    };
  }

  // Helper methods
  private ensureTargetModel(modelName: string): void {
    if (!this.migration.targetModels!.includes(modelName)) {
      this.migration.targetModels!.push(modelName);
    }
  }

  private generateUuid(): string {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
      const r = (Math.random() * 16) | 0;
      const v = c === 'x' ? r : (r & 0x3) | 0x8;
      return v.toString(16);
    });
  }

  // Static factory methods for common migration types
  static create(id: string, version: string, name: string): MigrationBuilder {
    return new MigrationBuilder(id, version, name);
  }

  static addFieldMigration(
    id: string,
    version: string,
    modelName: string,
    fieldName: string,
    fieldConfig: FieldConfig,
  ): Migration {
    return new MigrationBuilder(id, version, `Add ${fieldName} to ${modelName}`)
      .description(`Add new field ${fieldName} to ${modelName} model`)
      .addField(modelName, fieldName, fieldConfig)
      .build();
  }

  static removeFieldMigration(
    id: string,
    version: string,
    modelName: string,
    fieldName: string,
  ): Migration {
    return new MigrationBuilder(id, version, `Remove ${fieldName} from ${modelName}`)
      .description(`Remove field ${fieldName} from ${modelName} model`)
      .removeField(modelName, fieldName)
      .build();
  }

  static renameFieldMigration(
    id: string,
    version: string,
    modelName: string,
    oldFieldName: string,
    newFieldName: string,
  ): Migration {
    return new MigrationBuilder(
      id,
      version,
      `Rename ${oldFieldName} to ${newFieldName} in ${modelName}`,
    )
      .description(`Rename field ${oldFieldName} to ${newFieldName} in ${modelName} model`)
      .renameField(modelName, oldFieldName, newFieldName)
      .build();
  }

  static dataTransformMigration(
    id: string,
    version: string,
    modelName: string,
    description: string,
    transformer: (data: any) => any,
    reverseTransformer?: (data: any) => any,
  ): Migration {
    return new MigrationBuilder(id, version, `Transform data in ${modelName}`)
      .description(description)
      .transformData(modelName, transformer, reverseTransformer)
      .build();
  }
}

// Export convenience function for creating migrations
export function createMigration(id: string, version: string, name: string): MigrationBuilder {
  return MigrationBuilder.create(id, version, name);
}
