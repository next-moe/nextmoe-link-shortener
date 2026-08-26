// Display formatting shared by the dashboard surfaces. Everything here is
// pure so it can run identically on the server and the client.

const NUMBER_FORMAT = new Intl.NumberFormat('zh-CN')

// formatNumber thousands-separates a count. Used wherever a number sits in a
// column and must line up (pair it with `tabular-nums`).
export const formatNumber = (n: number): string => NUMBER_FORMAT.format(n)

// formatCompact shortens a headline number (12.9K) so a stat tile never wraps.
// Large standalone values keep proportional figures — tabular digits make a
// number like 121 look loose at display sizes.
export const formatCompact = (n: number): string => {
  if (Math.abs(n) < 10_000) {
    return NUMBER_FORMAT.format(n)
  }
  if (Math.abs(n) < 1_000_000) {
    return `${(n / 1000).toFixed(n % 1000 === 0 ? 0 : 1)}K`
  }
  return `${(n / 1_000_000).toFixed(1)}M`
}

// formatPercent renders a 0-1 ratio at one decimal, dropping a trailing ".0".
export const formatPercent = (ratio: number): string => {
  const pct = ratio * 100
  return `${pct >= 10 || pct === 0 ? Math.round(pct) : pct.toFixed(1)}%`
}

export interface Delta {
  ratio: number
  label: string
  direction: 'up' | 'down' | 'flat'
}

// Past this much change a percentage stops being readable ("+21830%"), so the
// label switches to a multiplier ("×219") — same fact, legible at a glance.
const MULTIPLIER_THRESHOLD = 10

// delta compares a value with the previous period. A zero baseline has no
// meaningful percentage, so it reports the direction without inventing one.
export const delta = (current: number, previous: number): Delta => {
  if (current === previous) {
    return { ratio: 0, label: '持平', direction: 'flat' }
  }
  const direction = current > previous ? 'up' : 'down'
  if (previous === 0) {
    return { ratio: 1, label: '新增', direction }
  }
  const ratio = (current - previous) / previous
  if (Math.abs(ratio) >= MULTIPLIER_THRESHOLD) {
    const factor = current / previous
    return {
      ratio,
      label: `×${factor >= 100 ? Math.round(factor) : factor.toFixed(1)}`,
      direction
    }
  }
  return {
    ratio,
    label: `${ratio > 0 ? '+' : '−'}${formatPercent(Math.abs(ratio))}`,
    direction
  }
}

const DATE_TIME = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false
})

const DATE_ONLY = new Intl.DateTimeFormat('zh-CN', {
  month: '2-digit',
  day: '2-digit'
})

const HOUR_ONLY = new Intl.DateTimeFormat('zh-CN', {
  hour: '2-digit',
  minute: '2-digit',
  hour12: false
})

export const formatDateTime = (iso: string | null | undefined): string =>
  iso ? DATE_TIME.format(new Date(iso)) : '—'

export const formatDate = (value: string | Date): string =>
  DATE_ONLY.format(typeof value === 'string' ? new Date(value) : value)

export const formatHour = (value: string | Date): string =>
  HOUR_ONLY.format(typeof value === 'string' ? new Date(value) : value)

const RELATIVE = new Intl.RelativeTimeFormat('zh-CN', { numeric: 'auto' })

const RELATIVE_STEPS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['second', 60],
  ['minute', 60],
  ['hour', 24],
  ['day', 30],
  ['month', 12]
]

// formatRelative renders "3 小时前" style text, falling back to years.
export const formatRelative = (iso: string | null | undefined): string => {
  if (!iso) {
    return '从未'
  }
  let value = (Date.now() - new Date(iso).getTime()) / 1000
  for (const [unit, span] of RELATIVE_STEPS) {
    if (Math.abs(value) < span) {
      return RELATIVE.format(-Math.round(value), unit)
    }
    value /= span
  }
  return RELATIVE.format(-Math.round(value), 'year')
}

// hostOf reduces a URL to its host for compact display; unparseable input
// falls back to the raw string so nothing silently disappears.
export const hostOf = (raw: string): string => {
  try {
    return new URL(raw).host.replace(/^www\./, '')
  } catch {
    return raw
  }
}

// pathOf is the rest of a URL after the host — the half worth truncating.
export const pathOf = (raw: string): string => {
  try {
    const url = new URL(raw)
    return `${url.pathname}${url.search}${url.hash}`.replace(/^\/$/, '')
  } catch {
    return ''
  }
}
