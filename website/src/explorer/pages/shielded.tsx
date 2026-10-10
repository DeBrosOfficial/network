import { useSearchParams } from "react-router";
import { useQuery } from "../data/use-query";
import { IndexQuery } from "../ui/index-query";
import { Page } from "../ui/page";
import { Query, Skeleton } from "../ui/query-states";
import { useDocumentTitle } from "../ui/use-document-title";
import { parsePageParam } from "./epochs/logic";
import { ActivityCard } from "./shielded/activity-card";
import { PoolStats, RulesCard } from "./shielded/pool-cards";
import { VisibilityCard } from "./shielded/visibility-card";

const PER_PAGE = 20;
const PAGE_PARAM = "page";

/** `/explorer/shielded` */
export function ShieldedPage() {
  useDocumentTitle("Shielded pool");
  const [params] = useSearchParams();
  const page = parsePageParam(params.get(PAGE_PARAM));
  const pool = useQuery((s) => s.assets.getShieldedPool(), []);
  const activity = useQuery((s) => s.assets.getShieldedActivity({ page, limit: PER_PAGE }), [page]);
  return (
    <Page>
      <div>
        <h1 className="font-display text-3xl font-semibold tracking-tight">Shielded pool</h1>
        <p className="mt-1 max-w-[75ch] text-sm text-muted">
          The private side of the chain. Value that enters the pool moves between notes with no visible sender, receiver or amount, and comes out
          again to its owner. Using the pool is optional.
        </p>
      </div>
      <VisibilityCard />
      <Query state={pool.state} onRetry={pool.refetch} loading={<Skeleton className="h-48 w-full" />}>
        {(p) => (
          <>
            <PoolStats pool={p} />
            <RulesCard pool={p} />
          </>
        )}
      </Query>
      <IndexQuery state={activity.state} onRetry={activity.refetch} loading={<Skeleton className="h-64 w-full" />}>
        {(a) => <ActivityCard activity={a} page={page} />}
      </IndexQuery>
    </Page>
  );
}
