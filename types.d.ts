// Type definitions for @debros/network
// Project: https://github.com/debros/anchat-relay
// Definitions by: Debros Team

declare module "@debros/network" {
  import { Request, Response, NextFunction } from "express";

  // Config types
  export interface DebrosConfig {
    ipfs: {
      swarm: {
        port: number;
        announceAddresses: string[];
        listenAddresses: string[];
        connectAddresses: string[];
      };
      blockstorePath: string;
      orbitdbPath: string;
      bootstrap: string[];
      privateKey?: string;
    };
    logger: {
      level: string;
      file?: string;
    };
  }

  export interface ValidationResult {
    valid: boolean;
    errors?: string[];
  }

  // Core configuration
  export const config: DebrosConfig;
  export const defaultConfig: DebrosConfig;
  export function validateConfig(
    config: Partial<DebrosConfig>
  ): ValidationResult;

  // IPFS types
  export interface IPFSModule {
    helia: any;
    libp2p: any;
  }

  // IPFS Service
  export const ipfsService: {
    init(): Promise<IPFSModule>;
    stop(): Promise<void>;
  };
  export function initIpfs(): Promise<IPFSModule>;
  export function stopIpfs(): Promise<void>;
  export function getHelia(): any;
  export function getProxyAgent(): any;
  export function getInstance(): IPFSModule;
  export function getLibp2p(): any;
  export function getConnectedPeers(): any[];
  export function getOptimalPeer(): any;
  export function updateNodeLoad(load: number): void;
  export function logPeersStatus(): void;

  // IPFS Config
  export const ipfsConfig: any;
  export function getIpfsPort(): number;
  export function getBlockstorePath(): string;

  // LoadBalancerController interface and value declaration
  export interface LoadBalancerController {
    getNodeInfo: (_req: Request, _res: Response, _next: NextFunction) => void;
    getOptimalPeer: (
      _req: Request,
      _res: Response,
      _next: NextFunction
    ) => void;
    getAllPeers: (_req: Request, _res: Response, _next: NextFunction) => void;
  }

  // Declare loadBalancerController as a value
  export const loadBalancerController: LoadBalancerController;

  // OrbitDB
  export const orbitDBService: {
    init(): Promise<any>;
  };
  export function initOrbitDB(): Promise<any>;
  export function openDB(
    dbName: string,
    dbType: string,
    options?: any
  ): Promise<any>;
  export function getOrbitDB(): any;
  export const orbitDB: any;
  export function getOrbitDBDir(): string;
  export function getDBAddress(dbName: string): string | null;
  export function saveDBAddress(dbName: string, address: string): void;

  // Logger
  export interface LoggerOptions {
    level?: string;
    file?: string;
    service?: string;
  }
  export const logger: any;
  export function createServiceLogger(
    name: string,
    options?: LoggerOptions
  ): any;
  export function createDebrosLogger(options?: LoggerOptions): any;

  // Crypto
  export function getPrivateKey(): Promise<string>;

  // Default export
  const defaultExport: {
    config: DebrosConfig;
    validateConfig: typeof validateConfig;
    ipfsService: typeof ipfsService;
    orbitDBService: typeof orbitDBService;
    logger: any;
    createServiceLogger: typeof createServiceLogger;
  };
  export default defaultExport;
}
