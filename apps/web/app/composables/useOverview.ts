// The dashboard's shared state: one time range, one payload, one refresh.
//
// The range is the page's ONLY filter and it scopes everything below it —
// tiles, chart, breakdowns, table — so the numbers on screen always agree.
// A refetch deliberately keeps `data` in place and only raises `pending`, so
// the charts hold their previous render instead of flashing a skeleton.
import type { OverviewDTO } from '~~/shared/types/shortlink'

export const useOverview = () => {
  const { overview } = useApi()

  const range = useState<number>('overview-range', () => DEFAULT_RANGE_DAYS)
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

  const setRange = async (days: number) => {
    if (days === range.value) {
      return
    }
    range.value = days
    await load()
  }

  // The series is dense and in the viewer's timezone; the granularity follows
  // the window (a 24-hour view reads by hour, longer windows by day).
  const granularity = computed(() => rangeMeta(range.value).granularity)

  const points = computed(() =>
    data.value
      ? buildSeries(data.value.series, data.value.range_days, granularity.value)
      : []
  )

  return { range, data, pending, error, load, setRange, granularity, points }
}
