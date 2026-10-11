import type { ChainNode } from "./nodes";
import { hasRole, isLive } from "./nodes";
import type { Norama } from "./types";

/** The Tor relays x/relay has registered, and how the chain rewards them. */

export interface TorRelay {
  /** The relay's 20-byte RSA identity fingerprint, as 40 upper-case hex digits. */
  fingerprint: string;
  nodeId: string;
  operator: string;
  /** The IPv4 /16 the relay sits in ("A.B.0.0/16"); rewards are capped per /16. */
  prefix16: string;
  /** The operator's declaration. Pay follows what the reporters measure, not this. */
  exit: boolean;
  jailed: boolean;
}

export interface RelayParams {
  /** Distinct reporters that must submit a complete report before an epoch pays. */
  reportersQuorum: number;
  /** The median uptime below which a relay is paid nothing. */
  minUptime: number;
  exitMultiplier: number;
  perRelayCap: Norama;
  perOperatorCap: Norama;
  perPrefix16Cap: Norama;
}

export interface RelayEpoch {
  epoch: number;
  quorumMet: boolean;
  /** The epoch that switched relay rewards on; it mints nothing. */
  activating: boolean;
  ceiling: Norama;
  minted: Norama;
}

export interface TorRelays {
  relays: TorRelay[];
  truncated: boolean;
  params: RelayParams;
  /** The accounts allowed to report relay measurements. */
  reporters: string[];
  /** The settlement of the newest epoch past its report window, or null when the chain holds none for it. */
  lastSettlement: RelayEpoch | null;
  /** The epoch the lookup was for (0 or less before any epoch is past its window), so the page can name it when nothing was settled. */
  settlementEpoch: number;
}

export interface RelayRow {
  relay: TorRelay;
  /** The node the relay is registered on; null when the node list did not hold it. */
  node: ChainNode | null;
}

/** Each registered relay with its node. */
export function relayRows(relays: readonly TorRelay[], nodes: readonly ChainNode[]): RelayRow[] {
  const byId = new Map(nodes.map((n) => [n.id, n]));
  return relays.map((relay) => ({ relay, node: byId.get(relay.nodeId) ?? null }));
}

/** Live nodes that hold one of the Tor roles, by role. */
export function torNodes(nodes: readonly ChainNode[]): { relays: ChainNode[]; exits: ChainNode[]; authorities: ChainNode[] } {
  const live = nodes.filter(isLive);
  return {
    relays: live.filter((n) => hasRole(n, "relay")),
    exits: live.filter((n) => hasRole(n, "exit")),
    authorities: live.filter((n) => hasRole(n, "dirauth")),
  };
}

/** Live relay-role nodes that registered no relay with x/relay: they carry the role but earn no relay reward. */
export function unregisteredRelayNodes(relays: readonly TorRelay[], nodes: readonly ChainNode[]): ChainNode[] {
  const registered = new Set(relays.map((r) => r.nodeId));
  return nodes.filter((n) => isLive(n) && (hasRole(n, "relay") || hasRole(n, "exit")) && !registered.has(n.id));
}
