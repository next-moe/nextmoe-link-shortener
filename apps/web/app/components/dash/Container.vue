<script setup lang="ts">
// The console.
//
// Layout is one vertical rhythm — filter row → KPI tiles → traffic chart →
// breakdowns → table — with no ragged side column. The previous version put a
// short stats panel beside a long list, so the panel stretched to the list's
// height and left a screen-tall hole in the middle of the page.
import type { LinkDTO, OverviewLinkDTO } from '~~/shared/types/shortlink'

const { range, data, pending, error, load, setRange, granularity, points } =
  useOverview()
const { deleteLink } = useApi()

onMounted(load)

const showCreate = ref(false)
const showEdit = ref(false)
const showDrawer = ref(false)
const showDelete = ref(false)
const selected = ref<LinkDTO | null>(null)
const editing = ref<LinkDTO | null>(null)
const deleting = ref<LinkDTO | null>(null)
const deleteBusy = ref(false)

const totals = computed(() => data.value?.totals ?? null)
const rangeLabel = computed(() => rangeMeta(range.value).label)

// Tiles compare against the equally-long window before this one, which is what
// turns a bare count into a trend.
const visitsDelta = computed(() =>
  totals.value ? delta(totals.value.range_visits, totals.value.prev_visits) : null
)
const uniqueDelta = computed(() =>
  totals.value ? delta(totals.value.range_unique, totals.value.prev_unique) : null
)

const visitsSpark = computed(() => sparkValues(points.value))
const uniqueSpark = computed(() => points.value.map((p) => p.unique))

// The link-count tile splits by STATUS, so it wears the reserved status tokens
// rather than categorical series hues — a status color must never read as
// "series 3".
const statusSegments = computed(() => {
  const t = totals.value
  if (!t) {
    return []
  }
  return [
    {
      key: 'active',
      label: '启用',
      value: t.active_links,
      color: 'oklch(var(--success-500))'
    },
    {
      key: 'disabled',
      label: '停用',
      value: t.disabled_links,
      color: 'oklch(var(--warning-500))'
    },
    {
      key: 'archived',
      label: '归档',
      value: t.archived_links,
      color: 'oklch(var(--default-300))'
    }
  ]
})

const rangeShare = computed(() => {
  const t = totals.value
  return t && t.all_time_visits > 0 ? t.range_visits / t.all_time_visits : 0
})

const sourceSegments = computed(() =>
  (data.value?.sources ?? []).map((s) => ({
    key: s.created_via,
    label: sourceLabel(s.created_via),
    hint: `${formatNumber(s.links)} 条短链`,
    value: s.range_visits
  }))
)

const referrerRows = computed(() =>
  (data.value?.referrers ?? []).map((r) => ({
    key: r.host || '__direct',
    label: r.host || '直接访问',
    value: r.visits
  }))
)

const openDrawer = (row: OverviewLinkDTO) => {
  selected.value = row.link
  showDrawer.value = true
}

const openEdit = (link: LinkDTO) => {
  editing.value = link
  showEdit.value = true
}

const openDelete = (link: LinkDTO) => {
  deleting.value = link
  showDelete.value = true
}

const confirmDelete = async () => {
  if (!deleting.value || deleteBusy.value) {
    return
  }
  deleteBusy.value = true
  try {
    await deleteLink(deleting.value.id)
    useKunMessage('短链已删除', 'success')
    showDelete.value = false
    // The drawer may be showing the row that just went away.
    if (selected.value?.id === deleting.value.id) {
      showDrawer.value = false
    }
    await load()
  } catch {
    useKunMessage('删除失败，请重试', 'error')
  } finally {
    deleteBusy.value = false
  }
}

const handleSaved = async () => {
  showEdit.value = false
  await load()
  // Keep the open drawer in sync with what was just saved.
  if (selected.value) {
    selected.value =
      data.value?.links.find((l) => l.link.id === selected.value?.id)?.link ??
      selected.value
  }
}

const handleCreated = async (link: LinkDTO) => {
  await load()
  selected.value = link
  showDrawer.value = true
}
</script>

