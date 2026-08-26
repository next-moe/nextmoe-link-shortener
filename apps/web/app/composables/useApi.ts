// useApi centralizes every dashboard call to the Go API. All calls go through
// the same-origin '/api' proxy so the httpOnly session cookie rides along —
// the whole dashboard is auth-gated, so there is no SSR/anonymous read path.
import type {
  CreateLinkPayload,
  KeyDTO,
  LinkDTO,
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
    // One call answers the whole console: totals, the traffic series, and the
    // per-link / source / referrer breakdowns.
    overview: (range: number) =>
      apiFetch<OverviewDTO>(`/stats/overview?range=${range}`),

    // ---- links ----
    listLinks: () => apiFetch<{ links: LinkDTO[] }>('/links'),
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
