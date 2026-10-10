import type { Norama, ValidatorRef } from "./types";

/**
 * What the chain indexer keeps per closed emission epoch (chain/indexer/agg_model.go, EpochRow).
 * Every amount is the chain's own figure in norama. Minted and burned amounts are what happened
 * during the epoch's blocks, not what the schedule allows.
 */
export interface EpochRow {
  epoch: number;
  startHeight: number;
  endHeight: number;
  /** ISO times: when the epoch began and when the block that closed it was made. */
  startTime: string;
  endTime: string;
  blocks: number;
  /** The index began inside this epoch: its totals cover only the blocks the index saw. */
  partial: boolean;
  /** The schedule's ceiling for the epoch (x/emission). */
  maxMintable: Norama;
  /** The validator and delegator share, minted when the epoch closed. */
  mintedValidators: Norama;
  /** Storage and relay together: the chain keeps one running total for both. */
  mintedService: Norama;
  mintedDevelopment: Norama;
  mintedFaucet: Norama;
  mintedTotal: Norama;
  burned: Norama;
  feesCollected: Norama;
  feesBurned: Norama;
  feesDistributed: Norama;
  /** What x/power credited to earnings accounts when the epoch closed. */
  rewardsCredited: Norama;
  /** What x/power bonded into bootstrap committee members' self-delegations instead. */
  rewardsForceBonded: Norama;
  /**
   * The payout is known validator by validator (the chain reported it when the epoch closed). False
   * for an epoch that closed before it did: the three figures below are then null, and the
   * explorer says "not recorded", never zero.
   */
  rewardsRecorded: boolean;
  /** The validators' commission on what they were paid; null unless recorded. */
  rewardsCommission: Norama | null;
  /** The operators' whole share, commission included; null unless recorded. */
  rewardsOperatorShare: Norama | null;
  /** What delegators other than the operators were credited; null unless recorded. */
  rewardsDelegators: Norama | null;
}

/** One page of a list the indexer pages through. The indexer does not count, so `hasMore` is "this page was full". */
export interface IndexPage<T> {
  items: T[];
  page: number;
  limit: number;
  hasMore: boolean;
}

export interface EpochListQuery {
  /** 1 to 1000. */
  page: number;
  /** 1 to 100. */
  limit: number;
}

/** One validator in one closed epoch (chain/indexer/agg_model.go, ValidatorEpoch). */
export interface EpochValidator {
  ref: ValidatorRef;
  /** CometBFT voting power in that epoch. */
  power: number;
  signedBlocks: number;
  missedBlocks: number;
  /** Signed over signed plus missed, in basis points; 0 to 10000. */
  uptimeBps: number;
  slashes: number;
  slashedBurned: Norama;
  jailed: boolean;
  /** x/staking's name for the status: bonded, unbonding or unbonded. */
  status: string;
}

/** The supply breakdown after an epoch's closing block (chain/indexer/agg_model.go, SupplyPoint). */
export interface SupplyPoint {
  epoch: number;
  height: number;
  time: string;
  totalSupply: Norama;
  mintedToDate: Norama;
  burnedToDate: Norama;
  genesisSupply: Norama;
  bonded: Norama;
  unbonding: Norama;
  earningsPools: Norama;
  storageEscrow: Norama;
  otherProtocol: Norama;
  circulating: Norama;
}

/**
 * The epoch in progress, read from x/emission (the current epoch and its parameters) and the
 * head at the same moment. An epoch closes when BOTH the duration and the block count have passed.
 */
export interface EpochClock {
  epoch: number;
  /** ISO time the epoch in progress started (BFT time). */
  startedAt: string;
  durationSeconds: number;
  minBlocks: number;
  /** Blocks counted in the epoch when it was read. */
  blocksInEpoch: number;
  /** The chain height at that read, to advance the count with the head. */
  readAtHeight: number;
}
