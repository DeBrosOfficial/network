import type { EpochRow, SupplyPoint } from "../../model/epochs";

const PERMILLE = 1000n;

export type SliceKey = "circulating" | "bonded" | "unbonding" | "earningsPools" | "storageEscrow" | "otherProtocol";

export interface Slice {
  key: SliceKey;
  label: string;
  /** What the slice holds, norama. */
  amount: bigint;
  /** Its share of the total supply in thousandths (0 to 1000), truncated; 0 when the supply is 0. */
  permille: number;
}

const LABELS: Record<SliceKey, string> = {
  circulating: "Circulating",
  bonded: "Staked (bonded to validators)",
  unbonding: "Unbonding",
  earningsPools: "Earnings pools",
  storageEscrow: "Storage escrow",
  otherProtocol: "Other protocol accounts",
};

/**
 * Where the total supply sits at one point. The six slices are the indexer's own breakdown:
 * circulating is the supply no protocol module account holds, so the slices add up to the total.
 */
export function composition(point: SupplyPoint): Slice[] {
  const total = BigInt(point.totalSupply);
  const amounts: Record<SliceKey, bigint> = {
    circulating: BigInt(point.circulating),
    bonded: BigInt(point.bonded),
    unbonding: BigInt(point.unbonding),
    earningsPools: BigInt(point.earningsPools),
    storageEscrow: BigInt(point.storageEscrow),
    otherProtocol: BigInt(point.otherProtocol),
  };
  return (Object.keys(LABELS) as SliceKey[]).map((key) => ({
    key,
    label: LABELS[key],
    amount: amounts[key],
    permille: total > 0n ? Number((amounts[key] * PERMILLE) / total) : 0,
  }));
}

/** Whether the slices add up to the total supply, which they do unless the index is inconsistent. */
export function compositionAddsUp(point: SupplyPoint): boolean {
  return composition(point).reduce((sum, s) => sum + s.amount, 0n) === BigInt(point.totalSupply);
}

/** One figure of each supply point, oldest first: the order a chart draws in (the index answers newest first). */
export function supplySeries(points: readonly SupplyPoint[], pick: (p: SupplyPoint) => string): bigint[] {
  return [...points].reverse().map((p) => BigInt(pick(p)));
}

/** One figure of each epoch row, oldest first. */
export function epochSeries(rows: readonly EpochRow[], pick: (r: EpochRow) => string): bigint[] {
  return [...rows].reverse().map((r) => BigInt(pick(r)));
}

/** The epochs the supply points cover, oldest and newest; null for no points. */
export function supplySpan(points: readonly SupplyPoint[]): { first: number; last: number } | null {
  const newest = points[0];
  const oldest = points[points.length - 1];
  return newest && oldest ? { first: oldest.epoch, last: newest.epoch } : null;
}

/** "3.4%" for a permille share: one decimal, exact. */
export function permilleText(permille: number): string {
  return `${(permille / 10).toFixed(1)}%`;
}
