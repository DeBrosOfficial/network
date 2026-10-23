import { describe, expect, it, vi } from 'vitest';
import { HttpClient } from '../../../src/core/http';
import { NetworkClient } from '../../../src/network/client';

function gateway(response: unknown) {
  const sent: Array<{ path: string; method: string; body: any }> = [];
  const fetchImpl = vi.fn(async (url: any, init: any) => {
    sent.push({
      path: new URL(String(url)).pathname,
      method: init?.method ?? 'GET',
      body: init?.body ? JSON.parse(init.body) : undefined,
    });
    return new Response(JSON.stringify(response), {
      status: 200,
      headers: { 'content-type': 'application/json' },
    });
  });
  const http = new HttpClient({ baseURL: 'https://gw.example', maxRetries: 0, fetch: fetchImpl as any });
  return { network: new NetworkClient(http), sent };
}

describe('the network client speaks the gateway contract', () => {
  /**
   * bug-724: `connect` sent `{peer_addr}` while `/v1/network/connect` decodes
   * `{multiaddr}`, so every call was a 400 "invalid body: expected {multiaddr}".
   */
  it('connect sends {multiaddr}', async () => {
    const { network, sent } = gateway({ status: 'ok' });
    const addr = '/ip4/10.0.0.2/tcp/4001/p2p/12D3KooWExample';

    await network.connect(addr);

    expect(sent).toEqual([{ path: '/v1/network/connect', method: 'POST', body: { multiaddr: addr } }]);
  });

  it('disconnect sends {peer_id}', async () => {
    const { network, sent } = gateway({ status: 'ok' });

    await network.disconnect('12D3KooWExample');

    expect(sent).toEqual([{ path: '/v1/network/disconnect', method: 'POST', body: { peer_id: '12D3KooWExample' } }]);
  });

  /** The gateway flattens peers into multiaddr strings, not objects. */
  it('peers returns the multiaddr strings the gateway sends', async () => {
    const peers = ['/p2p/12D3KooWExample', '/ip4/10.0.0.2/tcp/4001'];
    const { network } = gateway({ peers });

    await expect(network.peers()).resolves.toEqual(peers);
  });

  it('status carries the peer id and the IPFS identities', async () => {
    const status = {
      node_id: 'node-1',
      peer_id: '12D3KooWNode',
      connected: true,
      peer_count: 3,
      database_size: 0,
      uptime: 5_000_000_000,
      ipfs: { peer_id: '12D3KooWIpfs', swarm_addresses: ['/ip4/10.0.0.1/tcp/4101'] },
    };
    const { network } = gateway(status);

    const got = await network.status();

    expect(got.peer_id).toBe('12D3KooWNode');
    expect(got.ipfs?.swarm_addresses).toEqual(['/ip4/10.0.0.1/tcp/4101']);
  });
});
