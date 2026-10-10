import { describe, expect, it } from "vitest";
import { ChainIndexUnavailable, ChainNotFound, ChainReadError, CHAIN_BASE, DEFAULT_GATEWAY_ORIGIN, chainBase, createClient } from "./client";

describe("chainBase", () => {
  it("TestChainBase_puts_the_chain_proxy_on_the_gateways_origin", () => {
    expect(chainBase("https://stagenet.orama.network")).toBe("https://stagenet.orama.network/v1/chain");
    expect(chainBase("https://gw.example.org:8443")).toBe("https://gw.example.org:8443/v1/chain");
  });

  it("TestChainBase_default_is_a_gateway_not_the_page_origin", () => {
    // The page is served from orama.network, which has no chain behind it: a relative base would read the site shell.
    expect(CHAIN_BASE).toBe(`${DEFAULT_GATEWAY_ORIGIN}/v1/chain`);
    expect(CHAIN_BASE.startsWith("https://")).toBe(true);
  });

  it("TestChainBase_allows_a_gateway_on_this_machine_over_http", () => {
    expect(chainBase("http://localhost:6001")).toBe("http://localhost:6001/v1/chain");
    expect(chainBase("http://127.0.0.1:6001")).toBe("http://127.0.0.1:6001/v1/chain");
    expect(chainBase("https://localhost")).toBe("https://localhost/v1/chain");
  });

  it.each([
    ["an empty string", ""],
    ["a bare host", "stagenet.orama.network"],
    ["clear http on a public host", "http://stagenet.orama.network"],
    ["a trailing slash", "https://stagenet.orama.network/"],
    ["a path", "https://stagenet.orama.network/v1/chain"],
    ["a query", "https://stagenet.orama.network?x=1"],
    ["credentials in the address", "https://user:pw@stagenet.orama.network"],
    ["another scheme", "ftp://stagenet.orama.network"],
    ["a lookalike of localhost over http", "http://localhost.evil.example"],
    ["a lookalike of 127.0.0.1 over http", "http://127.0.0.1.evil.example"],
    ["credentials on a local address", "http://user:pw@localhost:6001"],
    ["an upper-case host", "https://Stagenet.orama.network"],
    ["an explicit default port", "https://stagenet.orama.network:443"],
    ["a fragment", "https://stagenet.orama.network#x"],
  ])("TestChainBase_refuses_%s", (_name, origin) => {
    expect(() => chainBase(origin)).toThrow(ChainReadError);
  });
});

describe("createClient", () => {
  const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });

  it("TestCreateClient_reads_from_the_gateway_origin_without_credentials", async () => {
    const seen: { url: string; init?: RequestInit }[] = [];
    const client = createClient(async (url, init) => {
      seen.push({ url, init });
      return json({ ok: true });
    });
    expect(await client.get("status")).toEqual({ ok: true });
    expect(seen[0]?.url).toBe(`${DEFAULT_GATEWAY_ORIGIN}/v1/chain/status`);
    // Nothing of the reader's (cookies, tokens) goes to another origin, and the request stays a simple one: no preflight.
    expect(seen[0]?.init?.credentials).toBe("omit");
    expect(seen[0]?.init?.method).toBe("GET");
    expect(seen[0]?.init?.headers).toEqual({ Accept: "application/json" });
  });

  it("TestCreateClient_takes_another_networks_base", async () => {
    let url = "";
    const client = createClient(async (u) => {
      url = u;
      return json({});
    }, chainBase("https://testnet.example.org"));
    await client.get("block?height=1");
    expect(url).toBe("https://testnet.example.org/v1/chain/block?height=1");
  });

  it("TestCreateClient_a_404_is_not_found_and_other_failures_are_read_errors", async () => {
    const notFound = createClient(async () => new Response("", { status: 404 }));
    await expect(notFound.get("index/txs/ab")).rejects.toBeInstanceOf(ChainNotFound);
    const limited = createClient(async () => new Response("rate limit exceeded", { status: 429 }));
    await expect(limited.get("status")).rejects.toThrow("HTTP 429");
  });

  it("TestCreateClient_a_503_from_a_node_with_no_indexer_is_told_apart_from_any_other_503", async () => {
    const noIndexer = createClient(
      async () => new Response("this node runs no chain indexer; install it with orama global install --services chain,indexer\n", { status: 503 }),
    );
    await expect(noIndexer.get("index/epochs")).rejects.toBeInstanceOf(ChainIndexUnavailable);
    await expect(noIndexer.get("index/epochs")).rejects.not.toThrow(/orama global install/);
    const busy = createClient(async () => new Response("too many chain queries in flight", { status: 503 }));
    const error = await busy.get("query/orama.fees.v1.Query/BaseFee").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ChainReadError);
    expect(error).not.toBeInstanceOf(ChainIndexUnavailable);
    expect((error as Error).message).toContain("HTTP 503");
  });

  it("TestCreateClient_a_blocked_or_unreachable_gateway_says_so_without_the_cause", async () => {
    // A CORS refusal and a dead gateway are the same to a script: fetch rejects.
    const client = createClient(async () => {
      throw new TypeError("Failed to fetch");
    });
    await expect(client.get("status")).rejects.toThrow("The chain could not be reached.");
  });
});
