// The link inventory: one page at a time, every filter resolved server-side.
//
// This used to ride along inside the overview payload — the whole list in one
// array, filtered and sorted in the browser. That only works while the list is
// small: the server capped it, and past the cap the extra links were invisible
// AND unsearchable, with the console still printing the true total in the tile
// right above them. Search in particular has to reach the database, because a
// client can only ever filter the rows it is already holding.
//
// State is shared (useState), so the container and the table read the same
// page without prop-drilling; only the debounce timer is per-caller, and only
// the search box uses it.
import type { LinkPageDTO } from '~~/shared/types/shortlink'

// How long the search box waits for typing to settle before it asks the
// server. Long enough that a word costs one request, short enough that the
// table still feels attached to the keyboard.
const SEARCH_DEBOUNCE_MS = 300

export const useLinks = () => {
  const { listLinks } = useApi()
  const range = useStatsRange()

  const page = useState<number>('links-page', () => 1)
  const perPage = useState<number>('links-per-page', () => DEFAULT_LINK_PAGE_SIZE)
  const query = useState<string>('links-query', () => '')
  const status = useState<number>('links-status', () => LINK_STATUS_ANY)
  const sort = useState<LinkSortKey>('links-sort', () => 'range')

  const data = useState<LinkPageDTO | null>('links-data', () => null)
  const pending = useState<boolean>('links-pending', () => false)
  const error = useState<boolean>('links-error', () => false)
  // Requests can land out of order — typing "abc" then backspacing to "ab"
  // fires two, and the slower one must not overwrite the newer one's rows.
  // Every load takes a ticket and only the newest one is allowed to write.
  const ticket = useState<number>('links-ticket', () => 0)

  const load = async (allowRetry = true): Promise<void> => {
    const mine = ++ticket.value
    pending.value = true
    try {
      const res = await listLinks({
        page: page.value,
        per_page: perPage.value,
        q: query.value.trim(),
        status: status.value,
        sort: sort.value,
        range: range.value
      })
      if (mine !== ticket.value) {
        return
      }
      // The open page can fall off the end underneath the reader: rows get
      // deleted, or a sibling product mints links and reshuffles the order.
      // Land on the last page that still exists instead of showing an empty
      // table beside a non-zero total. One retry only — the second answer is
      // taken as-is, so a list changing under us cannot loop.
      if (res.page > res.total_pages && allowRetry) {
        page.value = res.total_pages
        return await load(false)
      }
      data.value = res
      error.value = false
    } catch {
      if (mine !== ticket.value) {
        return
      }
      error.value = true
      useKunMessage('加载短链列表失败', 'error')
    } finally {
      if (mine === ticket.value) {
        pending.value = false
      }
    }
  }

  // Filter changes reset to page 1: page 7 of the previous result set means
  // nothing in the new one, and a reader who narrows a search should not land
  // on an empty page.
  const setPage = (next: number) => {
    page.value = next
    return load()
  }

  const setPerPage = (next: number) => {
    perPage.value = next
    page.value = 1
    return load()
  }

  const setStatus = (next: number) => {
    status.value = next
    page.value = 1
    return load()
  }

  const setSort = (next: LinkSortKey) => {
    sort.value = next
    page.value = 1
    return load()
  }

  let searchTimer: ReturnType<typeof setTimeout> | undefined
  const setQuery = (next: string) => {
    query.value = next
    page.value = 1
    clearTimeout(searchTimer)
    searchTimer = setTimeout(load, SEARCH_DEBOUNCE_MS)
  }

  const resetFilters = () => {
    clearTimeout(searchTimer)
    query.value = ''
    status.value = LINK_STATUS_ANY
    page.value = 1
    return load()
  }

  const rows = computed(() => data.value?.links ?? [])
  const total = computed(() => data.value?.total ?? 0)
  const totalPages = computed(() => data.value?.total_pages ?? 1)
  const isFiltered = computed(
    () => query.value.trim() !== '' || status.value !== LINK_STATUS_ANY
  )

  return {
    page,
    perPage,
    query,
    status,
    sort,
    data,
    rows,
    total,
    totalPages,
    pending,
    error,
    isFiltered,
    load,
    setPage,
    setPerPage,
    setStatus,
    setSort,
    setQuery,
    resetFilters
  }
}
