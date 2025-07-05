export interface QueryCondition {
  field: string;
  operator: string;
  value: any;
  logical?: 'and' | 'or';
  type?: 'condition' | 'group';
  conditions?: QueryCondition[];
}

export interface SortConfig {
  field: string;
  direction: 'asc' | 'desc';
}

export interface QueryOptions {
  limit?: number;
  offset?: number;
  relations?: string[];
}
