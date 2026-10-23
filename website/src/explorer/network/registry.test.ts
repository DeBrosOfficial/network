import { describe, expect, it } from "vitest";
import { DEFAULT_GATEWAY_ORIGIN } from "../data/chain/client";
import { ManifestError } from "./manifest";
import { PREFERRED_DEFAULT, REGISTRY, buildRegistry, findNetwork } from "./registry";

const SHA = "84ee8d46628343b85c464185e1009974beb34f91c99477a75a5dfb66a5918b67";

const manifest = (name: string, over: Record<string, unknown> = {}) => ({
  name,
  chain_id: `orama-${name}-1`,
  genesis_sha256: SHA,
  seeds: [`ns1.${name}.orama.network`, `ns2.${name}.orama.network`],
  ...over,
});

const at = (name: string) => `../../../../networks/${name}/manifest.json`;

describe("buildRegistry", () => {
  it("TestBuildRegistry_lists_created_networks_by_name_with_their_gateway", () => {
    const r = buildRegistry({ [at("testnet")]: manifest("testnet"), [at("devnet")]: manifest("devnet") });
    expect(r.networks.map((n) => n.manifest.name)).toEqual(["devnet", "testnet"]);
    expect(findNetwork(r, "testnet")?.gateway).toBe("https://testnet.orama.network");
    expect(findNetwork(r, "nope")).toBeUndefined();
  });

  it("TestBuildRegistry_testnet_is_the_default_when_it_exists", () => {
    const r = buildRegistry({ [at("alpha")]: manifest("alpha"), [at("testnet")]: manifest("testnet") });
    expect(r.defaultName).toBe("testnet");
  });

  it("TestBuildRegistry_without_testnet_the_first_by_name_is_the_default", () => {
    const r = buildRegistry({ [at("zeta")]: manifest("zeta"), [at("alpha")]: manifest("alpha") });
    expect(r.defaultName).toBe("alpha");
  });

  it("TestBuildRegistry_an_announced_network_has_no_chain_and_is_left_out", () => {
    const r = buildRegistry({
      [at("testnet")]: manifest("testnet"),
      [at("futurenet")]: manifest("futurenet", { genesis_sha256: "", seeds: [] }),
    });
    expect(r.networks.map((n) => n.manifest.name)).toEqual(["testnet"]);
  });

  it("TestBuildRegistry_a_manifest_named_differently_from_its_directory_is_refused", () => {
    expect(() => buildRegistry({ [at("devnet")]: manifest("testnet") })).toThrow(/names the network "testnet"/);
  });

  it.each(["custom", "tx", "wallet", "block", "validators", "validator"])(
    "TestBuildRegistry_a_network_named_%s_would_take_a_page_of_the_explorer_and_is_refused",
    (name) => {
      expect(() => buildRegistry({ [at(name)]: manifest(name) })).toThrow(ManifestError);
    },
  );

  it("TestBuildRegistry_a_path_that_is_not_a_manifest_is_refused", () => {
    expect(() => buildRegistry({ "networks/x/genesis.json": manifest("x") })).toThrow(ManifestError);
  });

  it("TestBuildRegistry_an_empty_registry_is_an_error_not_a_blank_selector", () => {
    expect(() => buildRegistry({})).toThrow(/no created network/);
  });
});

describe("the shipped registry", () => {
  it("TestRegistry_opens_testnet_once_it_ships_else_the_first_network", () => {
    const testnet = findNetwork(REGISTRY, PREFERRED_DEFAULT);
    expect(REGISTRY.defaultName).toBe(testnet === undefined ? REGISTRY.networks[0]?.manifest.name : PREFERRED_DEFAULT);
    if (testnet === undefined) return;
    expect(testnet.gateway).toBe(DEFAULT_GATEWAY_ORIGIN);
    expect(testnet.manifest.chainId).toMatch(/^orama-testnet-\d+$/);
  });

  it("TestRegistry_every_gateway_is_an_https_origin", () => {
    for (const n of REGISTRY.networks) expect(n.gateway).toMatch(/^https:\/\/[a-z0-9.-]+$/);
  });
});
