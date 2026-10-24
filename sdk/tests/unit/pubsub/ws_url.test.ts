import { afterEach, describe, expect, it, vi } from 'vitest';
import { PubSubClient } from '../../../src/pubsub/client';
import { HttpClient } from '../../../src/core/http';
import { WSClient } from '../../../src/core/ws';
import { createClient } from '../../../src/index';

/**
 * A PubSubClient built without a wsURL used to dial ws://127.0.0.1:6001, a
 * port no gateway listens on, whatever gateway its HTTP client talked to.
 * The WebSocket now goes to the same gateway as the HTTP calls.
 */

function dialedURLs(): string[] {
  const urls: string[] = [];
  vi.spyOn(WSClient.prototype, 'connect').mockImplementation(async function (this: WSClient) {
    urls.push(this.url);
  });
  return urls;
}

function httpClient(baseURL: string): HttpClient {
  const http = new HttpClient({ baseURL, jwt: 'header.payload.signature' });
  vi.spyOn(http, 'ensureCredential').mockResolvedValue(undefined);
  return http;
}

afterEach(() => vi.restoreAllMocks());

describe('the pub/sub WebSocket goes to the gateway the client talks to', () => {
  it('dials wss:// on the HTTPS gateway when no wsURL is given', async () => {
    const urls = dialedURLs();
    const pubsub = new PubSubClient(httpClient('https://gw.example/'));

    await pubsub.subscribe('room');

    expect(urls).toHaveLength(1);
    const dialed = new URL(urls[0]);
    expect(dialed.origin).toBe('wss://gw.example');
    expect(dialed.pathname).toBe('/v1/pubsub/ws');
    expect(dialed.searchParams.get('topic')).toBe('room');
  });

  it('dials ws:// on a plain-HTTP gateway', async () => {
    const urls = dialedURLs();
    const pubsub = new PubSubClient(httpClient('http://10.0.0.5:10004'));

    await pubsub.subscribe('room');

    expect(new URL(urls[0]).origin).toBe('ws://10.0.0.5:10004');
  });

  it('keeps a wsURL the caller names', async () => {
    const urls = dialedURLs();
    const pubsub = new PubSubClient(httpClient('https://gw.example'), { wsURL: 'wss://other.example' });

    await pubsub.subscribe('room');

    expect(new URL(urls[0]).origin).toBe('wss://other.example');
  });

  it('createClient subscribes on its baseURL', async () => {
    const urls = dialedURLs();
    vi.spyOn(HttpClient.prototype, 'ensureCredential').mockResolvedValue(undefined);
    const client = createClient({ baseURL: 'https://ns.orama.example', jwt: 'header.payload.signature' });

    await client.pubsub.subscribe('room');

    expect(new URL(urls[0]).origin).toBe('wss://ns.orama.example');
  });
});
