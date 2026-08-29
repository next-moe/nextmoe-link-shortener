// The dashboard's aggregates: one payload, one refresh, scoped to the shared
// time range (useStatsRange).
//
// The range scopes everything below it — tiles, chart, breakdowns, table — so
// the numbers on screen always agree. Switching it is the container's job, not
// this composable's: the range now drives two loaders (these aggregates and
// the paged inventory), and only the page that owns both can reload both.
// A refetch deliberately keeps `data` in place and only raises `pending`, so
// the charts hold their previous render instead of flashing a skeleton.
//
// This load stays on the client on purpose. The series is bucketed against the
// VIEWER's midnight (shared/utils/series.ts) and its axis is formatted in the
// viewer's timezone, so a server render would be built on the server's clock
// and the client would hydrate a different axis. `error` is what separates
// "the fetch failed" from "nothing has been fetched yet" — the two used to
// share a branch, and the console's first paint was an error message.
import type { OverviewDTO } from '~~/shared/types/shortlink'

export const useOverview = () => {
  const { overview } = useApi()

  const range = useStatsRange()
  const data = useState<OverviewDTO | null>('overview-data', () => null)
  const pending = useState<boolean>('overview-pending', () => false)
  const error = useState<boolean>('overview-error', () => false)

  const load = async () => {
    pending.value = true
    try {
      data.value = await overview(range.value)
      error.value = false
    } catch {
      error.value = true
      useKunMessage('加载控制台数据失败', 'error')
    } finally {
      pending.value = false
    }
  }

  // The series is dense and in the viewer's timezone; the granularity follows
  // the window (a 24-hour view reads by hour, longer windows by day).
  const granularity = computed(() => rangeMeta(range.value).granularity)

  const points = computed(() =>
    data.value
      ? buildSeries(data.value.series, data.value.range_days, granularity.value)
      : []
  )

  return { range, data, pending, error, load, granularity, points }
}
