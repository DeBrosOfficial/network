/**
 * PubSubManager - Automatic Event Publishing and Subscription
 *
 * This class handles automatic publishing of model changes and database events
 * to IPFS PubSub topics, enabling real-time synchronization across nodes:
 * - Model-level events (create, update, delete)
 * - Database-level events (replication, sync)
 * - Custom application events
 * - Topic management and subscription handling
 * - Event filtering and routing
 */

import { BaseModel } from '../models/BaseModel';

// Node.js types for compatibility
declare global {
  namespace NodeJS {
    interface Timeout {}
  }
}

export interface PubSubConfig {
  enabled: boolean;
  autoPublishModelEvents: boolean;
  autoPublishDatabaseEvents: boolean;
  topicPrefix: string;
  maxRetries: number;
  retryDelay: number;
  eventBuffer: {
    enabled: boolean;
    maxSize: number;
    flushInterval: number;
  };
  compression: {
    enabled: boolean;
    threshold: number; // bytes
  };
  encryption: {
    enabled: boolean;
    publicKey?: string;
    privateKey?: string;
  };
}

export interface PubSubEvent {
  id: string;
  type: string;
  topic: string;
  data: any;
  timestamp: number;
  source: string;
  metadata?: any;
}

export interface TopicSubscription {
  topic: string;
  handler: (event: PubSubEvent) => void | Promise<void>;
  filter?: (event: PubSubEvent) => boolean;
  options: {
    autoAck: boolean;
    maxRetries: number;
    deadLetterTopic?: string;
  };
}

export interface PubSubStats {
  totalPublished: number;
  totalReceived: number;
  totalSubscriptions: number;
  publishErrors: number;
  receiveErrors: number;
  averageLatency: number;
  topicStats: Map<
    string,
    {
      published: number;
      received: number;
      subscribers: number;
      lastActivity: number;
    }
  >;
}

export class PubSubManager {
  private ipfsService: any;
  private config: PubSubConfig;
  private subscriptions: Map<string, TopicSubscription[]> = new Map();
  private eventBuffer: PubSubEvent[] = [];
  private bufferFlushInterval: any = null;
  private stats: PubSubStats;
  private latencyMeasurements: number[] = [];
  private nodeId: string;
  private isInitialized: boolean = false;
  private eventListeners: Map<string, Function[]> = new Map();

