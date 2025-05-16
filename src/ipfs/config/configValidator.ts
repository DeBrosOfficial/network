import { config } from '../../config';

export interface ValidationResult {
  valid: boolean;
  errors: string[];
}

/**
 * Validates the IPFS configuration
 */
export const validateConfig = (): ValidationResult => {
  const errors: string[] = [];

  // Check fingerprint
  if (!config.env.fingerprint || config.env.fingerprint === 'default-fingerprint') {
    errors.push('Fingerprint not set or using default value. Please set a unique fingerprint.');
  }

  // Check port
  const port = Number(config.env.port);
  if (isNaN(port) || port < 1 || port > 65535) {
    errors.push('Invalid port configuration. Port must be a number between 1 and 65535.');
  }

  // Check service discovery topic
  if (!config.ipfs.serviceDiscovery.topic) {
    errors.push('Service discovery topic not configured.');
  }

  // Check blockstore path
  if (!config.ipfs.blockstorePath) {
    errors.push('Blockstore path not configured.');
  }

  // Check orbitdb directory
  if (!config.orbitdb.directory) {
    errors.push('OrbitDB directory not configured.');
  }

  return {
    valid: errors.length === 0,
    errors,
  };
};
