import { generateKeyPairFromSeed } from '@libp2p/crypto/keys';
import forge from 'node-forge';
import { config } from '../../config';
import { createServiceLogger } from '../../utils/logger';

const logger = createServiceLogger('CRYPTO');

/**
 * Generates a deterministic private key based on the node's fingerprint
 */
export const getPrivateKey = async () => {
  try {
    const userInput = config.env.fingerprint;

    // Use SHA-256 to create a deterministic seed
    const md = forge.md.sha256.create();
    md.update(userInput);
    const seedString = md.digest().getBytes(); // Get raw bytes as a string

    // Convert the seed string to Uint8Array
    const seed = Uint8Array.from(forge.util.binary.raw.decode(seedString));

    // Generate an Ed25519 private key (libp2p-compatible)
    const privateKey = await generateKeyPairFromSeed('Ed25519', seed);
    return privateKey;
  } catch (error) {
    logger.error('Error generating private key:', error);
    throw error;
  }
};