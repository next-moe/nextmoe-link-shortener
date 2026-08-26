<script setup lang="ts">
// Traffic-over-time chart: a filled area for the leading series plus a 2px
// line per series, on ONE shared axis (both series count visits, so a second
// y-scale would invent a correlation that isn't in the data).
//
// Interaction is part of the deliverable, not an upgrade: a crosshair snaps to
// the nearest x and one tooltip reports every series there, so the pointer
// never has to land on a 2px stroke. The values are also reachable without
// hovering — the parent card ships a table view.
import type { SeriesPoint } from '~~/shared/utils/series'

export interface AreaSeries {
  key: string
  label: string
  color: string
  /** Fill the area under this series. Only the leading series should. */
  fill?: boolean
  value: (point: SeriesPoint) => number
}

const props = withDefaults(
  defineProps<{
    points: SeriesPoint[]
    series: AreaSeries[]
    granularity: 'hour' | 'day'
    height?: number
  }>(),
  { height: 260 }
)

// The viewBox is a fixed coordinate space the SVG scales from; only the height
// is a real pixel budget, so the x-axis band always has room (a fixed height
// that excludes the axis is what produces a tiny nested scrollbar).
const VB_W = 1000
const PAD = { top: 16, right: 16, bottom: 28, left: 48 }

const vbHeight = computed(() => props.height)
const plotW = VB_W - PAD.left - PAD.right
const plotH = computed(() => vbHeight.value - PAD.top - PAD.bottom)

const maxValue = computed(() => {
  let max = 0
  for (const p of props.points) {
    for (const s of props.series) {
      max = Math.max(max, s.value(p))
    }
  }
  return max
})

const ticks = computed(() => niceTicks(maxValue.value))
const scaleMax = computed(() => ticks.value[ticks.value.length - 1] || 1)

const x = (i: number): number => {
  const n = props.points.length
  return PAD.left + (n <= 1 ? plotW / 2 : (i / (n - 1)) * plotW)
}

const y = (v: number): number =>
  PAD.top + plotH.value - (v / scaleMax.value) * plotH.value

const linePath = (s: AreaSeries): string =>
  props.points.map((p, i) => `${i ? 'L' : 'M'}${x(i)} ${y(s.value(p))}`).join(' ')

const areaPath = (s: AreaSeries): string => {
  if (!props.points.length) {
    return ''
  }
  const base = PAD.top + plotH.value
  return `${linePath(s)} L${x(props.points.length - 1)} ${base} L${x(0)} ${base} Z`
}

// X labels are thinned to ~6 so they never collide; the crosshair carries the
// exact position for every other point.
const xLabels = computed(() => {
  const n = props.points.length
  if (!n) {
    return []
  }
  const stride = Math.max(1, Math.ceil(n / 6))
  const out: { i: number; text: string }[] = []
  for (let i = n - 1; i >= 0; i -= stride) {
    const at = props.points[i]!.at
    out.unshift({
      i,
      text: props.granularity === 'hour' ? formatHour(at) : formatDate(at)
    })
  }
  return out
})

// ---- crosshair ----
const root = useTemplateRef<HTMLElement>('root')
const active = ref<number | null>(null)

// nearestIndex maps a pointer position to a data index. The reader aims at a
// date, never at a line, so this is a horizontal-distance snap.
const nearestIndex = (clientX: number): number | null => {
  const el = root.value
  if (!el || !props.points.length) {
    return null
  }
  const rect = el.getBoundingClientRect()
  const ratio = (clientX - rect.left) / rect.width
  const vbX = ratio * VB_W
  const n = props.points.length
  if (n === 1) {
    return 0
  }
  const i = Math.round(((vbX - PAD.left) / plotW) * (n - 1))
  return Math.min(n - 1, Math.max(0, i))
}

const onPointer = (event: PointerEvent) => {
  active.value = nearestIndex(event.clientX)
}

const onLeave = () => {
  active.value = null
}

// Keyboard users walk the series with the arrow keys and get the same readout.
const onKey = (event: KeyboardEvent) => {
  if (!props.points.length) {
    return
  }
  const last = props.points.length - 1
  if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
    event.preventDefault()
    const from = active.value ?? (event.key === 'ArrowRight' ? -1 : last + 1)
    active.value = Math.min(
      last,
      Math.max(0, from + (event.key === 'ArrowRight' ? 1 : -1))
    )
  } else if (event.key === 'Escape') {
    active.value = null
  }
}

const activePoint = computed(() =>
  active.value === null ? null : (props.points[active.value] ?? null)
)

// The tooltip flips to the left of the crosshair near the right edge so the
// plot never clips it. Positioned as a percentage because the SVG stretches
// horizontally — viewBox units are not pixels on the x axis.
const tooltipStyle = computed(() => {
  if (active.value === null) {
    return {}
  }
  const ratio = x(active.value) / VB_W
  return {
    left: `${ratio * 100}%`,
    transform: `translateX(${ratio > 0.6 ? 'calc(-100% - 12px)' : '12px'})`
  }
})

