import { describe, expect, it } from "vitest";
import { DEFAULT_GATEWAY_ORIGIN } from "../data/chain/client";
import { ManifestError, gatewayOrigin, isClusterOnly, isCreated, parseManifest } from "./manifest";

const SHA = "84ee8d46628343b85c464185e1009974beb34f91c99477a75a5dfb66a5918b67";

/** The shape networks/testnet/manifest.json has, with the fields the explorer ignores. */
const testnet = {
  name: "testnet",
  chain_id: "orama-testnet-7",
  genesis_sha256: SHA,
  seeds: ["ns1.testnet.orama.network", "ns2.testnet.orama.network", "ns3.testnet.orama.network"],
  channel: "nightly",
  min_version: "0.3.0",
  release_repo: "https://test-releases.orama.network",
  release_root_sha256: SHA,
  faucet: true,
};

describe("parseManifest", () => {
  it("TestParseManifest_reads_the_fields_the_explorer_uses", () => {
    expect(parseManifest(testnet)).toEqual({
      name: "testnet",
      chainId: "orama-testnet-7",
      genesisSha256: SHA,
      seeds: testnet.seeds,
    });
  });

  it("TestParseManifest_an_announcement_has_no_genesis_and_is_not_created", () => {
    const m = parseManifest({ ...testnet, name: "futurenet", genesis_sha256: "", seeds: [] });
    expect(isCreated(m)).toBe(false);
    expect(isCreated(parseManifest(testnet))).toBe(true);
  });

  it("TestParseManifest_a_cluster_only_network_has_no_chain_and_is_not_created", () => {
    const m = parseManifest({ ...testnet, name: "devnet", chain_id: "", genesis_sha256: "", seeds: [], faucet: false });
    expect(isClusterOnly(m)).toBe(true);
    expect(isCreated(m)).toBe(false);
    expect(isClusterOnly(parseManifest(testnet))).toBe(false);
  });

  it.each([
    ["not an object", null],
    ["no chain id but a genesis", { ...testnet, chain_id: "" }],
    ["no chain id but seeds", { ...testnet, chain_id: "", genesis_sha256: "" }],
    ["an array", []],
    ["a name with capitals", { ...testnet, name: "Testnet" }],
    ["a name that starts with a digit", { ...testnet, name: "1net" }],
    ["a chain id with a slash", { ...testnet, chain_id: "orama/7" }],
    ["a chain id that ends with a dash", { ...testnet, chain_id: "orama-" }],
    ["a genesis digest in upper case", { ...testnet, genesis_sha256: SHA.toUpperCase() }],
    ["a short genesis digest", { ...testnet, genesis_sha256: "abc" }],
    ["seeds that are not a list", { ...testnet, seeds: "ns1.testnet.orama.network" }],
    ["a seed that is an address with a scheme", { ...testnet, seeds: ["https://ns1.testnet.orama.network"] }],
    ["a seed that is one label", { ...testnet, seeds: ["localhost"] }],
    ["a name that is missing", { ...testnet, name: undefined }],
  ])("TestParseManifest_refuses_%s", (_what, raw) => {
    expect(() => parseManifest(raw)).toThrow(ManifestError);
  });
});

describe("gatewayOrigin", () => {
  it("TestGatewayOrigin_testnet_is_its_public_gateway", () => {
    expect(gatewayOrigin(parseManifest(testnet))).toBe("https://testnet.orama.network");
    expect(gatewayOrigin(parseManifest(testnet))).toBe(DEFAULT_GATEWAY_ORIGIN);
  });

  it("TestGatewayOrigin_nameservers_of_two_domains_do_not_make_a_domain_so_the_first_seed_is_the_gateway", () => {
    const m = parseManifest({ ...testnet, seeds: ["ns1.alpha.example.org", "ns2.beta.example.org"] });
    expect(gatewayOrigin(m)).toBe("https://ns1.alpha.example.org");
  });

  it("TestGatewayOrigin_a_seed_that_is_not_a_nameserver_is_a_gateway_itself", () => {
    const m = parseManifest({ ...testnet, seeds: ["gw.example.org", "gw2.example.org"] });
    expect(gatewayOrigin(m)).toBe("https://gw.example.org");
  });

  it("TestGatewayOrigin_one_nameserver_seed_names_its_domain", () => {
    expect(gatewayOrigin(parseManifest({ ...testnet, seeds: ["ns1.testnet.orama.network"] }))).toBe("https://testnet.orama.network");
  });

  it("TestGatewayOrigin_no_seed_names_no_gateway", () => {
    expect(() => gatewayOrigin(parseManifest({ ...testnet, seeds: [] }))).toThrow(ManifestError);
  });
});
