import { afterEach, describe, expect, it, vi } from 'vitest';
import { HttpClient } from '../../../src/core/http';

/**
 * The retry was `retryDelayMs * (attempt + 1)`: no jitter, and `Retry-After`
 * ignored. A gateway that answered 429 with "come back in 30 seconds" was
 * asked again a second later, twice more, and then reported as failed — and
 * every client that failed at the same moment came back at the same moment,
 * which is how a recovering gateway is knocked over a second time.
 */

/**
 * Captures how long each retry waited, without waiting.
 *
 * The request deadline is scheduled through the same `setTimeout`, so it is
 * recognised by its length (the client's 60s timeout) and left to run its full
 * course — collapsing it would abort the request instead of retrying it.
 */
const REQUEST_TIMEOUT_MS = 60_000;

function captureDelays() {
  const waits: number[] = [];
  const real = globalThis.setTimeout;
  vi.spyOn(globalThis, 'setTimeout').mockImplementation(((fn: () => void, ms?: number) => {
    if (ms !== undefined && ms > 0 && ms < REQUEST_TIMEOUT_MS) {
      waits.push(ms);
      return real(fn, 0);
    }
    return real(fn, ms);
  }) as typeof setTimeout);
  return waits;
}

function failing(status: number, headers: Record<string, string> = {}) {
  return vi.fn(
    async () =>
      new Response(JSON.stringify({ error: 'later' }), {
        status,
        headers: { 'content-type': 'application/json', ...headers },
      }),
  );
}

function client(fetchImpl: any, maxRetries = 3) {
  return new HttpClient({
    baseURL: 'https://gw.example',
    maxRetries,
    retryDelayMs: 1000,
    timeout: 60_000,
    fetch: fetchImpl,
  });
}

