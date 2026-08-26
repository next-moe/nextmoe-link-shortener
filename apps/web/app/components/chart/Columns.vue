<script setup lang="ts">
// Per-bucket column chart for the link detail drawer. One series, so one hue
// and no legend box — the card title already names what is plotted.
//
// The mark IS the hit target here (no crosshair): each column carries its own
// hover/focus tooltip, and the hit area spans the whole band including the gap,
// so a quiet 2px-tall column is still easy to hit.
import type { SeriesPoint } from '~~/shared/utils/series'

const props = withDefaults(
  defineProps<{
    points: SeriesPoint[]
    granularity: 'hour' | 'day'
    height?: number
  }>(),
  { height: 150 }
)

const max = computed(() => Math.max(1, ...props.points.map((p) => p.visits)))
const active = ref<number | null>(null)

const label = (point: SeriesPoint): string =>
  props.granularity === 'hour'
    ? `${formatDate(point.at)} ${formatHour(point.at)}`
    : formatDate(point.at)

// Edge labels only — a tick under every column would collide and go unread.
const edgeLabels = computed(() => {
  if (!props.points.length) {
    return null
  }
  return {
    first: label(props.points[0]!),
    last: label(props.points.at(-1)!)
  }
})
</script>

<template>
  <div class="flex flex-col gap-2">
    <div
      class="relative flex items-end gap-0.5"
      :style="{ height: `${height}px` }"
      role="img"
      :aria-label="`每${granularity === 'hour' ? '小时' : '日'}访问量，共 ${points.length} 个数据点`"
    >
      <button
        v-for="(p, i) in points"
        :key="p.at.getTime()"
        type="button"
        class="group relative flex h-full flex-1 items-end focus-visible:outline-none"
        :aria-label="`${label(p)}：${p.visits} 次访问`"
        @pointerenter="active = i"
        @pointerleave="active = null"
        @focus="active = i"
        @blur="active = null"
      >
        <!-- Bars cap at 24px thick; the band's leftover is deliberate air. -->
        <span
          class="mx-auto block w-full max-w-6 rounded-t transition-opacity"
          :class="active !== null && active !== i ? 'opacity-50' : ''"
          :style="{
            height: `${Math.max(p.visits ? 2 : 1, (p.visits / max) * height)}px`,
            backgroundColor: p.visits ? SERIES_VISITS : CHART_GRID_COLOR
          }"
        />
      </button>

      <div
        v-if="active !== null && points[active]"
        class="pointer-events-none absolute -top-1 z-10 whitespace-nowrap rounded-lg border border-kun bg-content1 px-2.5 py-1.5 text-xs shadow-lg"
        :style="{
          left: `${((active + 0.5) / points.length) * 100}%`,
          transform: `translateX(${active > points.length * 0.6 ? '-100%' : '0'})`
        }"
      >
        <span class="font-semibold tabular-nums">
          {{ formatNumber(points[active]!.visits) }}
        </span>
        <span class="text-default-500"> 次访问 · </span>
        <span class="tabular-nums text-default-500">
          {{ formatNumber(points[active]!.unique) }} 独立
        </span>
        <span class="ml-1 text-default-400">{{ label(points[active]!) }}</span>
      </div>
    </div>

    <div
      v-if="edgeLabels"
      class="flex justify-between text-[11px] tabular-nums text-default-400"
    >
      <span>{{ edgeLabels.first }}</span>
      <span>{{ edgeLabels.last }}</span>
    </div>
  </div>
</template>
