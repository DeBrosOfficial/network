import { parseExplorerPath } from "./routes";
import { shortAddress, shortHash } from "./units";

/** Long enough to follow a real investigation, short enough to stay readable. */
export const MAX_TRAIL = 12;

const SHARE_PARAM = "trail";
const SHARE_SEPARATOR = "|";

/**
 * The trail is the ordered list of explorer pages the reader has followed
 * since the home page. Visiting a page already on the trail goes back to it
 * (dropping what came after); visiting a new page appends. Home clears it.
 */
export function nextTrail(trail: readonly string[], path: string): string[] {
  if (parseExplorerPath(path).kind === "home") return [];
  const seen = trail.indexOf(path);
  if (seen >= 0) return trail.slice(0, seen + 1);
  const next = [...trail, path];
  return next.length > MAX_TRAIL ? next.slice(next.length - MAX_TRAIL) : next;
}

/** A path segment as the reader would type it; the segment as it is when it is not valid percent-encoding. */
function readableSegment(segment: string): string {
  try {
    return decodeURIComponent(segment);
  } catch {
    return segment;
  }
}

/** A short crumb label derived from the path alone. */
export function crumbLabel(path: string): string {
  const { kind, id } = parseExplorerPath(path);
  switch (kind) {
    case "tx":
      return `Tx ${shortHash(id)}`;
    case "wallet":
      return `Wallet ${shortAddress(id)}`;
    case "validator":
      return `Validator ${shortAddress(id)}`;
    case "block":
      return `Block ${id}`;
    case "validators":
      return "Validators";
    case "nodes":
      return "Nodes";
    case "node":
      return `Node ${readableSegment(id)}`;
    case "storage":
      return "Storage";
    case "tor":
      return "Tor";
    default:
      return "Explorer";
  }
}

const KNOWN_PATH = /^\/explorer(\/[a-z][a-z0-9-]{0,31})?\/(validators|nodes|storage|tor|(tx|wallet|block|validator)\/[A-Za-z0-9]+|node\/[A-Za-z0-9%][A-Za-z0-9._:%-]{0,191})$/;

/** True for a path this explorer serves. Used to reject a hostile shared trail. */
export function isTrailPath(path: string): boolean {
  return KNOWN_PATH.test(path);
}

/**
 * A link that reopens the current page with the whole trail attached. `query`
 * is what the page needs besides the trail to open again: a custom network's
 * gateway, and the chain id the trail was followed on.
 */
export function shareUrl(origin: string, current: string, trail: readonly string[], query: URLSearchParams = new URLSearchParams()): string {
  const q = new URLSearchParams(query);
  if (trail.length > 0) q.set(SHARE_PARAM, trail.join(SHARE_SEPARATOR));
  const text = q.toString();
  return text === "" ? `${origin}${current}` : `${origin}${current}?${text}`;
}

/**
 * Read a shared trail from a query string; anything that is not an explorer
 * path is dropped, and a path repeated later is dropped so each crumb is unique.
 */
export function readSharedTrail(search: string): string[] {
  const raw = new URLSearchParams(search).get(SHARE_PARAM);
  if (!raw) return [];
  return [...new Set(raw.split(SHARE_SEPARATOR).filter(isTrailPath))].slice(0, MAX_TRAIL);
}
