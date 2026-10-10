import { useLiveQuery } from "../data/use-query";
import { lagNote } from "../model/index-lag";
import type { LagTone } from "../model/index-lag";
import type { IndexStatus } from "../model/types";
import { cn } from "../../lib/utils";

const TONE: Record<LagTone, string> = {
  current: "text-muted",
  behind: "text-signal",
  empty: "text-muted",
  stuck: "text-loss",
};

/** One line saying how far behind the chain the indexed history is. A stuck indexer is an alert, the rest are quiet status text. */
export function IndexLagLine({ index, className }: { index: IndexStatus; className?: string }) {
  const note = lagNote(index);
  return (
    <p role={note.tone === "stuck" ? "alert" : "status"} data-index-lag={note.tone} className={cn("text-[12.5px]", TONE[note.tone], className)}>
      {note.text}
    </p>
  );
}

/**
 * The same line, for a page that has no index status of its own. It asks the indexer's light status
 * route again whenever the head moves, and says nothing while that is loading or when the node runs no
 * indexer (the page's own query reports that).
 */
export function IndexLagNote({ className }: { className?: string }) {
  const { state } = useLiveQuery((s) => s.getIndexStatus(), []);
  if (state.status !== "ready") return null;
  return <IndexLagLine index={state.data} className={className} />;
}
