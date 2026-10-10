import { ChainNotFound, ChainReadError, withQuery } from "./client";
import type { ChainClient } from "./client";
import { optArr, optStr, rec } from "./wire";

/** The most entries one list query answers: the gateway refuses a larger page (core/pkg/gateway/handlers/chainread, queryMaxPageLimit). */
const LIST_PAGE_SIZE = 100;

export interface Listed<T> {
  items: T[];
  /** True when the chain has more entries than the pages read. */
  more: boolean;
}

interface ListOptions {
  /** The most pages read; the rest is reported as `more`. */
  maxPages: number;
  /** Newest first, for a list keyed by an increasing id. */
  reverse?: boolean;
}

/** One page. A 404 on a list query is a gateway that does not serve it (an older version), not an empty list. */
async function getPage(client: ChainClient, method: string, field: string, pagination: Record<string, unknown>): Promise<unknown> {
  try {
    return await client.get(withQuery(method, { json: JSON.stringify({ pagination }) }));
  } catch (err) {
    if (err instanceof ChainNotFound) throw new ChainReadError(`This gateway does not serve the list of ${field} yet; it runs an older version.`);
    throw err;
  }
}

/**
 * Reads a paged list query of an Orama module (Nodes, Deals, Relays) by following the page key
 * each answer carries. `field` is the answer's list; `read` checks one entry.
 */
export async function listAll<T>(
  client: ChainClient,
  method: string,
  field: string,
  read: (raw: unknown) => T,
  options: ListOptions,
): Promise<Listed<T>> {
  const items: T[] = [];
  let key = "";
  for (let page = 0; page < options.maxPages; page++) {
    const pagination: Record<string, unknown> = { limit: LIST_PAGE_SIZE };
    if (key !== "") pagination.key = key;
    if (options.reverse) pagination.reverse = true;
    const body = rec(await getPage(client, method, field, pagination), method);
    items.push(...optArr(body[field], field).map(read));
    key = body.pagination === null || body.pagination === undefined ? "" : optStr(rec(body.pagination, "page").next_key, "page key");
    if (key === "") return { items, more: false };
  }
  return { items, more: true };
}
