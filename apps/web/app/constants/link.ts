// Link status wire values (mirror apps/api internal/model constants).
export const LINK_STATUS_ACTIVE = 0
export const LINK_STATUS_DISABLED = 1
export const LINK_STATUS_ARCHIVED = 2

export const LINK_STATUS_OPTIONS = [
  { value: LINK_STATUS_ACTIVE, label: '启用' },
  { value: LINK_STATUS_DISABLED, label: '停用' },
  { value: LINK_STATUS_ARCHIVED, label: '归档' }
] as const

export const LINK_STATUS_LABEL: Record<number, string> = {
  [LINK_STATUS_ACTIVE]: '启用',
  [LINK_STATUS_DISABLED]: '停用',
  [LINK_STATUS_ARCHIVED]: '归档'
}

// KunChip color per status.
export const LINK_STATUS_COLOR: Record<
  number,
  'success' | 'warning' | 'default'
> = {
  [LINK_STATUS_ACTIVE]: 'success',
  [LINK_STATUS_DISABLED]: 'warning',
  [LINK_STATUS_ARCHIVED]: 'default'
}

// Stats range choices (days) for the dashboard stats panel.
export const STATS_RANGES = [7, 14, 30] as const
