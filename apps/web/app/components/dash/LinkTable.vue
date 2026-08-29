<script setup lang="ts">
// The link inventory. A real table, because the columns must line up: this is
// the surface where an admin compares thirteen rows, not reads one.
//
// It shows ONE PAGE, and search / status / sort / page all resolve in the
// database. The previous version took the whole list from the overview payload
// and filtered it in the browser, which quietly capped the console: past the
// server's limit the extra links were invisible AND unsearchable, while the
// tile above the table went on printing the true total. A client can only ever
// filter the rows it is already holding, so the filter row has to talk to the
// server or it lies.
//
// Clicking a row opens the detail drawer — the per-link stats used to sit in a
// sibling column, which left a column of dead space as tall as the list.
import type { LinkRowDTO } from '~~/shared/types/shortlink'

const props = defineProps<{
  // The service's whole link count, from the aggregates above this table. It
  // is shown next to the filtered count so a narrowed view can never be read
  // as the whole inventory.
  totalLinks: number
  rangeLabel: string
}>()

const emit = defineEmits<{
  select: [LinkRowDTO]
  edit: [LinkRowDTO]
  remove: [LinkRowDTO]
  create: []
}>()

const {
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
  isFiltered,
  setPage,
  setPerPage,
  setStatus,
  setSort,
  setQuery,
  resetFilters
} = useLinks()

// KunInput and KunSelect are generic over what they carry, so their emits are
// typed as unions (`string | number`, `T | T[] | null`). These three controls
// are single-value, so the narrowing happens once here rather than widening
// the composable's API to shapes it never receives.
const onQuery = (value: string | number) => setQuery(String(value))
const onStatus = (value: unknown) => setStatus(Number(value))
const onSort = (value: unknown) => setSort(value as LinkSortKey)
const onPerPage = (value: unknown) => setPerPage(Number(value))

// The in-range bar is scaled against the busiest row ON THIS PAGE, so the
// column reads as a comparison within what the reader is actually looking at.
const maxRangeVisits = computed(() =>
  Math.max(1, ...rows.value.map((r) => r.range_visits))
)

// Which slice of the whole result set this page is, counted from 1. Stated
// explicitly because a pager alone leaves "20 of how many?" unanswered.
const firstRow = computed(() => (page.value - 1) * perPage.value + 1)
const lastRow = computed(() => firstRow.value + rows.value.length - 1)

const copy = async (row: LinkRowDTO) => {
  await navigator.clipboard.writeText(row.link.short_url)
  useKunMessage(`已复制 ${row.link.short_url}`, 'success')
}
</script>

