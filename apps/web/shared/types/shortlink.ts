// Wire shapes of the Go API (mirrors apps/api/internal/httpapi DTOs). The
// generated shared/types/api.d.ts is the authoritative OpenAPI-derived type
// set; these hand aliases are the ergonomic subset the components consume.

export interface LinkDTO {
  id: number
  alias: string
  short_url: string
  destination_url: string
  description: string
  status: number
  expires_at: string | null
  max_visits: number
  visit_count: number
  last_visited_at: string | null
  forward_params: boolean
  created_by: number
  created_via: string
  created_at: string
}

export interface BucketDTO {
  bucket_start: string
  visits: number
  unique_ips: number
}

export interface VisitDTO {
  id: number
  ip: string
  user_agent: string
  referer: string
  is_unique: boolean
  created_at: string
}

export interface LinkStatsDTO {
  link: LinkDTO
  range_days: number
  unique_visitors: number
  range_visits: number
  range_unique: number
  buckets: BucketDTO[]
  recent: VisitDTO[]
  referrers: ReferrerDTO[]
}

export interface KeyDTO {
  id: number
  name: string
  key_prefix: string
  disabled: boolean
  last_used_at: string | null
  created_by: number
  created_at: string
}

export interface CreateLinkPayload {
  destination_url: string
  alias?: string
  description?: string
  expires_at?: string | null
  max_visits?: number
  forward_params?: boolean
}

export interface UpdateLinkPayload {
  destination_url: string
  description: string
  status: number
  expires_at: string | null
  max_visits: number
  forward_params: boolean
}

export interface ReferrerDTO {
  // Empty host = the visit carried no Referer (direct traffic).
  host: string
  visits: number
}

export interface OverviewTotalsDTO {
  links: number
  active_links: number
  disabled_links: number
  archived_links: number
  all_time_visits: number
  range_visits: number
  range_unique: number
  prev_visits: number
  prev_unique: number
  keys: number
  active_keys: number
}

export interface OverviewLinkDTO {
  link: LinkDTO
  range_visits: number
  range_unique: number
}

export interface OverviewSourceDTO {
  created_via: string
  links: number
  range_visits: number
}

export interface OverviewDTO {
  range_days: number
  range_start: string
  totals: OverviewTotalsDTO
  // Hourly buckets summed across links; sparse (empty hours are omitted).
  series: BucketDTO[]
  links: OverviewLinkDTO[]
  sources: OverviewSourceDTO[]
  referrers: ReferrerDTO[]
}
