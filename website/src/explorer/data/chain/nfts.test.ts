import { describe, expect, it } from "vitest";
import { ChainReadError, chainPath, createClient, withQuery } from "./client";
import { FakeGateway, addr, hashOf } from "./fake-gateway";
import { loadBid, loadListing } from "./market";
import { queryPath } from "./module-query";
import { loadAsset, loadCollection, loadOwnerAssets, loadSnapshots, loadTree } from "./nfts";

const client = (gw: FakeGateway) => createClient(gw.fetch, "https://gw.example/v1/chain");
const ASSET = "ab".repeat(32);

function das(over: Record<string, unknown> = {}) {
  return {
    id: ASSET,
    burnt: false,
    content: { metadata_cid: "bafybeigdyrzt" },
    compression: { compressed: true, tree: 7, leaf_id: 4, nonce: 2, hash_id: 1, creator_hash: "cd".repeat(32), leaf_hash: "ef".repeat(32) },
    grouping: [{ group_key: "collection", group_value: "3" }],
    ownership: { owner: addr(1), delegate: "", delegated: false, state: "compressed" },
    last_update: { height: 90, tx: hashOf(77) },
    ...over,
  };
}

describe("collections and trees", () => {
  it("TestLoadCollection_reads_name_creator_and_royalty", async () => {
    const gw = new FakeGateway().on(queryPath("orama.cnft.v1.Query/Collection", { id: 3 }), { collection: { id: "3", creator: addr(2), royalty_bps: 250, name: "Apes" } });
    expect(await loadCollection(client(gw), 3)).toEqual({ id: 3, creator: addr(2), royaltyBps: 250, name: "Apes" });
  });

  it("TestLoadCollection_a_missing_collection_and_an_odd_id_are_null", async () => {
    const gw = new FakeGateway();
    expect(await loadCollection(client(gw), 9)).toBeNull();
    expect(await loadCollection(client(gw), 0)).toBeNull();
    expect(gw.asked).toHaveLength(1);
  });

  it("TestLoadCollection_another_collection_than_asked_for_is_rejected", async () => {
    const gw = new FakeGateway().on(queryPath("orama.cnft.v1.Query/Collection", { id: 3 }), { collection: { id: "4", creator: addr(2), royalty_bps: 0, name: "" } });
    await expect(loadCollection(client(gw), 3)).rejects.toBeInstanceOf(ChainReadError);
  });

  it("TestLoadTree_reads_depth_and_the_next_leaf_and_ignores_the_changelog", async () => {
    const gw = new FakeGateway().on(queryPath("orama.cnft.v1.Query/Tree", { id: 7 }), {
      tree: {
        id: "7",
        creator: addr(2),
        collection_id: "3",
        depth: 14,
        buffer_size: 64,
        canopy_depth: 0,
        sequence: "12",
        active_index: "5",
        change_logs: [{ root: "AAAA", path: [], index: 0 }],
        rightmost: { proof: [], leaf: "", index: 6 },
      },
    });
    expect(await loadTree(client(gw), 7)).toEqual({
      id: 7,
      creator: addr(2),
      collectionId: 3,
      depth: 14,
      bufferSize: 64,
      canopyDepth: 0,
      sequence: "12",
      nextLeaf: "6",
    });
  });

  it("TestLoadSnapshots_lists_content_ids_and_a_missing_tree_has_none", async () => {
    const gw = new FakeGateway().on(queryPath("orama.cnft.v1.Query/Snapshots", { tree_id: 7 }), { snapshots: [{ tree_id: "7", id: "1", cid: "bafysnap", sequence: "9" }] });
    expect(await loadSnapshots(client(gw), 7)).toEqual([{ id: 1, cid: "bafysnap", sequence: "9" }]);
    expect(await loadSnapshots(client(gw), 8)).toEqual([]);
  });
});

