<script setup lang="ts">
// The link inventory. A real table, because the columns must line up: this is
// the surface where an admin compares thirteen rows, not reads one.
//
// Search / status / sort live in a single row above the table. Clicking a row
// opens the detail drawer — the per-link stats used to sit in a sibling column,
// which left a column of dead space as tall as the list.
import type { OverviewLinkDTO } from '~~/shared/types/shortlink'

const props = defineProps<{
  rows: OverviewLinkDTO[]
  pending: boolean
  rangeLabel: string
}>()

const emit = defineEmits<{
  select: [OverviewLinkDTO]
  edit: [OverviewLinkDTO]
  remove: [OverviewLinkDTO]
  create: []
}>()

const query = ref('')
const statusFilter = ref<number | -1>(-1)
const sortKey = ref<LinkSortKey>('range')

const statusOptions = [
  { value: -1, label: '全部状态' },
  ...LINK_STATUS_OPTIONS
]

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return props.rows.filter((row) => {
    if (statusFilter.value !== -1 && row.link.status !== statusFilter.value) {
      return false
    }
    if (!q) {
      return true
    }
    return (
      row.link.alias.toLowerCase().includes(q) ||
      row.link.destination_url.toLowerCase().includes(q) ||
      row.link.description.toLowerCase().includes(q)
    )
  })
})

const sorted = computed(() => {
  const rows = [...filtered.value]
  switch (sortKey.value) {
    case 'total':
      return rows.sort((a, b) => b.link.visit_count - a.link.visit_count)
    case 'created':
      return rows.sort(
        (a, b) =>
          new Date(b.link.created_at).getTime() -
          new Date(a.link.created_at).getTime()
      )
    case 'alias':
      return rows.sort((a, b) => a.link.alias.localeCompare(b.link.alias))
    default:
      return rows.sort((a, b) => b.range_visits - a.range_visits)
  }
})

// The in-range bar is scaled against the busiest visible row, so the column
// reads as a comparison within what the reader is actually looking at.
const maxRangeVisits = computed(() =>
  Math.max(1, ...sorted.value.map((r) => r.range_visits))
)

const copy = async (row: OverviewLinkDTO) => {
  await navigator.clipboard.writeText(row.link.short_url)
  useKunMessage(`已复制 ${row.link.short_url}`, 'success')
}

const isFiltered = computed(
  () => query.value.trim() !== '' || statusFilter.value !== -1
)

const reset = () => {
  query.value = ''
  statusFilter.value = -1
}
</script>

<template>
  <KunCard padding="none" class="overflow-hidden">
    <div class="flex flex-col gap-4 border-b border-kun p-4 sm:p-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-baseline gap-2">
          <h2 class="font-semibold">短链列表</h2>
          <span class="text-xs tabular-nums text-default-400">
            {{ sorted.length }} / {{ rows.length }} 条
          </span>
        </div>
        <KunButton size="sm" icon @click="emit('create')">
          <template #icon>
            <KunIcon name="lucide:plus" />
          </template>
          新建短链
        </KunButton>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <div class="min-w-52 flex-1">
          <KunInput
            v-model="query"
            size="sm"
            placeholder="搜索别名 / 目标地址 / 备注"
            is-clearable
            aria-label="搜索短链"
          />
        </div>
        <KunSelect
          v-model="statusFilter"
          size="sm"
          :options="statusOptions"
          aria-label="按状态筛选"
          class-name="w-32"
        />
        <KunSelect
          v-model="sortKey"
          size="sm"
          :options="LINK_SORT_OPTIONS"
          aria-label="排序方式"
          class-name="w-32"
        />
      </div>
    </div>

    <div
      class="transition-opacity duration-200"
      :class="pending ? 'opacity-50' : ''"
    >
      <div v-if="!rows.length" class="p-6">
        <KunNull description="还没有短链，点击右上角新建第一条" />
      </div>

      <div v-else-if="!sorted.length" class="flex flex-col items-center gap-3 p-6">
        <KunNull description="没有符合条件的短链" />
        <KunButton v-if="isFiltered" variant="flat" size="sm" @click="reset">
          清除筛选
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
              <th class="w-44 text-right">{{ rangeLabel }}访问</th>
              <th class="w-20 text-right">总访问</th>
              <th class="w-24 text-right">操作</th>
            </tr>
          </thead>

          <tbody>
            <tr
              v-for="row in sorted"
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
  </KunCard>
</template>
