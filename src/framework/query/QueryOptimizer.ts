import { QueryBuilder } from './QueryBuilder';
import { QueryCondition } from '../types/queries';
import { BaseModel } from '../models/BaseModel';

export interface QueryPlan {
  strategy: 'single_user' | 'multi_user' | 'global_index' | 'all_shards' | 'specific_shards';
  targetDatabases: string[];
  estimatedCost: number;
  indexHints: string[];
  optimizations: string[];
}

export class QueryOptimizer {
  static analyzeQuery<T extends BaseModel>(query: QueryBuilder<T>): QueryPlan {
    const model = query.getModel();
    const conditions = query.getConditions();
    const relations = query.getRelations();
    const limit = query.getLimit();

    let strategy: QueryPlan['strategy'] = 'all_shards';
    let targetDatabases: string[] = [];
    let estimatedCost = 100; // Base cost
    let indexHints: string[] = [];
    let optimizations: string[] = [];

    // Analyze based on model scope
    if (model.scope === 'user') {
      const userConditions = conditions.filter(
        (c) => c.field === 'userId' || c.operator === 'userIn',
      );

      if (userConditions.length > 0) {
        const userCondition = userConditions[0];

        if (userCondition.operator === 'userIn') {
          strategy = 'multi_user';
          targetDatabases = userCondition.value.map(
            (userId: string) => `${userId}-${model.modelName.toLowerCase()}`,
          );
          estimatedCost = 20 * userCondition.value.length;
          optimizations.push('Direct user database access');
        } else {
          strategy = 'single_user';
          targetDatabases = [`${userCondition.value}-${model.modelName.toLowerCase()}`];
          estimatedCost = 10;
          optimizations.push('Single user database access');
        }
      } else {
        strategy = 'global_index';
        targetDatabases = [`${model.modelName}GlobalIndex`];
        estimatedCost = 50;
        indexHints.push(`${model.modelName}GlobalIndex`);
        optimizations.push('Global index lookup');
      }
    } else {
      // Global model
      if (model.sharding) {
        const shardKeyCondition = conditions.find((c) => c.field === model.sharding!.key);

        if (shardKeyCondition) {
          if (shardKeyCondition.operator === '=') {
            strategy = 'specific_shards';
            targetDatabases = [`${model.modelName}-shard-specific`];
            estimatedCost = 15;
            optimizations.push('Single shard access');
          } else if (shardKeyCondition.operator === 'in') {
            strategy = 'specific_shards';
            targetDatabases = shardKeyCondition.value.map(
              (_: any, i: number) => `${model.modelName}-shard-${i}`,
            );
            estimatedCost = 15 * shardKeyCondition.value.length;
            optimizations.push('Multiple specific shards');
          }
        } else {
          strategy = 'all_shards';
          estimatedCost = 30 * (model.sharding.count || 4);
          optimizations.push('All shards scan');
        }
      } else {
        strategy = 'single_user'; // Actually single global database
        targetDatabases = [`global-${model.modelName.toLowerCase()}`];
        estimatedCost = 25;
        optimizations.push('Single global database');
      }
    }

    // Adjust cost based on other factors
    if (limit && limit < 100) {
      estimatedCost *= 0.8;
      optimizations.push(`Limit optimization (${limit})`);
    }

    if (relations.length > 0) {
      estimatedCost *= 1 + relations.length * 0.3;
      optimizations.push(`Relationship loading (${relations.length})`);
    }

    // Suggest indexes based on conditions
    const indexedFields = conditions
      .filter((c) => c.field !== 'userId' && c.field !== '__or__' && c.field !== '__raw__')
      .map((c) => c.field);

    if (indexedFields.length > 0) {
      indexHints.push(...indexedFields.map((field) => `${model.modelName}_${field}_idx`));
    }

    return {
      strategy,
      targetDatabases,
      estimatedCost,
      indexHints,
      optimizations,
    };
  }