  constructor(ipfsService: any, config: Partial<PubSubConfig> = {}) {
    this.ipfsService = ipfsService;
    this.nodeId = `node-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

    this.config = {
      enabled: true,
      autoPublishModelEvents: true,
      autoPublishDatabaseEvents: true,
      topicPrefix: 'debros',
      maxRetries: 3,
      retryDelay: 1000,
      eventBuffer: {
        enabled: true,
        maxSize: 100,
        flushInterval: 5000,
      },
      compression: {
        enabled: true,
        threshold: 1024,
      },
      encryption: {
        enabled: false,
      },
      ...config,
    };

    this.stats = {
      totalPublished: 0,
      totalReceived: 0,
      totalSubscriptions: 0,
      publishErrors: 0,
      receiveErrors: 0,
      averageLatency: 0,
      topicStats: new Map(),
    };
  }

  // Simple event emitter functionality
  emit(event: string, ...args: any[]): boolean {
    const listeners = this.eventListeners.get(event) || [];
    listeners.forEach((listener) => {
      try {
        listener(...args);
      } catch (error) {
        console.error(`Error in event listener for ${event}:`, error);
      }
    });
    return listeners.length > 0;
  }

  on(event: string, listener: Function): this {
    if (!this.eventListeners.has(event)) {
      this.eventListeners.set(event, []);
    }
    this.eventListeners.get(event)!.push(listener);
    return this;
  }

  off(event: string, listener?: Function): this {
    if (!listener) {
      this.eventListeners.delete(event);
    } else {
      const listeners = this.eventListeners.get(event) || [];
      const index = listeners.indexOf(listener);
      if (index >= 0) {
        listeners.splice(index, 1);
      }
    }
    return this;
  }

  // Initialize PubSub system
  async initialize(): Promise<void> {
    if (!this.config.enabled) {
      console.log('📡 PubSub disabled in configuration');
      return;
    }

    try {
      console.log('📡 Initializing PubSubManager...');

      // Start event buffer flushing if enabled
      if (this.config.eventBuffer.enabled) {
        this.startEventBuffering();
      }

      // Subscribe to model events if auto-publishing is enabled
      if (this.config.autoPublishModelEvents) {
        this.setupModelEventPublishing();
      }

      // Subscribe to database events if auto-publishing is enabled
      if (this.config.autoPublishDatabaseEvents) {
        this.setupDatabaseEventPublishing();
      }

      this.isInitialized = true;
      console.log('✅ PubSubManager initialized successfully');
    } catch (error) {
      console.error('❌ Failed to initialize PubSubManager:', error);
      throw error;
    }
  }

  // Publish event to a topic
  async publish(
    topic: string,
    data: any,
    options: {
      priority?: 'low' | 'normal' | 'high';
      retries?: number;
      compress?: boolean;
      encrypt?: boolean;
      metadata?: any;
    } = {},
  ): Promise<boolean> {
    if (!this.config.enabled || !this.isInitialized) {
      return false;
    }

    const event: PubSubEvent = {
      id: this.generateEventId(),
      type: this.extractEventType(topic),
      topic: this.prefixTopic(topic),
      data,
      timestamp: Date.now(),
      source: this.nodeId,
      metadata: options.metadata,
    };

    try {
      // Process event (compression, encryption, etc.)
      const processedData = await this.processEventForPublishing(event, options);

      // Publish with buffering or directly
      if (this.config.eventBuffer.enabled && options.priority !== 'high') {
        return this.bufferEvent(event, processedData);
      } else {
        return await this.publishDirect(event.topic, processedData, options.retries);
      }
    } catch (error) {
      this.stats.publishErrors++;
      console.error(`❌ Failed to publish to ${topic}:`, error);
      this.emit('publishError', { topic, error, event });
      return false;
    }
  }

  // Subscribe to a topic
  async subscribe(
    topic: string,
    handler: (event: PubSubEvent) => void | Promise<void>,
    options: {
      filter?: (event: PubSubEvent) => boolean;
      autoAck?: boolean;
      maxRetries?: number;
      deadLetterTopic?: string;
    } = {},
  ): Promise<boolean> {
    if (!this.config.enabled || !this.isInitialized) {
      return false;
    }

    const fullTopic = this.prefixTopic(topic);

    try {
      const subscription: TopicSubscription = {
        topic: fullTopic,
        handler,
        filter: options.filter,
        options: {
          autoAck: options.autoAck !== false,
          maxRetries: options.maxRetries || this.config.maxRetries,
          deadLetterTopic: options.deadLetterTopic,
        },
      };

      // Add to subscriptions map
      if (!this.subscriptions.has(fullTopic)) {
        this.subscriptions.set(fullTopic, []);

        // Subscribe to IPFS PubSub topic
        await this.ipfsService.pubsub.subscribe(fullTopic, (message: any) => {
          this.handleIncomingMessage(fullTopic, message);
        });
      }

      this.subscriptions.get(fullTopic)!.push(subscription);
      this.stats.totalSubscriptions++;

      // Update topic stats
      this.updateTopicStats(fullTopic, 'subscribers', 1);

      console.log(`📡 Subscribed to topic: ${fullTopic}`);
      this.emit('subscribed', { topic: fullTopic, subscription });

      return true;
    } catch (error) {
      console.error(`❌ Failed to subscribe to ${topic}:`, error);
      this.emit('subscribeError', { topic, error });
      return false;
    }
  }

  // Unsubscribe from a topic
  async unsubscribe(topic: string, handler?: Function): Promise<boolean> {
    const fullTopic = this.prefixTopic(topic);
    const subscriptions = this.subscriptions.get(fullTopic);

    if (!subscriptions) {
      return false;
    }

    try {
      if (handler) {
        // Remove specific handler
        const index = subscriptions.findIndex((sub) => sub.handler === handler);
        if (index >= 0) {
          subscriptions.splice(index, 1);
          this.stats.totalSubscriptions--;
        }
      } else {
        // Remove all handlers for this topic
        this.stats.totalSubscriptions -= subscriptions.length;
        subscriptions.length = 0;
      }

      // If no more subscriptions, unsubscribe from IPFS
      if (subscriptions.length === 0) {
        await this.ipfsService.pubsub.unsubscribe(fullTopic);
        this.subscriptions.delete(fullTopic);
        this.stats.topicStats.delete(fullTopic);
      }

      console.log(`📡 Unsubscribed from topic: ${fullTopic}`);
      this.emit('unsubscribed', { topic: fullTopic });

      return true;
    } catch (error) {
      console.error(`❌ Failed to unsubscribe from ${topic}:`, error);
      return false;
    }
  }

  // Setup automatic model event publishing
  private setupModelEventPublishing(): void {
    const topics = {
      create: 'model.created',
      update: 'model.updated',
      delete: 'model.deleted',
      save: 'model.saved',
    };

    // Listen for model events on the global framework instance
    this.on('modelEvent', async (eventType: string, model: BaseModel, changes?: any) => {
      const topic = topics[eventType as keyof typeof topics];
      if (!topic) return;

      const eventData = {
        modelName: model.constructor.name,
        modelId: model.id,
        userId: (model as any).userId,
        changes,
        timestamp: Date.now(),
      };

      await this.publish(topic, eventData, {
        priority: eventType === 'delete' ? 'high' : 'normal',
        metadata: {
          modelType: model.constructor.name,
          scope: (model.constructor as any).scope,
        },
      });
    });
  }

  // Setup automatic database event publishing
  private setupDatabaseEventPublishing(): void {
    const databaseTopics = {
      replication: 'database.replicated',
      sync: 'database.synced',
      conflict: 'database.conflict',
      error: 'database.error',
    };

    // Listen for database events
    this.on('databaseEvent', async (eventType: string, data: any) => {
      const topic = databaseTopics[eventType as keyof typeof databaseTopics];
      if (!topic) return;

      await this.publish(topic, data, {
        priority: eventType === 'error' ? 'high' : 'normal',
        metadata: {
          eventType,
          source: 'database',
        },
      });
    });
  }

  // Handle incoming PubSub messages
  private async handleIncomingMessage(topic: string, message: any): Promise<void> {
    try {
      const startTime = Date.now();

      // Parse and validate message
      const event = await this.processIncomingMessage(message);
      if (!event) return;

      // Update stats
      this.stats.totalReceived++;
      this.updateTopicStats(topic, 'received', 1);

      // Calculate latency
      const latency = Date.now() - event.timestamp;
      this.latencyMeasurements.push(latency);
      if (this.latencyMeasurements.length > 100) {
        this.latencyMeasurements.shift();
      }
      this.stats.averageLatency =
        this.latencyMeasurements.reduce((a, b) => a + b, 0) / this.latencyMeasurements.length;

      // Route to subscribers
      const subscriptions = this.subscriptions.get(topic) || [];

      for (const subscription of subscriptions) {
        try {
          // Apply filter if present
          if (subscription.filter && !subscription.filter(event)) {
            continue;
          }

          // Call handler
          await this.callHandlerWithRetry(subscription, event);
        } catch (error: any) {
          this.stats.receiveErrors++;
          console.error(`❌ Handler error for ${topic}:`, error);

          // Send to dead letter topic if configured
          if (subscription.options.deadLetterTopic) {
            await this.publish(subscription.options.deadLetterTopic, {
              originalTopic: topic,
              originalEvent: event,
              error: error?.message || String(error),
              timestamp: Date.now(),
            });
          }
        }
      }

      this.emit('messageReceived', { topic, event, processingTime: Date.now() - startTime });
    } catch (error) {
      this.stats.receiveErrors++;
      console.error(`❌ Failed to handle message from ${topic}:`, error);
      this.emit('messageError', { topic, error });
    }
  }

  // Call handler with retry logic
  private async callHandlerWithRetry(
    subscription: TopicSubscription,
    event: PubSubEvent,
    attempt: number = 1,
  ): Promise<void> {
    try {
      await subscription.handler(event);
    } catch (error) {
      if (attempt < subscription.options.maxRetries) {
        console.warn(
          `🔄 Retrying handler (attempt ${attempt + 1}/${subscription.options.maxRetries})`,
        );
        await new Promise((resolve) => setTimeout(resolve, this.config.retryDelay * attempt));
        return this.callHandlerWithRetry(subscription, event, attempt + 1);
      }
      throw error;
    }
  }

  // Process event for publishing (compression, encryption, etc.)
  private async processEventForPublishing(event: PubSubEvent, options: any): Promise<string> {
    let data = JSON.stringify(event);

    // Compression
    if (
      options.compress !== false &&
      this.config.compression.enabled &&
      data.length > this.config.compression.threshold
    ) {
      // In a real implementation, you'd use a compression library like zlib
      // data = await compress(data);
    }

    // Encryption
    if (
      options.encrypt !== false &&
      this.config.encryption.enabled &&
      this.config.encryption.publicKey
    ) {
      // In a real implementation, you'd encrypt with the public key
      // data = await encrypt(data, this.config.encryption.publicKey);
    }

    return data;
  }

  // Process incoming message
  private async processIncomingMessage(message: any): Promise<PubSubEvent | null> {
    try {
      let data = message.data.toString();

      // Decryption
      if (this.config.encryption.enabled && this.config.encryption.privateKey) {
        // In a real implementation, you'd decrypt with the private key
        // data = await decrypt(data, this.config.encryption.privateKey);
      }

      // Decompression
      if (this.config.compression.enabled) {
        // In a real implementation, you'd detect and decompress
        // data = await decompress(data);
      }

      const event = JSON.parse(data) as PubSubEvent;

      // Validate event structure
      if (!event.id || !event.topic || !event.timestamp) {
        console.warn('❌ Invalid event structure received');
        return null;
      }

      // Ignore our own messages
      if (event.source === this.nodeId) {
        return null;
      }

      return event;
    } catch (error) {
      console.error('❌ Failed to process incoming message:', error);
      return null;
    }
  }

  // Direct publish without buffering
  private async publishDirect(
    topic: string,
    data: string,
    retries: number = this.config.maxRetries,
  ): Promise<boolean> {
    for (let attempt = 1; attempt <= retries; attempt++) {
      try {
        await this.ipfsService.pubsub.publish(topic, data);

        this.stats.totalPublished++;
        this.updateTopicStats(topic, 'published', 1);

        return true;
      } catch (error) {
        if (attempt === retries) {
          throw error;
        }

        console.warn(`🔄 Retrying publish (attempt ${attempt + 1}/${retries})`);
        await new Promise((resolve) => setTimeout(resolve, this.config.retryDelay * attempt));
      }
    }

    return false;
  }

  // Buffer event for batch publishing
  private bufferEvent(event: PubSubEvent, _data: string): boolean {
    if (this.eventBuffer.length >= this.config.eventBuffer.maxSize) {
      // Buffer is full, flush immediately
      this.flushEventBuffer();
    }

    this.eventBuffer.push(event);
    return true;
  }

  // Start event buffering
  private startEventBuffering(): void {
    this.bufferFlushInterval = setInterval(() => {
      this.flushEventBuffer();
    }, this.config.eventBuffer.flushInterval);
  }

  // Flush event buffer
  private async flushEventBuffer(): Promise<void> {
    if (this.eventBuffer.length === 0) return;

    const events = [...this.eventBuffer];
    this.eventBuffer.length = 0;

    console.log(`📡 Flushing ${events.length} buffered events`);

    // Group events by topic for efficiency
    const eventsByTopic = new Map<string, PubSubEvent[]>();
    for (const event of events) {
      if (!eventsByTopic.has(event.topic)) {
        eventsByTopic.set(event.topic, []);
      }
      eventsByTopic.get(event.topic)!.push(event);
    }

    // Publish batches
    for (const [topic, topicEvents] of eventsByTopic) {
      try {
        for (const event of topicEvents) {
          const data = await this.processEventForPublishing(event, {});
          await this.publishDirect(topic, data);
        }
      } catch (error) {
        console.error(`❌ Failed to flush events for ${topic}:`, error);
        this.stats.publishErrors += topicEvents.length;
      }
    }
  }

  // Update topic statistics
  private updateTopicStats(
    topic: string,
    metric: 'published' | 'received' | 'subscribers',
    delta: number,
  ): void {
    if (!this.stats.topicStats.has(topic)) {
      this.stats.topicStats.set(topic, {
        published: 0,
        received: 0,
        subscribers: 0,
        lastActivity: Date.now(),
      });
    }

    const stats = this.stats.topicStats.get(topic)!;
    stats[metric] += delta;
    stats.lastActivity = Date.now();
  }

  // Utility methods
  private generateEventId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
  }

  private extractEventType(topic: string): string {
    const parts = topic.split('.');
    return parts[parts.length - 1];
  }

  private prefixTopic(topic: string): string {
    return `${this.config.topicPrefix}.${topic}`;
  }

  // Get PubSub statistics
  getStats(): PubSubStats {
    return { ...this.stats };
  }

  // Get list of active topics
  getActiveTopics(): string[] {
    return Array.from(this.subscriptions.keys());
  }

  // Get subscribers for a topic
  getTopicSubscribers(topic: string): number {
    const fullTopic = this.prefixTopic(topic);
    return this.subscriptions.get(fullTopic)?.length || 0;
  }

  // Check if topic exists
  hasSubscriptions(topic: string): boolean {
    const fullTopic = this.prefixTopic(topic);
    return this.subscriptions.has(fullTopic) && this.subscriptions.get(fullTopic)!.length > 0;
  }

  // Clear all subscriptions
  async clearAllSubscriptions(): Promise<void> {
    const topics = Array.from(this.subscriptions.keys());

    for (const topic of topics) {
      try {
        await this.ipfsService.pubsub.unsubscribe(topic);
      } catch (error) {
        console.error(`Failed to unsubscribe from ${topic}:`, error);
      }
    }

    this.subscriptions.clear();
    this.stats.topicStats.clear();
    this.stats.totalSubscriptions = 0;

    console.log(`📡 Cleared all ${topics.length} subscriptions`);
  }

  // Shutdown
  async shutdown(): Promise<void> {
    console.log('📡 Shutting down PubSubManager...');

    // Stop event buffering
    if (this.bufferFlushInterval) {
      clearInterval(this.bufferFlushInterval as any);
      this.bufferFlushInterval = null;
    }

    // Flush remaining events
    await this.flushEventBuffer();

    // Clear all subscriptions
    await this.clearAllSubscriptions();

    // Clear event listeners
    this.eventListeners.clear();

    this.isInitialized = false;
    console.log('✅ PubSubManager shut down successfully');
  }
}
