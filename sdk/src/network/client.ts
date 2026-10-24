import { HttpClient } from "../core/http";
import { SDKError } from "../errors";

/** IPFS (Kubo) identity of the node, as `/v1/network/status` reports it. */
export interface IPFSPeerInfo {
  peer_id: string;
  swarm_addresses: string[];
}

/** IPFS Cluster identity of the node, as `/v1/network/status` reports it. */
export interface IPFSClusterPeerInfo {
  peer_id: string;
  addresses: string[];
}

/** The body of `GET /v1/network/status` (`client.NetworkStatus` in Go). */
export interface NetworkStatus {
  node_id: string;
  peer_id: string;
  connected: boolean;
  peer_count: number;
  database_size: number;
  /** Nanoseconds since the gateway's network client started. */
  uptime: number;
  ipfs?: IPFSPeerInfo;
  ipfs_cluster?: IPFSClusterPeerInfo;
}

export interface ProxyRequest {
  url: string;
  method: string;
  headers?: Record<string, string>;
  body?: string;
}

export interface ProxyResponse {
  status_code: number;
  headers: Record<string, string>;
  body: string;
  error?: string;
}

export class NetworkClient {
  private httpClient: HttpClient;

  constructor(httpClient: HttpClient) {
    this.httpClient = httpClient;
  }

  /**
   * Check gateway health.
   */
  async health(): Promise<boolean> {
    try {
      await this.httpClient.get("/v1/health");
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Get network status.
   */
  async status(): Promise<NetworkStatus> {
    const response = await this.httpClient.get<NetworkStatus>(
      "/v1/network/status"
    );
    return response;
  }

  /**
   * Get connected peers as multiaddr strings: a `/p2p/<peer id>` entry for
   * each peer followed by the addresses it is reachable at.
   */
  async peers(): Promise<string[]> {
    const response = await this.httpClient.get<{ peers: string[] }>(
      "/v1/network/peers"
    );
    return response.peers || [];
  }

  /**
   * Connect this gateway to a peer. Operator only.
   *
   * @param multiaddr - The peer's address, `/ip4/<ip>/tcp/<port>/p2p/<peer id>`
   */
  async connect(multiaddr: string): Promise<void> {
    await this.httpClient.post("/v1/network/connect", { multiaddr });
  }

  /**
   * Disconnect this gateway from a peer. Operator only.
   */
  async disconnect(peerId: string): Promise<void> {
    await this.httpClient.post("/v1/network/disconnect", { peer_id: peerId });
  }

  /**
   * Proxy an HTTP request through the Tor network (the node's Tor client).
   * Requires authentication (API key or JWT).
   *
   * @param request - The proxy request configuration
   * @returns The proxied response
   * @throws {SDKError} If the node's Tor proxy is not available or the request fails
   *
   * @example
   * ```ts
   * const response = await client.network.proxyAnon({
   *   url: 'https://api.example.com/data',
   *   method: 'GET',
   *   headers: {
   *     'Accept': 'application/json'
   *   }
   * });
   *
   * console.log(response.status_code); // 200
   * console.log(response.body); // Response data
   * ```
   */
  async proxyAnon(request: ProxyRequest): Promise<ProxyResponse> {
    const response = await this.httpClient.post<ProxyResponse>(
      "/v1/proxy/anon",
      request
    );

    // The proxy answers 200 with an `error` field when the upstream request
    // failed, so this is the only place the failure surfaces. It used to be a
    // bare Error, the one failure in the SDK a caller could not branch on with
    // `code`/`httpStatus` like every other.
    if (response.error) {
      throw new SDKError(
        `proxy request failed: ${response.error}`,
        502,
        "PROXY_FAILED",
        { upstream_status: response.status_code, upstream_error: response.error }
      );
    }

    return response;
  }
}
