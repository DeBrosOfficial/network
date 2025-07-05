// Mock OrbitDB for testing
export class MockOrbitDB {
  private databases = new Map<string, MockDatabase>();
  private isOpen = false;

  async open(name: string, options: any = {}) {
    if (!this.databases.has(name)) {
      this.databases.set(name, new MockDatabase(name, options));
    }
    return this.databases.get(name);
  }

  async stop() {
    this.isOpen = false;
    for (const db of this.databases.values()) {
      await db.close();
    }
  }

  async start() {
    this.isOpen = true;
  }

  get address() {
    return 'mock-orbitdb-address';
  }
}

export class MockDatabase {
  private data = new Map<string, any>();
  private _events: Array<{ type: string; payload: any }> = [];
  public name: string;
  public type: string;
  
  constructor(name: string, options: any = {}) {
    this.name = name;
    this.type = options.type || 'docstore';
  }

  // DocStore methods
  async put(doc: any, options?: any) {
    const id = doc._id || doc.id || this.generateId();
    const record = { ...doc, _id: id };
    this.data.set(id, record);
    this._events.push({ type: 'write', payload: record });
    return id;
  }

  async get(id: string) {
    return this.data.get(id) || null;
  }

  async del(id: string) {
    const deleted = this.data.delete(id);
    if (deleted) {
      this._events.push({ type: 'delete', payload: { _id: id } });
    }
    return deleted;
  }

  async query(filter?: (doc: any) => boolean) {
    const docs = Array.from(this.data.values());
    return filter ? docs.filter(filter) : docs;
  }

  async all() {
    return Array.from(this.data.values());
  }

  // EventLog methods
  async add(data: any) {
    const entry = {
      payload: data,
      hash: this.generateId(),
      clock: { time: Date.now() }
    };
    this._events.push(entry);
    return entry.hash;
  }

  async iterator(options?: any) {
    const events = this._events.slice();
    return {
      collect: () => events,
      [Symbol.iterator]: function* () {
        for (const event of events) {
          yield event;
        }
      }
    };
  }

  // KeyValue methods
  async set(key: string, value: any) {
    this.data.set(key, value);
    this._events.push({ type: 'put', payload: { key, value } });
    return key;
  }

  // Counter methods
  async inc(amount: number = 1) {
    const current = this.data.get('counter') || 0;
    const newValue = current + amount;
    this.data.set('counter', newValue);
    this._events.push({ type: 'increment', payload: { amount, value: newValue } });
    return newValue;
  }

  get value() {
    return this.data.get('counter') || 0;
  }

  // General methods
  async close() {
    // Mock close
  }

  async drop() {
    this.data.clear();
    this._events = [];
  }

  get address() {
    return `mock-db-${this.name}`;
  }

  get events() {
    return this._events;
  }

  // Event emitter mock
  on(event: string, callback: Function) {
    // Mock event listener
  }

  off(event: string, callback: Function) {
    // Mock event listener removal
  }

  private generateId(): string {
    return `mock-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
  }
}

export const createOrbitDB = jest.fn(async (options: any) => {
  return new MockOrbitDB();
});

// Default export for ES modules
export default {
  createOrbitDB,
  MockOrbitDB,
  MockDatabase
};