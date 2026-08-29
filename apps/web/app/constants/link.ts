import type { KunUIColor } from '@kungal/ui-core'

// Link status wire values (mirror apps/api internal/model constants).
export const LINK_STATUS_ACTIVE = 0
export const LINK_STATUS_DISABLED = 1
export const LINK_STATUS_ARCHIVED = 2

export interface LinkStatusMeta {
  value: number
  label: string
  // Status wears reserved status colors, never a categorical series hue, and
  // always ships with its icon + label so it never reads by color alone.
  color: KunUIColor
  icon: string
  description: string
}

export const LINK_STATUS_META: readonly LinkStatusMeta[] = [
  {
    value: LINK_STATUS_ACTIVE,
    label: '启用',
    color: 'success',
    icon: 'lucide:circle-check',
    description: '正常跳转'
  },
  {
    value: LINK_STATUS_DISABLED,
    label: '停用',
    color: 'warning',
    icon: 'lucide:circle-pause',
    description: '访问返回 410'
  },
  {
    value: LINK_STATUS_ARCHIVED,
    label: '归档',
    color: 'default',
    icon: 'lucide:archive',
    description: '保留记录，不再跳转'
  }
] as const

export const LINK_STATUS_OPTIONS = LINK_STATUS_META.map((s) => ({
  value: s.value,
  label: s.label
}))

// The status filter's "no filter" value. It sits outside the wire range, so it
// can never collide with a real status (mirrors engine.StatusAny).
export const LINK_STATUS_ANY = -1

export const LINK_STATUS_FILTER_OPTIONS = [
  { value: LINK_STATUS_ANY, label: '全部状态' },
  ...LINK_STATUS_OPTIONS
]

// statusMeta resolves a wire status to its display metadata, falling back to
// "archived" so an unknown value still renders something honest.
export const statusMeta = (status: number): LinkStatusMeta =>
  LINK_STATUS_META.find((s) => s.value === status) ?? LINK_STATUS_META[2]!

// Time windows the dashboard filter row offers. `days` is the API's `range`;
// a one-day window is read at hour resolution, longer ones at day resolution.
export interface StatsRange {
  days: number
  label: string
  granularity: 'hour' | 'day'
}

export const STATS_RANGES: readonly StatsRange[] = [
  { days: 1, label: '24 小时', granularity: 'hour' },
  { days: 7, label: '7 天', granularity: 'day' },
  { days: 14, label: '14 天', granularity: 'day' },
  { days: 30, label: '30 天', granularity: 'day' }
] as const

export const DEFAULT_RANGE_DAYS = 7

export const rangeMeta = (days: number): StatsRange =>
  STATS_RANGES.find((r) => r.days === days) ?? STATS_RANGES[1]!

// Links created through the dashboard record "dashboard"; everything else is
// the S2S key name of the sibling product that minted it.
export const DASHBOARD_SOURCE = 'dashboard'

export const sourceLabel = (via: string): string =>
  via === DASHBOARD_SOURCE ? '控制台' : via || '未知来源'

// How the link table can be ordered. These are wire values, not comparators:
// ordering happens in SQL over the WHOLE inventory, because sorting the page
// the reader is holding would only ever rank that page.
export const LINK_SORT_OPTIONS = [
  { value: 'range', label: '本期访问' },
  { value: 'total', label: '总访问' },
  { value: 'created', label: '创建时间' },
  { value: 'alias', label: '别名' }
] as const

export type LinkSortKey = (typeof LINK_SORT_OPTIONS)[number]['value']

// Page sizes the inventory offers. The list is one page at a time on purpose:
// it grows without bound (siblings mint links over S2S all day), and a table
// that renders everything silently stops at whatever cap the server picked.
export const LINK_PAGE_SIZES = [20, 50, 100] as const

export const DEFAULT_LINK_PAGE_SIZE = 20

export const LINK_PAGE_SIZE_OPTIONS = LINK_PAGE_SIZES.map((n) => ({
  value: n,
  label: `${n} 条/页`
}))
