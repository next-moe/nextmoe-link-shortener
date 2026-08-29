// useApi centralizes every dashboard call to the Go API. All calls go through
// the same-origin '/api' proxy so the httpOnly session cookie rides along —
// the whole dashboard is auth-gated, so there is no SSR/anonymous read path.
import type {
  CreateLinkPayload,
  KeyDTO,
  LinkDTO,
  LinkPageDTO,
  LinkQuery,
  LinkStatsDTO,
  OverviewDTO,
  UpdateLinkPayload
} from '~~/shared/types/shortlink'

export const useApi = () => {
  const apiFetch = <T>(
    path: string,
    opts: Parameters<typeof $fetch>[1] = {}
  ): Promise<T> =>
    $fetch<T>(`/api${path}`, { credentials: 'include', ...opts }) as Promise<T>

  return {
    // ---- dashboard read model ----
    // One call answers the console's aggregates: totals, the traffic series,
    // and the source / referrer breakdowns. The link inventory is NOT in here —
    // it pages separately, because it is the one part of the page that grows
    // without bound.
    overview: (range: number) =>
      apiFetch<OverviewDTO>(`/stats/overview?range=${range}`),

    // ---- links ----
    // One page of the inventory. Every field of the query resolves in the
    // database, so search and sort reach rows this page is not holding.
    listLinks: (query: LinkQuery) =>
      apiFetch<LinkPageDTO>(
        `/links?${new URLSearchParams({
          page: String(query.page),
          per_page: String(query.per_page),
          q: query.q,
          status: String(query.status),
          sort: query.sort,
          range: String(query.range)
        })}`
      ),
    createLink: (payload: CreateLinkPayload) =>
      apiFetch<LinkDTO>('/links', { method: 'POST', body: payload }),
    linkStats: (alias: string, range = 7) =>
      apiFetch<LinkStatsDTO>(
        `/links/${encodeURIComponent(alias)}/stats?range=${range}`
      ),
    updateLink: (id: number, payload: UpdateLinkPayload) =>
      apiFetch<LinkDTO>(`/links/${id}`, { method: 'PUT', body: payload }),
    deleteLink: (id: number) =>
      apiFetch<null>(`/links/${id}`, { method: 'DELETE' }),

    // ---- API keys ----
    listKeys: () => apiFetch<{ keys: KeyDTO[] }>('/keys'),
    createKey: (name: string) =>
      apiFetch<{ key: KeyDTO; plaintext: string }>('/keys', {
        method: 'POST',
        body: { name }
      }),
    setKeyDisabled: (id: number, disabled: boolean) =>
      apiFetch<{ ok: boolean }>(`/keys/${id}/disabled`, {
        method: 'PUT',
        body: { disabled }
      }),
    deleteKey: (id: number) =>
      apiFetch<null>(`/keys/${id}`, { method: 'DELETE' })
  }
}
