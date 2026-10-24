import { describe, expect, it, vi } from 'vitest';
import { CacheClient } from '../../../src/cache/client';
import { HttpClient } from '../../../src/core/http';
import { NetworkError, NotFoundError, SDKError } from '../../../src/errors';

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

function cache(fetchImpl: any) {
  return new CacheClient(
    new HttpClient({ baseURL: 'https://gw.example', maxRetries: 0, fetch: fetchImpl }),
  );
}

/**
 * bug-385: every failure of multiGet, and a 404 above all, came back as a map
 * of nulls, so a cache that was down or refused the credential looked like a
 * run of misses and the application fell through to its cold path for ever.
 */
describe('cache multiGet', () => {
  it('maps every key, with null for the ones the cache does not hold', async () => {
    const fetchImpl = vi.fn(async () => json(200, { results: [{ key: 'a', value: 1 }] }));

    const got = await cache(fetchImpl).multiGet('users', ['a', 'b']);

    expect([...got]).toEqual([
      ['a', 1],
      ['b', null],
    ]);
  });

  it('asks nothing for no keys', async () => {
    const fetchImpl = vi.fn();

    expect((await cache(fetchImpl).multiGet('users', [])).size).toBe(0);
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it('raises a gateway error instead of answering misses', async () => {
    const fetchImpl = vi.fn(async () => json(503, { error: 'cache service unavailable' }));

    await expect(cache(fetchImpl).multiGet('users', ['a'])).rejects.toMatchObject({
      httpStatus: 503,
      message: 'cache service unavailable',
    });
  });

  it('raises a 404 instead of answering misses', async () => {
    const fetchImpl = vi.fn(async () => json(404, { error: 'not found' }));

    await expect(cache(fetchImpl).multiGet('users', ['a'])).rejects.toBeInstanceOf(NotFoundError);
  });

  it('raises an unreachable gateway', async () => {
    const fetchImpl = vi.fn(async () => {
      throw new TypeError('fetch failed');
    });

    const failure = await cache(fetchImpl).multiGet('users', ['a']).catch((e) => e);

    expect(failure).toBeInstanceOf(NetworkError);
    expect(failure).toBeInstanceOf(SDKError);
  });

  it('is retried like a read: it changes nothing', async () => {
    const fetchImpl = vi
      .fn()
      .mockResolvedValueOnce(json(502, { error: 'bad gateway' }))
      .mockResolvedValueOnce(json(200, { results: [{ key: 'a', value: 1 }] }));
    const client = new CacheClient(
      new HttpClient({ baseURL: 'https://gw.example', maxRetries: 1, retryDelayMs: 1, fetch: fetchImpl }),
    );

    expect((await client.multiGet('users', ['a'])).get('a')).toBe(1);
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });
});
