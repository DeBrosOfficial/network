import { createServiceLogger } from '../../utils/logger';
import { ErrorCode, StoreType, FileUploadResult, FileResult } from '../types';
import { DBError } from '../core/error';
import { openStore } from './baseStore';
import ipfsService, { getHelia } from '../../ipfs/ipfsService';
import { CreateResult, StoreOptions } from '../types';

async function readAsyncIterableToBuffer(
  asyncIterable: AsyncIterable<Uint8Array>,
): Promise<Buffer> {
  const chunks: Uint8Array[] = [];
  for await (const chunk of asyncIterable) {
    chunks.push(chunk);
  }
  return Buffer.concat(chunks);
}

const logger = createServiceLogger('FILE_STORE');

/**
 * Upload a file to IPFS
 */
export const uploadFile = async (
  fileData: Buffer,
  options?: {
    filename?: string;
    connectionId?: string;
    metadata?: Record<string, any>;
  },
): Promise<FileUploadResult> => {
  try {
    const ipfs = getHelia();
    if (!ipfs) {
      logger.error('IPFS instance not available - Helia is null or undefined');
      // Try to check if IPFS service is running
      try {
        const heliaInstance = ipfsService.getHelia();
        logger.error(
          'IPFS Service getHelia() returned:',
          heliaInstance ? 'instance available' : 'null/undefined',
        );
      } catch (importError) {
        logger.error('Error importing IPFS service:', importError);
      }
      throw new DBError(ErrorCode.OPERATION_FAILED, 'IPFS instance not available');
    }

    logger.info(`Attempting to upload file with size: ${fileData.length} bytes`);

    // Add to IPFS
    const unixfs = await import('@helia/unixfs');
    const fs = unixfs.unixfs(ipfs);
    const cid = await fs.addBytes(fileData);
    const cidStr = cid.toString();

    // Store metadata
    const filesDb = await openStore('_files', StoreType.KEYVALUE);
    await filesDb.put(cidStr, {
      filename: options?.filename,
      size: fileData.length,
      uploadedAt: Date.now(),
      ...options?.metadata,
    });

    logger.info(`Uploaded file with CID: ${cidStr}`);
    return { cid: cidStr };
  } catch (error: unknown) {
    if (error instanceof DBError) {
      throw error;
    }

    logger.error('Error uploading file:', error);
    throw new DBError(ErrorCode.OPERATION_FAILED, 'Failed to upload file', error);
  }
};

/**
 * Get a file from IPFS by CID
 */
export const getFile = async (cid: string): Promise<FileResult> => {
  try {
    const ipfs = getHelia();
    if (!ipfs) {
      throw new DBError(ErrorCode.OPERATION_FAILED, 'IPFS instance not available');
    }

    // Get from IPFS
    const unixfs = await import('@helia/unixfs');
    const fs = unixfs.unixfs(ipfs);
    const { CID } = await import('multiformats/cid');
    const resolvedCid = CID.parse(cid);

    try {
      // Convert AsyncIterable to Buffer
      const bytes = await readAsyncIterableToBuffer(fs.cat(resolvedCid));

      // Get metadata if available
      let metadata = null;
      try {
        const filesDb = await openStore('_files', StoreType.KEYVALUE);
        metadata = await filesDb.get(cid);
      } catch (_err) {
        // Metadata might not exist, continue without it
      }

      return { data: bytes, metadata };
    } catch (error) {
      throw new DBError(ErrorCode.FILE_NOT_FOUND, `File with CID ${cid} not found`, error);
    }
  } catch (error: unknown) {
    if (error instanceof DBError) {
      throw error;
    }

    logger.error(`Error getting file with CID ${cid}:`, error);
    throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to get file with CID ${cid}`, error);
  }
};

/**
 * Delete a file from IPFS by CID
 */
export const deleteFile = async (cid: string): Promise<boolean> => {
  try {
    // Delete metadata
    try {
      const filesDb = await openStore('_files', StoreType.KEYVALUE);
      await filesDb.del(cid);
    } catch (_err) {
      // Ignore if metadata doesn't exist
    }

    logger.info(`Deleted file with CID: ${cid}`);
    return true;
  } catch (error: unknown) {
    if (error instanceof DBError) {
      throw error;
    }

    logger.error(`Error deleting file with CID ${cid}:`, error);
    throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to delete file with CID ${cid}`, error);
  }
};

export const create = async <T extends Record<string, any>>(
  collection: string,
  id: string,
  data: Omit<T, 'createdAt' | 'updatedAt'>,
  options?: StoreOptions,
): Promise<CreateResult> => {
  try {
    const db = await openStore(collection, StoreType.KEYVALUE, options);

    // Prepare document for storage with ID
    // const document = {
    //   id,
    //   ...prepareDocument<T>(collection, data)
    // };
    const document = { id, ...data };

    // Add to database
    const hash = await db.add(document);

    // Emit change event
    // events.emit('document:created', { collection, id, document, hash });

    logger.info(`Created entry in file ${collection} with id ${id} and hash ${hash}`);
    return { id, hash };
  } catch (error: unknown) {
    if (error instanceof DBError) {
      throw error;
    }

    logger.error(`Error creating entry in file ${collection}:`, error);
    throw new DBError(
      ErrorCode.OPERATION_FAILED,
      `Failed to create entry in file ${collection}`,
      error,
    );
  }
};
