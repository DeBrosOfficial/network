/** How long one read may take before the reader is told the chain could not be reached. */
const REQUEST_TIMEOUT_MS = 15_000;

const HTTP_NOT_FOUND = 404;
const HTTP_UNAVAILABLE = 503;
/** The gateway's words for an index route on a node with no indexer (core/pkg/gateway/handlers/chainread/index.go, msgNoIndexer). */
const NO_INDEXER_MARKER = "runs no chain indexer";
/** A 503 body is read only this far to tell the no-indexer answer from any other. */
const NO_INDEXER_BODY_MAX = 512;

/** A read that failed. The message is written for the reader: it never carries what the node said. */
export class ChainReadError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ChainReadError";
  }
}

/** The thing asked for is not on the chain (HTTP 404). A lookup turns this into null. */
export class ChainNotFound extends ChainReadError {
  constructor() {
    super("That is not on the chain.");
    this.name = "ChainNotFound";
  }
}

/** The gateway answered 503 because the node it reached runs no chain indexer, so none of the index routes (blocks, epochs, supply) can be read from it. */
export class ChainIndexUnavailable extends ChainReadError {
  constructor() {
    super("This node runs no chain indexer, so it cannot serve this page. Another node of the network may; try again later.");
    this.name = "ChainIndexUnavailable";
  }
}

/** The gateway's read-only chain proxy (docs/whitepaper/technical-reference/vol2/42-archive-and-indexer.md, "The gateway's chain proxy"). */
const CHAIN_PATH = "/v1/chain";

/**
 * The network the explorer reads: its gateway's origin (stagenet's; the page has
 * no network picker yet, and createChainSource takes another base). The page is served from orama.network, which has no chain behind it,
 * so the browser calls the network's own gateway across origins; that gateway
 * answers the explorer's origin on the chain's read routes
 * (core/pkg/gateway/cors_explorer.go).
 */
export const DEFAULT_GATEWAY_ORIGIN = "https://stagenet.orama.network";

const LOCAL_HOSTS = new Set(["localhost", "127.0.0.1"]);

/**
 * The chain proxy's base URL on a network's gateway. The origin must be exactly
 * an origin (no path, query or trailing slash) and https, or http for a gateway
 * on this machine: a reader's balance and address lookups never travel in clear.
 */
export function chainBase(gatewayOrigin: string): string {
  let url: URL;
  try {
    url = new URL(gatewayOrigin);
  } catch {
    throw new ChainReadError(`"${gatewayOrigin}" is not a gateway address.`);
  }
  const local = url.protocol === "http:" && LOCAL_HOSTS.has(url.hostname);
  if ((url.protocol !== "https:" && !local) || url.origin !== gatewayOrigin) {
    throw new ChainReadError(`"${gatewayOrigin}" is not an https gateway origin such as https://stagenet.orama.network.`);
  }
  return `${url.origin}${CHAIN_PATH}`;
}

/** The chain proxy of the default network. */
export const CHAIN_BASE = chainBase(DEFAULT_GATEWAY_ORIGIN);

export type ChainFetch = (input: string, init?: RequestInit) => Promise<Response>;

export interface ChainClient {
  /** GET one proxy path and return its JSON body. Rejects with ChainNotFound for a 404. */
  get(path: string): Promise<unknown>;
}

/** A path from segments, each percent-encoded, so no value can add a segment or a query. */
export function chainPath(...segments: string[]): string {
  return segments.map(encodeURIComponent).join("/");
}

/** The query string of a path: keys and values are encoded, and an undefined value is left out. */
export function withQuery(path: string, query: Record<string, string | number | undefined>): string {
  const pairs = Object.entries(query)
    .filter((entry): entry is [string, string | number] => entry[1] !== undefined)
    .map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`);
  return pairs.length === 0 ? path : `${path}?${pairs.join("&")}`;
}

async function saysNoIndexer(response: Response): Promise<boolean> {
  try {
    return (await response.text()).slice(0, NO_INDEXER_BODY_MAX).includes(NO_INDEXER_MARKER);
  } catch {
    return false;
  }
}

export function createClient(fetcher: ChainFetch = (input, init) => fetch(input, init), base: string = CHAIN_BASE): ChainClient {
  return {
    async get(path: string): Promise<unknown> {
      let response: Response;
      try {
        response = await fetcher(`${base}/${path}`, {
          method: "GET",
          headers: { Accept: "application/json" },
          cache: "no-store",
          credentials: "omit",
          signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
        });
      } catch {
        throw new ChainReadError("The chain could not be reached.");
      }
      if (response.status === HTTP_NOT_FOUND) throw new ChainNotFound();
      if (response.status === HTTP_UNAVAILABLE && (await saysNoIndexer(response))) throw new ChainIndexUnavailable();
      if (!response.ok) throw new ChainReadError(`The chain could not be read (HTTP ${response.status}).`);
      try {
        return await response.json();
      } catch {
        throw new ChainReadError("The chain answered something that is not JSON.");
      }
    },
  };
}
