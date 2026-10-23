import { HttpClient } from "../core/http";
import { SDKError } from "../errors";

export interface CacheGetRequest {
  dmap: string;
  key: string;
}

export interface CacheGetResponse {
  key: string;
  value: any;
  dmap: string;
}

export interface CachePutRequest {
  dmap: string;
  key: string;
  value: any;
  ttl?: string; // Duration string like "1h", "30m"
}

export interface CachePutResponse {
  status: string;
  key: string;
  dmap: string;
}

export interface CacheDeleteRequest {
  dmap: string;
  key: string;
}

export interface CacheDeleteResponse {
  status: string;
  key: string;
  dmap: string;
}

export interface CacheMultiGetRequest {
  dmap: string;
  keys: string[];
}

export interface CacheMultiGetResponse {
  results: Array<{
    key: string;
    value: any;
  }>;
  dmap: string;
}

export interface CacheScanRequest {
  dmap: string;
  match?: string; // Optional regex pattern
}

export interface CacheScanResponse {
  keys: string[];
  count: number;
  dmap: string;
}

export interface CacheHealthResponse {
  status: string;
  service: string;
}

export class CacheClient {
  private httpClient: HttpClient;

  constructor(httpClient: HttpClient) {
    this.httpClient = httpClient;
  }

  /**
   * Check cache service health
   */
  async health(): Promise<CacheHealthResponse> {
    return this.httpClient.get("/v1/cache/health");
  }

  /**
   * Get a value from cache
   * Returns null if the key is not found (cache miss/expired), which is normal behavior
   */
  async get(dmap: string, key: string): Promise<CacheGetResponse | null> {
    try {
      return await this.httpClient.post<CacheGetResponse>(
        "/v1/cache/get",
        { dmap, key },
        { idempotent: true }
      );
    } catch (error) {
      // Cache misses (404 or "key not found" messages) are normal behavior - return null instead of throwing
      if (
        error instanceof SDKError &&
        (error.httpStatus === 404 ||
          (error.httpStatus === 500 &&
            error.message?.toLowerCase().includes("key not found")))
      ) {
        return null;
      }
      // Re-throw other errors (network issues, server errors, etc.)
      throw error;
    }
  }

  /**
   * Put a value into cache
   */
  async put(
    dmap: string,
    key: string,
    value: any,
    ttl?: string
  ): Promise<CachePutResponse> {
    return this.httpClient.post<CachePutResponse>("/v1/cache/put", {
      dmap,
      key,
      value,
      ttl,
    });
  }

  /**
   * Delete a value from cache
   */
  async delete(dmap: string, key: string): Promise<CacheDeleteResponse> {
    return this.httpClient.post<CacheDeleteResponse>("/v1/cache/delete", {
      dmap,
      key,
    });
  }

  /**
   * Get multiple values from cache in a single request.
   *
   * Resolves to a map of every requested key to its value, or to null for a
   * key the cache does not hold. A failure (the cache unreachable, the
   * credential refused, a gateway error) rejects with the `SDKError`, as every
   * other cache method does: answering it with nulls made an outage look like
   * a run of cache misses.
   */
  async multiGet(
    dmap: string,
    keys: string[]
  ): Promise<Map<string, any | null>> {
    if (keys.length === 0) {
      return new Map();
    }

    const response = await this.httpClient.post<CacheMultiGetResponse>(
      "/v1/cache/mget",
      { dmap, keys },
      { idempotent: true }
    );

    const resultMap = new Map<string, any | null>();
    keys.forEach((key) => {
      resultMap.set(key, null);
    });
    response.results?.forEach(({ key, value }) => {
      resultMap.set(key, value);
    });
    return resultMap;
  }

  /**
   * Scan keys in a distributed map, optionally matching a regex pattern
   */
  async scan(dmap: string, match?: string): Promise<CacheScanResponse> {
    return this.httpClient.post<CacheScanResponse>(
      "/v1/cache/scan",
      { dmap, match },
      { idempotent: true }
    );
  }
}
