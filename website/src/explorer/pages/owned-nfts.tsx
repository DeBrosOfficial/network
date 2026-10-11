import { useParams, useSearchParams } from "react-router";
import { useQuery } from "../data/use-query";
import type { EpochListQuery, IndexPage } from "../model/epochs";
import { parseAccount } from "../model/lookup";
import type { NftRecord } from "../model/nfts";
import { shortAddress } from "../model/units";
import { Card } from "../ui/card";
import { IndexQuery } from "../ui/index-query";
import { Page } from "../ui/page";
import { NotFoundBox, Skeleton } from "../ui/query-states";
import { useDocumentTitle } from "../ui/use-document-title";
import { parsePageParam } from "./epochs/logic";
import { Pager } from "./epochs/pager";
import { AssetTable } from "./nft/asset-table";

const PER_PAGE = 20;
const PAGE_PARAM = "page";
const NOT_FOUND_TITLE = "Not an address";

/** The assets one address holds. */
export function OwnedList({ data, page }: { data: IndexPage<NftRecord>; page: number }) {
  if (data.items.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-muted">
        {page === 1 ? "The chain indexer lists no NFT held by this address." : "There are no more assets on later pages."}
      </p>
    );
  }
  return (
    <>
      <AssetTable rows={data.items} />
      <Pager param={PAGE_PARAM} page={page} hasMore={data.hasMore} />
    </>
  );
}

function OwnedLoader({ owner }: { owner: string }) {
  const [params] = useSearchParams();
  const page = parsePageParam(params.get(PAGE_PARAM));
  const query: EpochListQuery = { page, limit: PER_PAGE };
  const { state, refetch } = useQuery((s) => s.assets.getOwnedNfts(owner, query), [owner, page]);
  return (
    <>
      <div>
        <h1 className="font-display text-3xl font-semibold tracking-tight">NFTs of {shortAddress(owner)}</h1>
        <p className="mt-1 break-all font-mono text-xs text-muted">{owner}</p>
      </div>
      <Card title="Assets held">
        <IndexQuery state={state} onRetry={refetch} loading={<Skeleton className="h-48 w-full" />}>
          {(data) => <OwnedList data={data} page={page} />}
        </IndexQuery>
        <p className="mt-3 text-xs text-muted">Ownership is public: a compressed NFT's owner is a field of its leaf. The list is in asset id order, from the chain indexer.</p>
      </Card>
    </>
  );
}

/** `/explorer/nfts/:owner` */
export function OwnedNftsPage() {
  const owner = parseAccount(useParams().owner ?? "");
  useDocumentTitle(owner === null ? NOT_FOUND_TITLE : `NFTs of ${shortAddress(owner)}`);
  return (
    <Page>
      {owner === null ? (
        <NotFoundBox title={NOT_FOUND_TITLE} hint="An owner is an Orama wallet address, which starts with orama1." />
      ) : (
        <OwnedLoader key={owner} owner={owner} />
      )}
    </Page>
  );
}