// The y-axis gutter, as a percentage of the stretched coordinate space.
const gutterWidth = `${((PAD.left - 8) / VB_W) * 100}%`

const label = (point: SeriesPoint): string =>
  props.granularity === 'hour'
    ? `${formatDate(point.at)} ${formatHour(point.at)}`
    : formatDate(point.at)
</script>

<template>
  <div class="flex flex-col gap-3">
    <!-- Legend: always present for two or more series, so identity never rests
         on color alone. Line keys mirror the mark. -->
    <div v-if="series.length > 1" class="flex flex-wrap items-center gap-4">
      <span
        v-for="s in series"
        :key="s.key"
        class="flex items-center gap-2 text-xs text-default-500"
      >
        <span
          class="h-0.5 w-4 rounded-full"
          :style="{ backgroundColor: s.color }"
        />
        {{ s.label }}
      </span>
    </div>

    <div
      ref="root"
      class="relative select-none rounded-lg focus-visible:ring-2 focus-visible:ring-primary/50"
      tabindex="0"
      role="img"
      :aria-label="`访问趋势图，共 ${points.length} 个数据点`"
      @pointermove="onPointer"
      @pointerleave="onLeave"
      @blur="onLeave"
      @keydown="onKey"
    >
      <svg
        :viewBox="`0 0 ${VB_W} ${vbHeight}`"
        :style="{ height: `${vbHeight}px` }"
        class="w-full overflow-visible"
        preserveAspectRatio="none"
        aria-hidden="true"
      >
        <!-- Gridlines: hairline, solid, one step off the surface. -->
        <g>
          <line
            v-for="t in ticks"
            :key="`grid-${t}`"
            :x1="PAD.left"
            :x2="VB_W - PAD.right"
            :y1="y(t)"
            :y2="y(t)"
            :stroke="CHART_GRID_COLOR"
            stroke-width="1"
            vector-effect="non-scaling-stroke"
          />
        </g>

        <!-- Area wash for the leading series: the hue at ~10%, never a block. -->
        <path
          v-for="s in series.filter((item) => item.fill)"
          :key="`area-${s.key}`"
          :d="areaPath(s)"
          :fill="s.color"
          fill-opacity="0.1"
        />

        <path
          v-for="s in series"
          :key="`line-${s.key}`"
          :d="linePath(s)"
          fill="none"
          :stroke="s.color"
          stroke-width="2"
          stroke-linejoin="round"
          stroke-linecap="round"
          vector-effect="non-scaling-stroke"
        />

        <!-- Crosshair + the hovered点 markers, each with a 2px surface ring so
             they stay legible where the two series cross. -->
        <g v-if="active !== null">
          <line
            :x1="x(active)"
            :x2="x(active)"
            :y1="PAD.top"
            :y2="PAD.top + plotH"
            :stroke="CHART_GRID_COLOR"
            stroke-width="1"
            vector-effect="non-scaling-stroke"
          />
          <circle
            v-for="s in series"
            :key="`dot-${s.key}`"
            :cx="x(active)"
            :cy="y(s.value(points[active]!))"
            r="4"
            :fill="s.color"
            :stroke="CHART_SURFACE"
            stroke-width="2"
            vector-effect="non-scaling-stroke"
          />
        </g>
      </svg>

      <!-- Axis text lives in HTML, not SVG: preserveAspectRatio="none" stretches
           the coordinate space horizontally, which would distort glyphs. -->
      <div class="pointer-events-none absolute inset-0">
        <span
          v-for="t in ticks"
          :key="`ytick-${t}`"
          class="absolute left-0 -translate-y-1/2 text-right text-[11px] tabular-nums text-default-400"
          :style="{ top: `${y(t)}px`, width: gutterWidth }"
        >
          {{ formatCompact(t) }}
        </span>
        <span
          v-for="l in xLabels"
          :key="`xtick-${l.i}`"
          class="absolute -translate-x-1/2 text-[11px] tabular-nums text-default-400"
          :style="{ bottom: '2px', left: `${(x(l.i) / VB_W) * 100}%` }"
        >
          {{ l.text }}
        </span>
      </div>

      <!-- One tooltip lists every series at that x. Values lead, labels follow. -->
      <div
        v-if="activePoint"
        class="pointer-events-none absolute top-2 z-10 min-w-36 rounded-lg border border-kun bg-content1 p-2.5 shadow-lg"
        :style="tooltipStyle"
      >
        <p class="mb-1.5 text-[11px] text-default-500">
          {{ label(activePoint) }}
        </p>
        <p
          v-for="s in series"
          :key="`tip-${s.key}`"
          class="flex items-center gap-2 text-xs"
        >
          <span
            class="h-0.5 w-3 shrink-0 rounded-full"
            :style="{ backgroundColor: s.color }"
          />
          <span class="font-semibold tabular-nums">
            {{ formatNumber(s.value(activePoint)) }}
          </span>
          <span class="text-default-500">{{ s.label }}</span>
        </p>
      </div>
    </div>
  </div>
</template>