  static optimizeConditions(conditions: QueryCondition[]): QueryCondition[] {
    const optimized = [...conditions];

    // Remove redundant conditions
    const seen = new Set();
    const filtered = optimized.filter((condition) => {
      const key = `${condition.field}_${condition.operator}_${JSON.stringify(condition.value)}`;
      if (seen.has(key)) {
        return false;
      }
      seen.add(key);
      return true;
    });

    // Sort conditions by selectivity (most selective first)
    return filtered.sort((a, b) => {
      const selectivityA = this.getConditionSelectivity(a);
      const selectivityB = this.getConditionSelectivity(b);
      return selectivityA - selectivityB;
    });
  }

  private static getConditionSelectivity(condition: QueryCondition): number {
    // Lower numbers = more selective (better to evaluate first)
    switch (condition.operator) {
      case '=':
        return 1;
      case 'in':
        return Array.isArray(condition.value) ? condition.value.length : 10;
      case '>':
      case '<':
      case '>=':
      case '<=':
        return 50;
      case 'like':
      case 'ilike':
        return 75;
      case 'is_not_null':
        return 90;
      default:
        return 100;
    }
  }

  static shouldUseIndex(field: string, operator: string, model: typeof BaseModel): boolean {
    // Check if field has index configuration
    const fieldConfig = model.fields?.get(field);
    if (fieldConfig?.index) {
      return true;
    }

    // Certain operators benefit from indexes
    const indexBeneficialOps = ['=', 'in', '>', '<', '>=', '<=', 'between'];
    return indexBeneficialOps.includes(operator);
  }

  static estimateResultSize(query: QueryBuilder<any>): number {
    const conditions = query.getConditions();
    const limit = query.getLimit();

    // If there's a limit, that's our upper bound
    if (limit) {
      return limit;
    }

    // Estimate based on conditions
    let estimate = 1000; // Base estimate

    for (const condition of conditions) {
      switch (condition.operator) {
        case '=':
          estimate *= 0.1; // Very selective
          break;
        case 'in':
          estimate *= Array.isArray(condition.value) ? condition.value.length * 0.1 : 0.1;
          break;
        case '>':
        case '<':
        case '>=':
        case '<=':
          estimate *= 0.5; // Moderately selective
          break;
        case 'like':
          estimate *= 0.3; // Somewhat selective
          break;
        default:
          estimate *= 0.8;
      }
    }

    return Math.max(1, Math.round(estimate));
  }

  static suggestOptimizations<T extends BaseModel>(query: QueryBuilder<T>): string[] {
    const suggestions: string[] = [];
    const conditions = query.getConditions();
    const model = query.getModel();
    const limit = query.getLimit();

    // Check for missing userId in user-scoped queries
    if (model.scope === 'user') {
      const hasUserFilter = conditions.some((c) => c.field === 'userId' || c.operator === 'userIn');
      if (!hasUserFilter) {
        suggestions.push('Add userId filter to avoid expensive global index query');
      }
    }

    // Check for missing limit on potentially large result sets
    if (!limit) {
      const estimatedSize = this.estimateResultSize(query);
      if (estimatedSize > 100) {
        suggestions.push('Add limit() to prevent large result sets');
      }
    }

    // Check for unindexed field queries
    for (const condition of conditions) {
      if (!this.shouldUseIndex(condition.field, condition.operator, model)) {
        suggestions.push(`Consider adding index for field: ${condition.field}`);
      }
    }

    // Check for expensive operations
    const expensiveOps = conditions.filter((c) =>
      ['like', 'ilike', 'array_contains'].includes(c.operator),
    );
    if (expensiveOps.length > 0) {
      suggestions.push('Consider using more selective filters before expensive operations');
    }

    // Check for OR conditions
    const orConditions = conditions.filter((c) => c.operator === 'or');
    if (orConditions.length > 0) {
      suggestions.push('OR conditions can be expensive, consider restructuring query');
    }

    return suggestions;
  }
}