<template>
  <main class="mx-auto flex w-full max-w-[1400px] flex-col gap-6 px-4 py-6 sm:px-6 sm:py-8">
    <!-- One filter row, above everything it scopes. Every number below —
         tiles, chart, breakdowns, table — re-renders against this same slice,
         so the figures on screen always agree with each other. -->
    <header class="flex flex-wrap items-end justify-between gap-4">
      <div class="flex flex-col gap-1">
        <h1 class="text-2xl font-semibold">控制台</h1>
        <p class="text-sm text-default-500">
          <template v-if="totals">
            {{ formatNumber(totals.links) }} 条短链 ·
            {{ formatNumber(totals.active_links) }} 条启用中 ·
            {{ formatNumber(totals.active_keys) }} 个可用 API key
          </template>
          <template v-else>NextMoe 生态共享短链接服务</template>
        </p>
      </div>

      <div class="flex items-center gap-2">
        <KunTab
          :model-value="String(range)"
          size="sm"
          variant="pills"
          :items="STATS_RANGES.map((r) => ({ value: String(r.days), textValue: r.label }))"
          @update:model-value="setRange(Number($event))"
        />
        <KunTooltip text="刷新">
          <KunButton
            variant="flat"
            size="sm"
            is-icon-only
            aria-label="刷新数据"
            :loading="pending"
            @click="load"
          >
            <KunIcon name="lucide:refresh-ccw" />
          </KunButton>
        </KunTooltip>
      </div>
    </header>

    <template v-if="totals">
      <section
        class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4"
        aria-label="关键指标"
      >
        <DashStatTile
          label="本期访问"
          icon="lucide:mouse-pointer-click"
          :value="totals.range_visits"
          :delta="visitsDelta"
          :spark="visitsSpark"
          :hint="`对比上一个${rangeLabel}周期`"
        />
        <DashStatTile
          label="本期独立访客"
          icon="lucide:users"
          :value="totals.range_unique"
          :delta="uniqueDelta"
          :spark="uniqueSpark"
          :hint="`对比上一个${rangeLabel}周期`"
        />
        <DashStatTile
          label="短链总数"
          icon="lucide:link"
          :value="totals.links"
          :segments="statusSegments"
          hint="按状态划分"
        />
        <DashStatTile
          label="累计访问"
          icon="lucide:infinity"
          :value="totals.all_time_visits"
          hint="全部短链的历史总访问量"
        >
          <template #footer>
            <!-- A share of a whole is a meter, not a chart: how much of all
                 recorded traffic landed inside the selected window. -->
            <div class="flex flex-col gap-1.5">
              <KunProgress
                :value="rangeShare * 100"
                size="sm"
                color="primary"
                aria-label="本期访问占累计访问的比例"
              />
              <p class="text-[11px] text-default-400">
                其中 {{ formatPercent(rangeShare) }} 发生在本期
              </p>
            </div>
          </template>
        </DashStatTile>
      </section>

      <DashTrafficCard
        :points="points"
        :granularity="granularity"
        :pending="pending"
      />

      <!-- KunCard's content wrapper is `justify-between` so cards in a grid can
           pin a footer to the bottom edge. Each card therefore gets ONE child
           that owns its own stacking — otherwise the grid's equal-height
           stretch would spread these sections apart with a hole in between. -->
      <section class="grid grid-cols-1 items-start gap-4 lg:grid-cols-2">
        <KunCard>
          <div class="flex flex-col gap-4">
            <div class="flex flex-col gap-0.5">
              <h2 class="font-semibold">流量来源</h2>
              <p class="text-xs text-default-400">
                本期访问按创建这条短链的产品划分
              </p>
            </div>
            <ChartShareBar :segments="sourceSegments" />
          </div>
        </KunCard>

        <KunCard>
          <div class="flex flex-col gap-4">
            <div class="flex flex-col gap-0.5">
              <h2 class="font-semibold">引荐来源</h2>
              <p class="text-xs text-default-400">
                访问时携带的 Referer，按站点归并
              </p>
            </div>
            <ChartBarList :rows="referrerRows" unit="次" />
          </div>
        </KunCard>
      </section>

      <DashLinkTable
        :rows="data?.links ?? []"
        :pending="pending"
        :range-label="rangeLabel"
        @create="showCreate = true"
        @select="openDrawer"
        @edit="(row) => openEdit(row.link)"
        @remove="(row) => openDelete(row.link)"
      />
    </template>

    <!-- Branch on what is actually known, in that order: numbers, a failed
         load, or a load still in flight. The old order asked `pending` first,
         so the very first render — nothing fetched yet, nothing failed yet —
         fell through to the error state and the server shipped "加载失败" as
         the console's first paint. -->
    <KunNull
      v-else-if="error"
      description="加载控制台数据失败，请刷新重试"
    />

    <DashSkeleton v-else />

    <DashLinkDrawer
      v-model="showDrawer"
      :link="selected"
      :range="range"
      :granularity="granularity"
      @edit="openEdit"
      @remove="openDelete"
    />

    <DashCreateModal v-model="showCreate" @created="handleCreated" />
    <DashEditModal v-model="showEdit" :link="editing" @updated="handleSaved" />

    <KunModal v-model="showDelete" size="sm">
      <div class="flex flex-col gap-4">
        <h2 class="text-lg font-bold">删除短链</h2>
        <p class="text-sm text-default-500">
          确定删除
          <span class="font-mono text-foreground">/s/{{ deleting?.alias }}</span>
          吗？
          <template v-if="deleting?.visit_count">
            它已经被访问了
            {{ formatNumber(deleting.visit_count) }} 次，
          </template>
          访问记录会一并删除，此操作不可撤销。
        </p>
        <div class="flex justify-end gap-2">
          <KunButton variant="flat" @click="showDelete = false">取消</KunButton>
          <KunButton
            color="danger"
            :loading="deleteBusy"
            @click="confirmDelete"
          >
            删除
          </KunButton>
        </div>
      </div>
    </KunModal>
  </main>
</template>