describe("assets from the indexer", () => {
  it("TestLoadAsset_reads_every_record_of_an_id", async () => {
    const gw = new FakeGateway().on(`index/cnft/assets/${ASSET}`, {
      id: ASSET,
      records: [das(), das({ compression: { compressed: false, tree: 8, leaf_id: 0, nonce: 0, hash_id: 1, creator_hash: "cd".repeat(32) }, ownership: { owner: addr(2), delegate: addr(3), delegated: true, state: "decompressed" } })],
    });
    const records = await loadAsset(client(gw), ASSET.toUpperCase());
    expect(records).toHaveLength(2);
    expect(records?.[0]).toMatchObject({ id: ASSET, state: "compressed", treeId: 7, leafIndex: 4, collectionId: 3, owner: addr(1), delegate: null, metadataCid: "bafybeigdyrzt", nonce: "2", updatedHeight: 90 });
    expect(records?.[1]).toMatchObject({ state: "decompressed", treeId: 8, delegate: addr(3) });
  });

  it("TestLoadAsset_an_id_the_index_does_not_hold_is_null_and_a_malformed_id_asks_nothing", async () => {
    const gw = new FakeGateway();
    expect(await loadAsset(client(gw), ASSET)).toBeNull();
    expect(await loadAsset(client(gw), "nothex")).toBeNull();
    expect(gw.asked).toEqual([`index/cnft/assets/${ASSET}`]);
  });

  it("TestLoadAsset_an_unknown_state_rejects_the_read", async () => {
    const gw = new FakeGateway().on(`index/cnft/assets/${ASSET}`, { id: ASSET, records: [das({ ownership: { owner: addr(1), state: "stolen" } })] });
    await expect(loadAsset(client(gw), ASSET)).rejects.toThrow("The chain sent a malformed asset state.");
  });

  it("TestLoadOwnerAssets_pages_by_number_and_says_when_the_page_was_full", async () => {
    const path = withQuery(chainPath("index", "cnft", "owners", addr(1), "assets"), { page: 2, limit: 2 });
    const gw = new FakeGateway().on(path, { page: 2, limit: 2, assets: [das(), das({ id: "cd".repeat(32) })] });
    const page = await loadOwnerAssets(client(gw), addr(1), 2, 2);
    expect(page).toMatchObject({ page: 2, limit: 2, hasMore: true });
    expect(page.items.map((r) => r.id)).toEqual([ASSET, "cd".repeat(32)]);
  });

  it("TestLoadOwnerAssets_an_owner_with_nothing_is_an_empty_last_page", async () => {
    const gw = new FakeGateway().on(withQuery(chainPath("index", "cnft", "owners", addr(1), "assets"), { page: 1, limit: 20 }), { page: 1, limit: 20, assets: [] });
    expect(await loadOwnerAssets(client(gw), addr(1), 1, 20)).toEqual({ items: [], page: 1, limit: 20, hasMore: false });
  });
});

describe("market", () => {
  const listing = {
    id: "5",
    seller: addr(1),
    tree_id: "7",
    leaf_index: 4,
    asset_id: btoa(String.fromCharCode(...new Uint8Array(32).fill(0xab))),
    price: "2500000000",
    royalty_creator: addr(2),
    royalty_bps: 500,
    collection_id: "3",
  };

  it("TestLoadListing_reads_price_seller_asset_and_royalty", async () => {
    const gw = new FakeGateway().on(queryPath("orama.market.v1.Query/Listing", { id: 5 }), { listing });
    expect(await loadListing(client(gw), 5)).toEqual({
      id: 5,
      seller: addr(1),
      treeId: 7,
      leafIndex: 4,
      assetId: ASSET,
      price: "2500000000",
      royaltyCreator: addr(2),
      royaltyBps: 500,
      collectionId: 3,
    });
  });

  it("TestLoadListing_a_cancelled_or_settled_listing_is_null", async () => {
    expect(await loadListing(client(new FakeGateway()), 5)).toBeNull();
  });

  it("TestLoadListing_an_asset_id_of_the_wrong_length_is_rejected", async () => {
    const gw = new FakeGateway().on(queryPath("orama.market.v1.Query/Listing", { id: 5 }), { listing: { ...listing, asset_id: btoa("short") } });
    await expect(loadListing(client(gw), 5)).rejects.toBeInstanceOf(ChainReadError);
  });

  it("TestLoadBid_reads_the_bidder_and_the_escrow_and_a_missing_bid_is_null", async () => {
    const gw = new FakeGateway().on(queryPath("orama.market.v1.Query/Bid", { listing_id: 5, bid_id: 9 }), { bid: { id: "9", listing_id: "5", bidder: addr(3), amount: "100" } });
    expect(await loadBid(client(gw), 5, 9)).toEqual({ id: 9, listingId: 5, bidder: addr(3), amount: "100" });
    expect(await loadBid(client(gw), 5, 10)).toBeNull();
    expect(await loadBid(client(gw), 0, 1)).toBeNull();
  });
});
