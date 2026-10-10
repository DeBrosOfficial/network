import type { Head } from "../../model/types";
import { nextHead } from "../head";
import { ChainReadError } from "./client";
import { iso, rec, str, uint } from "./wire";

/**
 * The chain's head over the gateway's server-sent event stream (/v1/chain/head/stream,
 * core/pkg/gateway/handlers/chainread/head.go). The gateway reads the node once a second for all its
 * readers and tells each open page when the height moves, so a page asks for nothing: it listens.
 *
 * There is no other way to learn the head here. A browser without EventSource, or a stream that
 * cannot be opened, is an error the page shows; it is not met by asking for the status on a timer,
 * which would be the per-reader load the stream exists to remove. EventSource reconnects by itself
 * when the connection drops or the gateway ends a stream (it does, after 100 s).
 */

/** What a feed needs of an EventSource, so a test can stand in for one. */
export interface EventSourceLike {
  addEventListener(type: string, listener: (event: { data?: unknown }) => void): void;
  close(): void;
  onerror: ((event: Event) => unknown) | null;
}

export type EventSourceFactory = (url: string) => EventSourceLike;

/** How long the first head may take to arrive before the reader is told the chain could not be reached. */
const FIRST_HEAD_TIMEOUT_MS = 15_000;
const MAX_CHAIN_ID = 64;

const CONNECTION_LOST = "The live connection to the chain was lost; it reconnects by itself.";
const UNREACHABLE = "The chain could not be reached.";
const NO_EVENT_SOURCE = "This browser cannot follow the chain live.";

/** The browser's EventSource, or null where there is none (a server render, an old browser). */
export function browserEventSource(): EventSourceFactory | null {
  if (typeof EventSource === "undefined") return null;
  return (url) => new EventSource(url);
}

export function readHeadEvent(data: unknown): Head {
  if (typeof data !== "string") throw new ChainReadError("The chain sent a malformed head.");
  let raw: unknown;
  try {
    raw = JSON.parse(data);
  } catch {
    throw new ChainReadError("The chain sent a malformed head.");
  }
  const h = rec(raw, "head");
  return { height: uint(h.height, "block height"), time: iso(h.time, "block time"), chainId: str(h.chain_id, "chain id", MAX_CHAIN_ID) };
}

type Subscriber = { listener: (head: Head) => void; onError: (error: Error) => void; last: Head | null };

export interface HeadFeed {
  /** The current head: the one the stream sends when it opens, or the newest since. Rejects if none arrives. */
  current(): Promise<Head>;
  /**
   * Call `listener` whenever the head advances (or the chain id changes, which is a reset and may be
   * a lower height). It is not called for the head already known. `onError` is called whenever the
   * stream says the chain cannot be read or the connection drops; the subscription goes on.
   */
  subscribe(listener: (head: Head) => void, onError: (error: Error) => void): () => void;
}

/** One stream shared by every reader, opened when the first needs it and closed when the last is done. */
export function createHeadFeed(streamUrl: string, open: EventSourceFactory | null): HeadFeed {
  const subscribers = new Set<Subscriber>();
  const waiting = new Set<{ resolve: (h: Head) => void; reject: (e: Error) => void }>();
  let source: EventSourceLike | null = null;
  let last: Head | null = null;

  const fail = (error: Error) => {
    for (const s of subscribers) s.onError(error);
  };
  const failWaiting = (error: Error) => {
    for (const w of waiting) w.reject(error);
    waiting.clear();
    release();
  };

  const onHead = (event: { data?: unknown }) => {
    let head: Head;
    try {
      head = readHeadEvent(event.data);
    } catch (err) {
      fail(err instanceof Error ? err : new ChainReadError("The chain sent a malformed head."));
      return;
    }
    last = nextHead(last, head);
    for (const w of waiting) w.resolve(last);
    waiting.clear();
    release();
    for (const s of subscribers) {
      if (s.last !== null && (s.last.chainId !== head.chainId || head.height > s.last.height)) s.listener(head);
      s.last = nextHead(s.last, head);
    }
  };

  const connect = () => {
    if (source !== null) return;
    if (open === null) throw new ChainReadError(NO_EVENT_SOURCE);
    const opened = open(streamUrl);
    opened.addEventListener("head", onHead);
    opened.addEventListener("unavailable", () => {
      const error = new ChainReadError(UNREACHABLE);
      failWaiting(error);
      fail(error);
    });
    opened.onerror = () => fail(new ChainReadError(CONNECTION_LOST));
    source = opened;
  };

  function release() {
    if (subscribers.size > 0 || waiting.size > 0 || source === null) return;
    source.close();
    source = null;
    last = null;
  }

  return {
    current(): Promise<Head> {
      if (last !== null) return Promise.resolve(last);
      return new Promise<Head>((resolve, reject) => {
        try {
          connect();
        } catch (err) {
          reject(err);
          return;
        }
        const timer = setTimeout(() => {
          waiting.delete(entry);
          entry.reject(new ChainReadError(UNREACHABLE));
          release();
        }, FIRST_HEAD_TIMEOUT_MS);
        const entry = {
          resolve: (h: Head) => {
            clearTimeout(timer);
            resolve(h);
          },
          reject: (e: Error) => {
            clearTimeout(timer);
            reject(e);
          },
        };
        waiting.add(entry);
      });
    },

    subscribe(listener, onError): () => void {
      const subscriber: Subscriber = { listener, onError, last };
      try {
        connect();
      } catch (err) {
        onError(err instanceof Error ? err : new ChainReadError(NO_EVENT_SOURCE));
        return () => undefined;
      }
      subscribers.add(subscriber);
      return () => {
        subscribers.delete(subscriber);
        release();
      };
    },
  };
}
