import { afterEach, describe, expect, it, vi } from 'vitest';
import { HttpClient } from '../../../src/core/http';
import { StorageClient } from '../../../src/storage/client';

/**
 * Every stored object is pinned (bugboard #432). `upload` takes no pin option,
 * so a caller still passing `{ pin: false }` fails to compile instead of
 * having the gateway refuse it with 400 UNPINNED_UPLOAD_REFUSED.
 */
describe('StorageClient.upload', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('sends the file and no pin field', async () => {
    const http = new HttpClient({ baseURL: 'https://gw.example' });
    const sent: FormData[] = [];
    vi.spyOn(http, 'uploadFile').mockImplementation(async (_path: string, form: FormData) => {
      sent.push(form);
      return { cid: 'bafyone', name: 'a.txt', size: 5 } as never;
    });
    const storage = new StorageClient(http);

    await storage.upload(new Uint8Array([104, 101, 108, 108, 111]), 'a.txt');

    expect(sent).toHaveLength(1);
    expect(sent[0].has('file')).toBe(true);
    expect(sent[0].has('pin')).toBe(false);
  });

  it('has no pin option', () => {
    const storage = new StorageClient(new HttpClient({ baseURL: 'https://gw.example' }));
    const call = () =>
      // @ts-expect-error -- pin was removed in 0.3.5: every stored object is pinned.
      storage.upload(new Uint8Array([1]), 'a.bin', { pin: false });
    expect(typeof call).toBe('function');
  });
});
