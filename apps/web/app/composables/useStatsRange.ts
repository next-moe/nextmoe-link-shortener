// The one time window the console is scoped to.
//
// It lives on its own because two loaders now read it — the aggregates
// (useOverview) and the link inventory (useLinks) — and both have to agree.
// A range the two composables each owned a copy of would let the tiles and the
// table describe different weeks while looking like one page.
export const useStatsRange = () =>
  useState<number>('stats-range', () => DEFAULT_RANGE_DAYS)