describe('retry backoff', () => {
  afterEach(() => vi.restoreAllMocks());

  it('waits what the gateway asked for, in seconds', async () => {
    const waits = captureDelays();
    const fetchImpl = failing(429, { 'retry-after': '5' });

    await expect(client(fetchImpl, 1).get('/v1/x')).rejects.toMatchObject({ httpStatus: 429 });

    expect(waits).toEqual([5000]);
  });

  it('understands a Retry-After given as a date', async () => {
    const waits = captureDelays();
    // An HTTP date has whole seconds, so the clock is pinned to one: the wait is
    // then exactly the four seconds the header names.
    const now = Date.UTC(2026, 9, 24, 12, 0, 0);
    vi.spyOn(Date, 'now').mockReturnValue(now);
    const when = new Date(now + 4000).toUTCString();
    const fetchImpl = failing(503, { 'retry-after': when });

    await expect(client(fetchImpl, 1).get('/v1/x')).rejects.toMatchObject({ httpStatus: 503 });

    expect(waits).toEqual([4000]);
  });

  it('ignores a Retry-After it cannot read', async () => {
    const waits = captureDelays();
    const fetchImpl = failing(500, { 'retry-after': 'whenever' });

    await expect(client(fetchImpl, 1).get('/v1/x')).rejects.toMatchObject({ httpStatus: 500 });

    const backoff = waits;
    expect(backoff).toHaveLength(1);
    // Falls back to the computed delay: 1000ms plus up to 25% jitter.
    expect(backoff[0]).toBeGreaterThanOrEqual(1000);
    expect(backoff[0]).toBeLessThanOrEqual(1250);
  });

  it('caps a gateway that asks for an unreasonable wait', async () => {
    const waits = captureDelays();
    const fetchImpl = failing(429, { 'retry-after': '3600' });

    await expect(client(fetchImpl, 1).get('/v1/x')).rejects.toMatchObject({ httpStatus: 429 });

    expect(waits).toEqual([30_000]);
  });

  it('grows the delay with each attempt and keeps it within the jitter band', async () => {
    const waits = captureDelays();
    const fetchImpl = failing(503);

    await expect(client(fetchImpl, 3).get('/v1/x')).rejects.toMatchObject({ httpStatus: 503 });

    const backoff = waits;
    expect(backoff).toHaveLength(3);
    expect(backoff[0]).toBeGreaterThanOrEqual(1000);
    expect(backoff[0]).toBeLessThanOrEqual(1250);
    expect(backoff[1]).toBeGreaterThanOrEqual(2000);
    expect(backoff[1]).toBeLessThanOrEqual(2500);
    expect(backoff[2]).toBeGreaterThanOrEqual(3000);
    expect(backoff[2]).toBeLessThanOrEqual(3750);
  });

  /**
   * Jitter is the point: two clients that failed together must not return
   * together. Over enough draws the delays cannot all be identical.
   */
  it('does not send every client back at the same instant', async () => {
    const seen = new Set<number>();

    for (let i = 0; i < 12; i++) {
      const waits = captureDelays();
      await expect(client(failing(500), 1).get('/v1/x')).rejects.toBeDefined();
      seen.add(waits[0]);
      vi.restoreAllMocks();
    }

    expect(seen.size).toBeGreaterThan(1);
  });

  it('does not retry a status that will not change', async () => {
    const fetchImpl = failing(400);
    await expect(client(fetchImpl, 3).get('/v1/x')).rejects.toMatchObject({ httpStatus: 400 });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('retries the statuses that can change', async () => {
    for (const status of [408, 429, 500, 502, 503, 504]) {
      const waits = captureDelays();
      const fetchImpl = failing(status);
      await expect(client(fetchImpl, 1).get('/v1/x')).rejects.toMatchObject({ httpStatus: status });
      expect(fetchImpl, `status ${status} was not retried`).toHaveBeenCalledTimes(2);
      expect(waits.length).toBeGreaterThan(0);
      vi.restoreAllMocks();
    }
  });

  /**
   * bug-936: a function stopped at its timeout ran. Its 504 used to be a 429
   * RATE_LIMITED, which a client reads as "not sent"; now that it is its own
   * code, this loop must not send it again either.
   */
  it('does not resend a function that ran out of time', async () => {
    const fetchImpl = vi.fn(
      async () =>
        new Response(
          JSON.stringify({ ok: false, error: { code: 'FUNCTION_TIMEOUT', message: 'timeout', retryable: false } }),
          { status: 504, headers: { 'content-type': 'application/json' } },
        ),
    );
    await expect(client(fetchImpl, 3).post('/v1/invoke/ns/fn', {})).rejects.toMatchObject({
      httpStatus: 504,
      code: 'FUNCTION_TIMEOUT',
      retryable: false,
    });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });
});

/**
 * bug-426: every method was retried on 5xx, POST included. A write whose
 * response was lost after the database committed it was sent again and could
 * land twice. A request is now resent only when a repeat is safe: the gateway
 * said so, the method is idempotent, or the gateway refused it unprocessed
 * (429).
 */
describe('retry safety', () => {
  afterEach(() => vi.restoreAllMocks());

  function envelope(status: number, code: string, retryable: boolean) {
    return vi.fn(
      async () =>
        new Response(JSON.stringify({ ok: false, error: { code, message: 'no', retryable } }), {
          status,
          headers: { 'content-type': 'application/json' },
        }),
    );
  }

  it('does not resend a POST whose outcome is unknown', async () => {
    for (const status of [500, 502, 503, 504]) {
      const fetchImpl = failing(status);
      await expect(client(fetchImpl, 3).post('/v1/rqlite/exec', { sql: 'INSERT' })).rejects.toMatchObject({
        httpStatus: status,
      });
      expect(fetchImpl, `POST answered ${status} was sent again`).toHaveBeenCalledTimes(1);
    }
  });

  it('does not resend an upload whose outcome is unknown', async () => {
    const fetchImpl = failing(502);
    await expect(client(fetchImpl, 3).uploadFile('/v1/storage/upload', new FormData())).rejects.toMatchObject({
      httpStatus: 502,
    });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('resends a POST the gateway refused before doing any work', async () => {
    captureDelays();
    const fetchImpl = failing(429);
    await expect(client(fetchImpl, 1).post('/v1/rqlite/exec', {})).rejects.toMatchObject({ httpStatus: 429 });
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it('resends a POST the gateway marked retryable', async () => {
    captureDelays();
    const fetchImpl = envelope(503, 'NAMESPACE_GATEWAY_UNAVAILABLE', true);
    await expect(client(fetchImpl, 1).post('/v1/invoke/ns/fn', {})).rejects.toMatchObject({ httpStatus: 503 });
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it('does not resend a POST the gateway marked not retryable', async () => {
    const fetchImpl = envelope(500, 'FUNCTION_EXECUTION_FAILED', false);
    await expect(client(fetchImpl, 3).post('/v1/invoke/ns/fn', {})).rejects.toMatchObject({
      code: 'FUNCTION_EXECUTION_FAILED',
    });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('takes the gateway at its word for an idempotent method too', async () => {
    const fetchImpl = envelope(500, 'INTERNAL', false);
    await expect(client(fetchImpl, 3).get('/v1/x')).rejects.toMatchObject({ httpStatus: 500 });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
  });

  it('resends the idempotent methods on a status that can change', async () => {
    for (const method of ['get', 'put', 'delete'] as const) {
      captureDelays();
      const fetchImpl = failing(503);
      const c = client(fetchImpl, 1);
      const call = method === 'put' ? c.put('/v1/x', {}) : c[method]('/v1/x');
      await expect(call).rejects.toMatchObject({ httpStatus: 503 });
      expect(fetchImpl, `${method} was not retried`).toHaveBeenCalledTimes(2);
      vi.restoreAllMocks();
    }
  });
});
