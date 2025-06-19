export interface QueryCondition {
  field: string;
  operator: string;
  value: any;
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
