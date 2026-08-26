// Turning the API's sparse hourly buckets into a plottable series.
//
// The API deliberately returns only the hours that saw traffic, and in UTC-
// anchored hour steps. Rolling those up to days happens HERE, on the client,
// so a day boundary lands on the *viewer's* midnight rather than the server's —
// and so the axis is gap-filled, which is what makes a quiet stretch read as
// zero traffic instead of a missing segment.

import type { BucketDTO } from '~~/shared/types/shortlink'

export interface SeriesPoint {
  /** Start of the bucket, in local time. */
  at: Date
  visits: number
  unique: number
}

const HOUR_MS = 60 * 60 * 1000

// floorTo snaps a date down to the start of its local hour or local day.
const floorTo = (date: Date, granularity: 'hour' | 'day'): Date => {
  const out = new Date(date)
  out.setMinutes(0, 0, 0)
  if (granularity === 'day') {
    out.setHours(0)
  }
  return out
}

// step advances one bucket. Day steps go through setDate so a DST transition
// still lands on the next local midnight rather than drifting an hour.
const step = (date: Date, granularity: 'hour' | 'day'): Date => {
  const out = new Date(date)
  if (granularity === 'day') {
    out.setDate(out.getDate() + 1)
  } else {
    out.setTime(out.getTime() + HOUR_MS)
  }
  return out
}

// buildSeries folds sparse buckets into a dense, ordered series covering the
// whole window — every slot present, quiet ones at zero.
export const buildSeries = (
  buckets: readonly BucketDTO[],
  rangeDays: number,
  granularity: 'hour' | 'day'
): SeriesPoint[] => {
  const totals = new Map<number, { visits: number; unique: number }>()
  for (const b of buckets) {
    const key = floorTo(new Date(b.bucket_start), granularity).getTime()
    const row = totals.get(key) ?? { visits: 0, unique: 0 }
    row.visits += b.visits
    row.unique += b.unique_ips
    totals.set(key, row)
  }

  // The window ends at the current bucket and reaches back `rangeDays`. Anchor
  // on the end so the newest point is always the rightmost one.
  const end = floorTo(new Date(), granularity)
  const windowStart = new Date(Date.now() - rangeDays * 24 * HOUR_MS)

  // The window opens mid-bucket (a range is "7 days back from now", not "from
  // midnight"), so the first bucket only ever holds a fraction of its span and
  // would draw a dip that is an artifact of the window, not of the traffic.
  // Start at the first COMPLETE bucket instead. The trailing bucket is left in
  // place: "so far today" is what the reader expects at the right edge.
  const floored = floorTo(windowStart, granularity)
  const start =
    floored.getTime() === windowStart.getTime()
      ? floored
      : step(floored, granularity)

  const out: SeriesPoint[] = []
  for (let at = start; at <= end; at = step(at, granularity)) {
    const row = totals.get(at.getTime())
    out.push({ at, visits: row?.visits ?? 0, unique: row?.unique ?? 0 })
  }
  return out
}

// sparkValues is the compact form a sparkline needs: just the magnitudes.
export const sparkValues = (points: readonly SeriesPoint[]): number[] =>
  points.map((p) => p.visits)

// niceTicks picks round axis values (0 / 50 / 100 …) covering [0, max]. Round
// numbers are what let the reader decode the points we did not label.
export const niceTicks = (max: number, count = 4): number[] => {
  if (max <= 0) {
    return [0, 1]
  }
  const rawStep = max / count
  const magnitude = 10 ** Math.floor(Math.log10(rawStep))
  const normalized = rawStep / magnitude
  const stepSize =
    (normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10) *
    magnitude
  const ticks: number[] = []
  for (let v = 0; v <= max + stepSize / 2; v += stepSize) {
    ticks.push(Math.round(v * 1000) / 1000)
  }
  return ticks.length > 1 ? ticks : [0, stepSize]
}
