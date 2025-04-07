import { createServiceLogger } from '../../utils/logger';
import { ErrorCode, StoreType, FileUploadResult, FileResult } from '../types';
import { DBError } from '../core/error';
import { getConnection } from '../core/connection';
import { openStore } from './baseStore';
import { getHelia } from '../../ipfs/ipfsService';
import { measurePerformance } from '../metrics/metricsService';

// Helper function to convert AsyncIterable to Buffer
async function readAsyncIterableToBuffer(asyncIterable: AsyncIterable<Uint8Array>): Promise<Buffer> {
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
  }
): Promise<FileUploadResult> => {
  return measurePerformance(async () => {
    try {
      const connection = getConnection(options?.connectionId);
      const ipfs = getHelia();
      if (!ipfs) {
        throw new DBError(ErrorCode.OPERATION_FAILED, 'IPFS instance not available');
      }
      
      // Add to IPFS
      const blockstore = ipfs.blockstore;
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
        ...options?.metadata
      });
      
      logger.info(`Uploaded file with CID: ${cidStr}`);
      return { cid: cidStr };
    } catch (error) {
      if (error instanceof DBError) {
        throw error;
      }
      
      logger.error('Error uploading file:', error);
      throw new DBError(ErrorCode.OPERATION_FAILED, 'Failed to upload file', error);
    }
  });
};

/**
 * Get a file from IPFS by CID
 */
export const getFile = async (
  cid: string,
  options?: { connectionId?: string }
): Promise<FileResult> => {
  return measurePerformance(async () => {
    try {
      const connection = getConnection(options?.connectionId);
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
        } catch (err) {
          // Metadata might not exist, continue without it
        }
        
        return { data: bytes, metadata };
      } catch (error) {
        throw new DBError(ErrorCode.FILE_NOT_FOUND, `File with CID ${cid} not found`, error);
      }
    } catch (error) {
      if (error instanceof DBError) {
        throw error;
      }
      
      logger.error(`Error getting file with CID ${cid}:`, error);
      throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to get file with CID ${cid}`, error);
    }
  });
};

/**
 * Delete a file from IPFS by CID
 */
export const deleteFile = async (
  cid: string,
  options?: { connectionId?: string }
): Promise<boolean> => {
  return measurePerformance(async () => {
    try {
      const connection = getConnection(options?.connectionId);
      
      // Delete metadata
      try {
        const filesDb = await openStore('_files', StoreType.KEYVALUE);
        await filesDb.del(cid);
      } catch (err) {
        // Ignore if metadata doesn't exist
      }
      
      // In IPFS we can't really delete files, but we can remove them from our local blockstore
      // and they will eventually be garbage collected if no one else has pinned them
      logger.info(`Deleted file with CID: ${cid}`);
      return true;
    } catch (error) {
      if (error instanceof DBError) {
        throw error;
      }
      
      logger.error(`Error deleting file with CID ${cid}:`, error);
      throw new DBError(ErrorCode.OPERATION_FAILED, `Failed to delete file with CID ${cid}`, error);
    }
  });
};