<template>
  <KunCard padding="none" class="overflow-hidden">
    <div class="flex flex-col gap-4 border-b border-kun p-4 sm:p-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-baseline gap-2">
          <h2 class="font-semibold">短链列表</h2>
          <span class="text-xs tabular-nums text-default-400">
            <template v-if="isFiltered">
              筛选出 {{ formatNumber(total) }} 条 · 全部
              {{ formatNumber(props.totalLinks) }} 条
            </template>
            <template v-else>共 {{ formatNumber(props.totalLinks) }} 条</template>
          </span>
        </div>
        <KunButton size="sm" icon @click="emit('create')">
          <template #icon>
            <KunIcon name="lucide:plus" />
          </template>
          新建短链
        </KunButton>
      </div>

      <!-- Every control here queries the database, not the page in hand. -->
      <div class="flex flex-wrap items-center gap-2">
        <div class="min-w-52 flex-1">
          <KunInput
            :model-value="query"
            size="sm"
            placeholder="搜索别名 / 目标地址 / 备注"
            is-clearable
            aria-label="搜索短链"
            @update:model-value="onQuery"
          />
        </div>
        <KunSelect
          :model-value="status"
          size="sm"
          :options="LINK_STATUS_FILTER_OPTIONS"
          aria-label="按状态筛选"
          class-name="w-32"
          @update:model-value="onStatus"
        />
        <KunSelect
          :model-value="sort"
          size="sm"
          :options="LINK_SORT_OPTIONS"
          aria-label="排序方式"
          class-name="w-32"
          @update:model-value="onSort"
        />
      </div>
    </div>

    <div
      class="transition-opacity duration-200"
      :class="pending && data ? 'opacity-50' : ''"
    >
      <!-- Nothing fetched yet: hold a page's worth of rows open so the card
           below does not get shoved down when the first page lands. -->
      <ul v-if="!data" class="divide-y divide-kun" aria-hidden="true">
        <li v-for="i in perPage" :key="`row-${i}`" class="px-4 py-3">
          <KunSkeleton height="2.75rem" rounded="lg" />
        </li>
      </ul>

      <div v-else-if="!total && !isFiltered" class="p-6">
        <KunNull description="还没有短链，点击右上角新建第一条" />
      </div>

      <div v-else-if="!total" class="flex flex-col items-center gap-3 p-6">
        <KunNull description="没有符合条件的短链" />
        <KunButton variant="flat" size="sm" @click="resetFilters">
          清除筛选
        </KunButton>
      </div>

      <!-- Rows matched, but not on this page: the inventory shrank under an
           open page faster than the composable could step back to a page that
           exists. Say so instead of showing an empty table under a non-zero
           total. -->
      <div v-else-if="!rows.length" class="flex flex-col items-center gap-3 p-6">
        <KunNull description="这一页已经没有内容了" />
        <KunButton variant="flat" size="sm" @click="setPage(1)">
          回到第一页
        </KunButton>
      </div>

      <!-- A plain overflow container, not KunScrollShadow: its horizontal mode
           lays children out as a `w-max` flex strip (built for card rows), which
           collapses a `w-full` table to its content width. -->
      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[880px] text-sm">
          <thead class="text-xs text-default-500">
            <tr class="border-b border-kun [&_th]:px-4 [&_th]:py-2.5 [&_th]:font-normal">
              <th class="text-left">短链</th>
              <th class="text-left">目标</th>
              <th class="text-left">状态</th>
              <th class="w-44 text-right">{{ props.rangeLabel }}访问</th>
              <th class="w-20 text-right">总访问</th>
              <th class="w-24 text-right">操作</th>
            </tr>
          </thead>

          <tbody>
            <tr
              v-for="row in rows"
              :key="row.link.id"
              class="cursor-pointer border-b border-kun/60 transition-colors last:border-0 hover:bg-default-100/70 [&_td]:px-4 [&_td]:py-3"
              tabindex="0"
              @click="emit('select', row)"
              @keydown.enter="emit('select', row)"
            >
              <td>
                <div class="flex flex-col gap-0.5">
                  <span class="font-mono text-sm font-medium text-primary">
                    /s/{{ row.link.alias }}
                  </span>
                  <span
                    v-if="row.link.description"
                    class="max-w-56 truncate text-xs text-default-400"
                  >
                    {{ row.link.description }}
                  </span>
                </div>
              </td>

              <td>
                <div class="flex max-w-72 flex-col gap-0.5">
                  <span class="truncate text-default-700">
                    {{ hostOf(row.link.destination_url) }}
                  </span>
                  <span class="truncate text-xs text-default-400">
                    {{ pathOf(row.link.destination_url) || '/' }}
                  </span>
                </div>
              </td>

              <td>
                <div class="flex flex-wrap items-center gap-1.5">
                  <!-- Status wears its icon and its label, never color alone. -->
                  <KunChip
                    :color="statusMeta(row.link.status).color"
                    size="sm"
                    variant="flat"
                  >
                    <template #start>
                      <KunIcon :name="statusMeta(row.link.status).icon" />
                    </template>
                    {{ statusMeta(row.link.status).label }}
                  </KunChip>
                  <KunTooltip
                    v-if="row.link.forward_params"
                    text="访问时把查询参数透传到目标地址"
                  >
                    <KunChip size="sm" variant="flat" color="default">
                      透传参数
                    </KunChip>
                  </KunTooltip>
                  <KunTooltip
                    v-if="row.link.expires_at"
                    :text="`过期时间 ${formatDateTime(row.link.expires_at)}`"
                  >
                    <KunChip size="sm" variant="flat" color="default">
                      <template #start>
                        <KunIcon name="lucide:clock" />
                      </template>
                      有期限
                    </KunChip>
                  </KunTooltip>
                </div>
              </td>

              <td>
                <div class="flex items-center justify-end gap-2">
                  <span class="h-1.5 w-20 overflow-hidden rounded-full bg-default-100">
                    <span
                      class="block h-full rounded-full"
                      :style="{
                        width: `${row.range_visits ? Math.max(4, (row.range_visits / maxRangeVisits) * 100) : 0}%`,
                        backgroundColor: SERIES_VISITS
                      }"
                    />
                  </span>
                  <span class="w-12 text-right font-medium tabular-nums">
                    {{ formatNumber(row.range_visits) }}
                  </span>
                </div>
                <p class="mt-1 text-right text-[11px] tabular-nums text-default-400">
                  {{ formatNumber(row.range_unique) }} 独立
                </p>
              </td>

              <td class="text-right tabular-nums text-default-500">
                {{ formatNumber(row.link.visit_count) }}
                <p class="mt-1 text-[11px] text-default-400">
                  {{ formatRelative(row.link.last_visited_at) }}
                </p>
              </td>

              <td>
                <div class="flex items-center justify-end gap-0.5">
                  <KunTooltip text="复制短链">
                    <KunButton
                      variant="light"
                      size="xs"
                      is-icon-only
                      aria-label="复制短链"
                      @click.stop="copy(row)"
                    >
                      <KunIcon name="lucide:copy" />
                    </KunButton>
                  </KunTooltip>
                  <KunTooltip text="编辑">
                    <KunButton
                      variant="light"
                      size="xs"
                      is-icon-only
                      aria-label="编辑"
                      @click.stop="emit('edit', row)"
                    >
                      <KunIcon name="lucide:pencil" />
                    </KunButton>
                  </KunTooltip>
                  <KunTooltip text="删除">
                    <KunButton
                      variant="light"
                      size="xs"
                      color="danger"
                      is-icon-only
                      aria-label="删除"
                      @click.stop="emit('remove', row)"
                    >
                      <KunIcon name="lucide:trash-2" />
                    </KunButton>
                  </KunTooltip>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- The footer states which slice of the whole set is on screen, next to
         the controls that move it. It stays mounted whenever there are rows,
         so paging never changes the card's chrome under the reader. -->
    <div
      v-if="data && total > 0"
      class="flex flex-col gap-3 border-t border-kun px-4 py-3 sm:px-5 lg:flex-row lg:items-center lg:justify-between"
    >
      <p class="text-xs tabular-nums text-default-400">
        <template v-if="rows.length">
          第 {{ formatNumber(firstRow) }}–{{ formatNumber(lastRow) }} 条，共
          {{ formatNumber(total) }} 条
        </template>
        <template v-else>共 {{ formatNumber(total) }} 条</template>
      </p>

      <div class="flex flex-wrap items-center gap-3 lg:justify-end">
        <KunSelect
          :model-value="perPage"
          size="sm"
          :options="LINK_PAGE_SIZE_OPTIONS"
          aria-label="每页条数"
          class-name="w-28"
          @update:model-value="onPerPage"
        />
        <KunPagination
          v-if="totalPages > 1"
          :current-page="page"
          :total-page="totalPages"
          :is-loading="pending"
          @update:current-page="setPage"
        />
      </div>
    </div>
  </KunCard>
</template>
