/**
 * Chart geometry for amounts that do not fit a double. Values are bigint norama; only the final
 * 0-to-1 position is a number, so a 10^20 total never loses the difference between two points.
 */

const RATIO_SCALE = 100_000n;
const FLAT = 0.5;

export interface Extent {
  min: bigint;
  max: bigint;
}

/** The range a chart spans. `fromZero` pins the bottom to 0 (bars); a line fits its own values. */
export function extent(values: readonly bigint[], fromZero: boolean): Extent {
  if (values.length === 0) return { min: 0n, max: 0n };
  let min = fromZero ? 0n : values[0]!;
  let max = values[0]!;
  for (const v of values) {
    if (v < min) min = v;
    if (v > max) max = v;
  }
  return { min, max };
}

/** Where `value` sits between min (0) and max (1); a range of one value sits in the middle, or on the floor for bars. */
export function position(value: bigint, range: Extent, fromZero: boolean): number {
  const span = range.max - range.min;
  if (span <= 0n) return fromZero ? 0 : FLAT;
  const clamped = value < range.min ? range.min : value > range.max ? range.max : value;
  return Number(((clamped - range.min) * RATIO_SCALE) / span) / Number(RATIO_SCALE);
}

export interface Size {
  width: number;
  height: number;
}

/** The polyline of a line chart: first value on the left edge, last on the right, up is larger. */
export function linePoints(values: readonly bigint[], size: Size, range: Extent): string {
  if (values.length === 0) return "";
  const step = values.length > 1 ? size.width / (values.length - 1) : 0;
  return values
    .map((v, i) => `${(i * step).toFixed(1)},${(size.height - position(v, range, false) * size.height).toFixed(1)}`)
    .join(" ");
}

export interface Bar {
  x: number;
  width: number;
  y: number;
  height: number;
}

const BAR_GAP_SHARE = 0.2;

/** One bar per value, left to right, with a gap between them. A zero value is a bar of no height. */
export function bars(values: readonly bigint[], size: Size, range: Extent): Bar[] {
  if (values.length === 0) return [];
  const slot = size.width / values.length;
  const width = slot * (1 - BAR_GAP_SHARE);
  return values.map((v, i) => {
    const height = position(v, range, true) * size.height;
    return { x: i * slot + (slot - width) / 2, width, y: size.height - height, height };
  });
}

/** The ceiling drawn across a bar chart: a step per epoch, level over each bar's slot. */
export function stepPoints(values: readonly bigint[], size: Size, range: Extent): string {
  if (values.length === 0) return "";
  const slot = size.width / values.length;
  return values
    .flatMap((v, i) => {
      const y = (size.height - position(v, range, true) * size.height).toFixed(1);
      return [`${(i * slot).toFixed(1)},${y}`, `${((i + 1) * slot).toFixed(1)},${y}`];
    })
    .join(" ");
}
