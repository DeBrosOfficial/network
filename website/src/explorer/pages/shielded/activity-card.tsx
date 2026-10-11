import { groupedInteger } from "../../model/paging";
import type { ShieldedActivity } from "../../model/shielded";
import { Amount } from "../../ui/amount";
import { Card } from "../../ui/card";
import { Pager } from "../epochs/pager";
import { ActivityTable } from "./activity-table";
import { AMOUNT_FRACTION, FEE_FRACTION } from "./constants";
import { summarizeWindow } from "./window-summary";

const PAGE_PARAM = "page";

/** Totals over the rows on screen, in plain words. They are for this page only: the index keeps no running total. */
export function PageTotals({ activity }: { activity: ShieldedActivity }) {
  const t = summarizeWindow(activity.items);
  if (t.accepted === 0) return null;
  return (
    <p className="mb-3 text-[13px] text-muted">
      On this page: <b className="font-medium text-fg"><Amount norama={t.shieldedAmount.toString()} maxFraction={AMOUNT_FRACTION} /></b> shielded in {t.shieldedCount}{" "}
      {t.shieldedCount === 1 ? "transaction" : "transactions"}, <b className="font-medium text-fg"><Amount norama={t.unshieldedAmount.toString()} maxFraction={AMOUNT_FRACTION} /></b>{" "}
      unshielded in {t.unshieldedCount}, and {t.transferCount} private {t.transferCount === 1 ? "transfer" : "transfers"} paying{" "}
      <b className="font-medium text-fg"><Amount norama={t.transferFees.toString()} maxFraction={FEE_FRACTION} /></b> in fees. The amounts of the private transfers are not on chain.
    </p>
  );
}

/** The transactions that touched the pool, newest first, a page at a time. */
export function ActivityCard({ activity, page }: { activity: ShieldedActivity; page: number }) {
  return (
    <Card title="Transactions that touched the pool">
      {activity.items.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted">
          {page === 1 ? "No transaction has touched the shielded pool in this index." : "There are no older transactions in this index."}
        </p>
      ) : (
        <>
          <PageTotals activity={activity} />
          <ActivityTable rows={activity.items} />
          <Pager param={PAGE_PARAM} page={page} hasMore={activity.hasMore} />
        </>
      )}
      <p className="mt-3 text-xs text-muted">
        {activity.total === null ? "" : `${groupedInteger(String(activity.total))} transactions in all since this node's index began. `}
        The index lists the transactions whose events name the pool's account: every shield, shield of earnings and unshield, and every private
        transfer, which burns its fee from the pool. A running total of all volume is not kept, so the totals above cover this page only.
      </p>
    </Card>
  );
}
