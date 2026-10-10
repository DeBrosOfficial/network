import type { IndexStatus } from "./types";
import { formatInt } from "./units";

export type LagTone = "current" | "behind" | "stuck" | "empty";

export interface LagNote {
  tone: LagTone;
  text: string;
}

/**
 * What a reader should know about how current the history on a page is. The lists that come from the
 * chain indexer (transactions, epochs, rewards) end at its cursor; the chain's head may be ahead.
 * A stuck indexer is said so with its reason, because its lag then does not close by itself.
 */
export function lagNote(index: IndexStatus): LagNote {
  if (index.stuck !== null) {
    const why = index.stuck.statePruned
      ? "the node no longer keeps the chain state it needs there"
      : index.stuck.reason === ""
        ? "it has failed there repeatedly"
        : index.stuck.reason;
    return {
      tone: "stuck",
      text: `The indexer is stuck at block ${formatInt(index.stuck.height)}: ${why}. History stops at block ${formatInt(index.cursor)}, ${formatInt(index.lag)} behind the chain.`,
    };
  }
  if (index.cursor === 0) return { tone: "empty", text: "The indexer has not indexed a block yet, so the history on this page is empty." };
  if (index.lag === 0) return { tone: "current", text: `History is up to date at block ${formatInt(index.cursor)}.` };
  const blocks = index.lag === 1 ? "1 block" : `${formatInt(index.lag)} blocks`;
  return {
    tone: "behind",
    text: `History is ${blocks} behind the chain: indexed to block ${formatInt(index.cursor)}, the chain is at ${formatInt(index.tip)}.`,
  };
}
