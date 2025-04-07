import { 
  initDB, 
  create, 
  get, 
  update, 
  remove,
  list, 
  query, 
  uploadFile, 
  getFile, 
  createTransaction,
  commitTransaction,
  subscribe, 
  defineSchema, 
  getMetrics, 
  createIndex,
  ErrorCode,
  StoreType,
  logger 
} from '../src';

// Define a user schema
const userSchema = {
  properties: {
    username: {
      type: 'string',
      required: true,
      min: 3,
      max: 20,
    },
    email: {
      type: 'string',
      pattern: '^[\\w-\\.]+@([\\w-]+\\.)+[\\w-]{2,4}$',
    },
    age: {
      type: 'number',
      min: 18,
      max: 120,
    },
    roles: {
      type: 'array',
      items: {
        type: 'string',
      },
    },
    isActive: {
      type: 'boolean',
    },
  },
  required: ['username'],
};

async function advancedDatabaseExample() {
  try {
    logger.info('Starting advanced database example...');
    
    // Initialize the database service with a specific connection ID
    const connectionId = await initDB('example-connection');
    logger.info(`Database service initialized with connection ID: ${connectionId}`);
    
    // Define schema for validation
    defineSchema('users', userSchema);
    logger.info('User schema defined');
    
    // Create index for faster queries (works with docstore)
    await createIndex('users', 'username', { storeType: StoreType.DOCSTORE });
    logger.info('Created index on username field');
    
    // Set up subscription for real-time updates
    const unsubscribe = subscribe('document:created', (data) => {
      logger.info('Document created:', data.collection, data.id);
    });
    
    // Create multiple users using a transaction
    const transaction = createTransaction(connectionId);
    
    transaction
      .create('users', 'user1', {
        username: 'alice',
        email: 'alice@example.com',
        age: 28,
        roles: ['admin', 'user'],
        isActive: true,
      })
      .create('users', 'user2', {
        username: 'bob',
        email: 'bob@example.com',
        age: 34,
        roles: ['user'],
        isActive: true,
      })
      .create('users', 'user3', {
        username: 'charlie',
        email: 'charlie@example.com',
        age: 45,
        roles: ['moderator', 'user'],
        isActive: false,
      });
    
    const txResult = await commitTransaction(transaction);
    logger.info(`Transaction committed with ${txResult.results.length} operations`);
    
    // Get a specific user with type safety
    interface User {
      username: string;
      email: string;
      age: number;
      roles: string[];
      isActive: boolean;
      createdAt: number;
      updatedAt: number;
    }
    
    // Using KeyValue store (default)
    const user = await get<User>('users', 'user1');
    logger.info('Retrieved user from KeyValue store:', user?.username, user?.email);
    
    // Using DocStore
    await create<User>(
      'users_docstore',
      'user1',
      {
        username: 'alice_doc',
        email: 'alice_doc@example.com',
        age: 28,
        roles: ['admin', 'user'],
        isActive: true,
      },
      { storeType: StoreType.DOCSTORE }
    );
    
    const docStoreUser = await get<User>('users_docstore', 'user1', { storeType: StoreType.DOCSTORE });
    logger.info('Retrieved user from DocStore:', docStoreUser?.username, docStoreUser?.email);
    
    // Using Feed/EventLog store
    await create<User>(
      'users_feed',
      'user1',
      {
        username: 'alice_feed',
        email: 'alice_feed@example.com',
        age: 28,
        roles: ['admin', 'user'],
        isActive: true,
      },
      { storeType: StoreType.FEED }
    );
    
    // Update the feed entry (creates a new entry with the same ID)
    await update<User>(
      'users_feed',
      'user1',
      {
        roles: ['admin', 'user', 'tester'],
      },
      { storeType: StoreType.FEED }
    );
    
    // List all entries in the feed
    const feedUsers = await list<User>('users_feed', { storeType: StoreType.FEED });
    logger.info(`Found ${feedUsers.total} feed entries:`);
    feedUsers.documents.forEach(user => {
      logger.info(`- ${user.username} (${user.email})`);
    });
    
    // Using Counter store
    await create(
      'counters', 
      'visitors', 
      { value: 100 }, 
      { storeType: StoreType.COUNTER }
    );
    
    // Increment the counter
    await update(
      'counters',
      'visitors',
      { increment: 5 },
      { storeType: StoreType.COUNTER }
    );
    
    // Get the counter value
    const counter = await get('counters', 'visitors', { storeType: StoreType.COUNTER });
    logger.info(`Counter value: ${counter?.value}`);
    
    // Update a user in KeyValue store
    await update<User>('users', 'user1', {
      roles: ['admin', 'user', 'tester'],
    });
    
    // Query users from KeyValue store
    const result = await query<User>(
      'users',
      (user) => user.isActive === true,
      {
        limit: 10,
        offset: 0,
        sort: { field: 'username', order: 'asc' },
      }
    );
    
    logger.info(`Found ${result.total} active users:`);
    result.documents.forEach(user => {
      logger.info(`- ${user.username} (${user.email})`);
    });
    
    // List all users from KeyValue store with pagination
    const allUsers = await list<User>('users', {
      limit: 10,
      offset: 0,
      sort: { field: 'age', order: 'desc' },
    });
    
    logger.info(`Listed ${allUsers.total} users sorted by age (desc):`);
    allUsers.documents.forEach(user => {
      logger.info(`- ${user.username}: ${user.age} years old`);
    });
    
    // Upload a file with metadata
    const fileData = Buffer.from('This is a test file with advanced features.');
    const fileUpload = await uploadFile(fileData, { 
      filename: 'advanced-test.txt',
      metadata: {
        contentType: 'text/plain',
        tags: ['example', 'test'],
        owner: 'user1',
      }
    });
    
    logger.info(`Uploaded file with CID: ${fileUpload.cid}`);
    
    // Retrieve the file
    const file = await getFile(fileUpload.cid);
    logger.info('Retrieved file content:', file.data.toString());
    logger.info('File metadata:', file.metadata);
    
    // Get performance metrics
    const metrics = getMetrics();
    logger.info('Database metrics:', {
      operations: metrics.operations,
      performance: {
        averageOperationTime: `${metrics.performance.averageOperationTime?.toFixed(2)}ms`,
      },
      cache: metrics.cacheStats,
    });
    
    // Unsubscribe from events
    unsubscribe();
    logger.info('Unsubscribed from events');
    
    // Delete a user
    await remove('users', 'user3');
    logger.info('Deleted user user3');
    
    return true;
  } catch (error) {
    if (error && typeof error === 'object' && 'code' in error) {
      logger.error(`Database error (${error.code}):`, error.message);
    } else {
      logger.error('Error in advanced database example:', error);
    }
    return false;
  }
}

advancedDatabaseExample();