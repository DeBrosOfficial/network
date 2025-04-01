import { createLogger, format, transports } from 'winston';
import fs from 'fs';
import path from 'path';

// Define logger options interface
export interface LoggerOptions {
  logsDir?: string;
  level?: string;
  disableConsole?: boolean;
  disableFile?: boolean;
}

// Define colors for different service types
const colors: Record<string, string> = {
  error: '\x1b[31m', // red
  warn: '\x1b[33m', // yellow
  info: '\x1b[32m', // green
  debug: '\x1b[36m', // cyan
  reset: '\x1b[0m', // reset

  // Service specific colors
  IPFS: '\x1b[36m', // cyan
  HEARTBEAT: '\x1b[33m', // yellow
  SOCKET: '\x1b[34m', // blue
  'LOAD-BALANCER': '\x1b[35m', // magenta
  DEFAULT: '\x1b[37m', // white
};

// Create a customizable logger factory
export function createDebrosLogger(options: LoggerOptions = {}) {
  // Set default options
  const logsDir = options.logsDir || path.join(process.cwd(), 'logs');
  const logLevel = options.level || process.env.LOG_LEVEL || 'info';
  
  // Create logs directory if it doesn't exist
  if (!fs.existsSync(logsDir) && !options.disableFile) {
    fs.mkdirSync(logsDir, { recursive: true });
  }

  // Custom format for console output with colors
  const customConsoleFormat = format.printf(({ level, message, timestamp, service }: any) => {
    // Truncate error messages
    if (level === 'error' && typeof message === 'string' && message.length > 300) {
      message = message.substring(0, 300) + '... [truncated]';
    }

    // Handle objects and errors
    if (typeof message === 'object' && message !== null) {
      if (message instanceof Error) {
        message = message.message;
        // Truncate error messages
        if (message.length > 300) {
          message = message.substring(0, 300) + '... [truncated]';
        }
      } else {
        try {
          message = JSON.stringify(message, null, 2);
        } catch (e) {
          message = '[Object]';
        }
      }
    }

    const serviceColor = service && colors[service] ? colors[service] : colors.DEFAULT;
    const levelColor = colors[level] || colors.DEFAULT;
    const serviceTag = service ? `[${service}]` : '';

    return `${timestamp} ${levelColor}${level}${colors.reset}: ${serviceColor}${serviceTag}${colors.reset} ${message}`;
  });

  // Custom format for file output (no colors)
  const customFileFormat = format.printf(({ level, message, timestamp, service }) => {
    // Handle objects and errors
    if (typeof message === 'object' && message !== null) {
      if (message instanceof Error) {
        message = message.message;
      } else {
        try {
          message = JSON.stringify(message);
        } catch (e) {
          message = '[Object]';
        }
      }
    }

    const serviceTag = service ? `[${service}]` : '';
    return `${timestamp} ${level}: ${serviceTag} ${message}`;
  });

  // Configure transports
  const loggerTransports = [];
  
  // Add console transport if not disabled
  if (!options.disableConsole) {
    loggerTransports.push(
      new transports.Console({
        format: format.combine(format.timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }), customConsoleFormat),
      })
    );
  }
  
  // Add file transports if not disabled
  if (!options.disableFile) {
    loggerTransports.push(
      // Combined log file
      new transports.File({
        filename: path.join(logsDir, 'app.log'),
        format: format.combine(format.timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }), customFileFormat),
      }),
      // Error log file
      new transports.File({
        filename: path.join(logsDir, 'error.log'),
        level: 'error',
        format: format.combine(format.timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }), customFileFormat),
      })
    );
  }

  // Create the logger
  const logger = createLogger({
    level: logLevel,
    format: format.combine(format.timestamp({ format: 'YYYY-MM-DD HH:mm:ss' }), format.splat()),
    defaultMeta: { service: 'DEFAULT' },
    transports: loggerTransports,
  });

  // Helper function to create a logger for a specific service
  const createServiceLogger = (serviceName: string) => {
    return {
      error: (message: any, ...meta: any[]) => logger.error(message, { service: serviceName, ...meta }),
      warn: (message: any, ...meta: any[]) => logger.warn(message, { service: serviceName, ...meta }),
      info: (message: any, ...meta: any[]) => logger.info(message, { service: serviceName, ...meta }),
      debug: (message: any, ...meta: any[]) => logger.debug(message, { service: serviceName, ...meta }),
    };
  };

  return {
    logger,
    createServiceLogger,
  };
}

// Create a default logger instance
const { logger, createServiceLogger } = createDebrosLogger();

export { logger, createServiceLogger };
export default logger;