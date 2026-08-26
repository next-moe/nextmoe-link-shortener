<script setup lang="ts">
// The dashboard's main chart, with its table-view twin.
//
// The toggle is not a nicety: the tooltip must never be the ONLY way to read a
// value, so every point the chart draws is also reachable as text. Same data,
// same order, no filtering between the two views.
import type { AreaSeries } from '~/components/chart/Area.vue'
import type { SeriesPoint } from '~~/shared/utils/series'

const props = defineProps<{
  points: SeriesPoint[]
  granularity: 'hour' | 'day'
  pending: boolean
}>()

const view = ref<'chart' | 'table'>('chart')

const series = computed<AreaSeries[]>(() => [
  {
    key: 'visits',
    label: '访问',
    color: SERIES_VISITS,
    fill: true,
    value: (p) => p.visits
  },
  {
    key: 'unique',
    label: '独立访客',
    color: SERIES_UNIQUE,
    value: (p) => p.unique
  }
])

const hasTraffic = computed(() => props.points.some((p) => p.visits > 0))

// Newest first in the table — the reader scanning text wants the latest row at
// the top, the opposite of the chart's left-to-right reading order.
const rows = computed(() => [...props.points].reverse())

const rowLabel = (point: SeriesPoint): string =>
  props.granularity === 'hour'
    ? `${formatDate(point.at)} ${formatHour(point.at)}`
    : formatDate(point.at)
</script>

<template>
  <!-- One child: KunCard's content wrapper is `justify-between`, so a card with
       several direct children would be spread apart by the grid's stretch. -->
  <KunCard>
    <div class="flex flex-col gap-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-col gap-0.5">
          <h2 class="font-semibold">访问趋势</h2>
          <p class="text-xs text-default-400">
            按{{ granularity === 'hour' ? '小时' : '天' }}聚合 · 本地时区
          </p>
        </div>

        <KunTab
          v-model="view"
          size="sm"
          variant="pills"
          :items="[
            { value: 'chart', textValue: '图表', icon: 'lucide:chart-area' },
            { value: 'table', textValue: '表格', icon: 'lucide:table' }
          ]"
        />
      </div>

      <!-- Refetch holds the previous render at reduced opacity: no skeleton
           flash, no layout jump, and the axis never moves under the pointer. -->
      <div
        class="transition-opacity duration-200"
        :class="pending ? 'opacity-50' : ''"
      >
        <KunNull
          v-if="!points.length"
          description="所选时间范围内没有访问数据"
        />

        <ChartArea
          v-else-if="view === 'chart'"
          :points="points"
          :series="series"
          :granularity="granularity"
        />

        <div v-else class="max-h-[300px] overflow-y-auto">
          <table class="w-full text-sm">
            <thead
              class="sticky top-0 bg-content1 text-xs text-default-500 [&_th]:px-2 [&_th]:py-2 [&_th]:font-normal"
            >
              <tr class="border-b border-kun">
                <th class="text-left">时间</th>
                <th class="text-right">访问</th>
                <th class="text-right">独立访客</th>
              </tr>
            </thead>
            <tbody class="[&_td]:px-2 [&_td]:py-1.5">
              <tr
                v-for="p in rows"
                :key="p.at.getTime()"
                class="border-b border-kun/60 last:border-0"
              >
                <td class="tabular-nums text-default-600">{{ rowLabel(p) }}</td>
                <td class="text-right font-medium tabular-nums">
                  {{ formatNumber(p.visits) }}
                </td>
                <td class="text-right tabular-nums text-default-500">
                  {{ formatNumber(p.unique) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <p v-if="points.length && !hasTraffic" class="text-xs text-default-400">
        本期没有任何访问，图中为零基线。
      </p>
    </div>
  </KunCard>
</template>
