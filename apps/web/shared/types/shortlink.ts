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

// One row of the link inventory: the link plus its aggregates for the window
// the page is scoped to.
export interface LinkRowDTO {
  link: LinkDTO
  range_visits: number
  range_unique: number
}

// One page of the inventory. `total` counts every row the filters match, not
// the rows in `links` — it is what the pager describes and what the header
// reports, so the console never claims to be showing more than it has.
export interface LinkPageDTO {
  links: LinkRowDTO[]
  total: number
  page: number
  per_page: number
  total_pages: number
  range_days: number
}

// The query one page request carries. Every field resolves server-side: the
// inventory is unbounded, so filtering the page in hand would only ever search
// the rows the browser happens to be holding.
export interface LinkQuery {
  page: number
  per_page: number
  q: string
  // -1 = any status.
  status: number
  sort: string
  range: number
}

export interface OverviewSourceDTO {
  created_via: string
  links: number
  range_visits: number
}

// The console's aggregates. The link inventory is NOT here — it is paged
// separately (LinkPageDTO), because it is the one part of the page that grows
// without bound.
export interface OverviewDTO {
  range_days: number
  range_start: string
  totals: OverviewTotalsDTO
  // Hourly buckets summed across links; sparse (empty hours are omitted).
  series: BucketDTO[]
  sources: OverviewSourceDTO[]
  referrers: ReferrerDTO[]
}
