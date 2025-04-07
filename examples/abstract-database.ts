import { initDB, create, get, update, remove, list, query, uploadFile, getFile, deleteFile, stopDB, logger } from '../src';

// Alternative import method
// import debros from '../src';
// const { db } = debros;

async function databaseExample() {
  try {
    logger.info('Starting database example...');
    
    // Initialize the database service (abstracts away IPFS and OrbitDB)
    await initDB();
    logger.info('Database service initialized');
    
    // Create a new user document
    const userId = 'user123';
    const userData = {
      username: 'johndoe',
      walletAddress: '0x1234567890',
      avatar: null
    };
    
    const createResult = await create('users', userId, userData);
    logger.info(`Created user with ID: ${createResult.id} and hash: ${createResult.hash}`);
    
    // Retrieve the user
    const user = await get('users', userId);
    logger.info('Retrieved user:', user);
    
    // Update the user
    const updateResult = await update('users', userId, {
      avatar: 'profile.jpg',
      bio: 'Software developer'
    });
    logger.info(`Updated user with hash: ${updateResult.hash}`);
    
    // Query users
    const filteredUsers = await query('users', (user) => user.username === 'johndoe');
    logger.info(`Found ${filteredUsers.length} matching users`);
    
    // List all users
    const allUsers = await list('users', { limit: 10 });
    logger.info(`Retrieved ${allUsers.length} users`);
    
    // Upload a file
    const fileData = Buffer.from('This is a test file content');
    const fileUpload = await uploadFile(fileData, { filename: 'test.txt' });
    logger.info(`Uploaded file with CID: ${fileUpload.cid}`);
    
    // Retrieve the file
    const file = await getFile(fileUpload.cid);
    logger.info('Retrieved file:', {
      content: file.data.toString(),
      metadata: file.metadata
    });
    
    // Delete the file
    await deleteFile(fileUpload.cid);
    logger.info('File deleted');
    
    // Delete the user
    await remove('users', userId);
    logger.info('User deleted');
    
    // Stop the database service
    await stopDB();
    logger.info('Database service stopped');
  } catch (error) {
    logger.error('Error in database example:', error);
  }
}

databaseExample();