<script setup lang="ts">
// A headline number is a stat tile, not a one-bar chart.
//
// Contract: label · value (compact, proportional figures) · optional delta
// against a named period · optional trend. The delta's color carries direction,
// and its arrow icon + text carry it again, so meaning never rests on color.
//
// Every tile ends in a footer band of the same height — a sparkline, a status
// split, or a meter — so a row of tiles reads as one strip instead of two tall
// cards beside two half-empty ones.
import type { Delta } from '~~/shared/utils/format'

export interface TileSegment {
  key: string
  label: string
  value: number
  /** A CSS color. Status splits pass reserved status tokens, not series hues. */
  color: string
}

withDefaults(
  defineProps<{
    label: string
    value: number
    icon: string
    delta?: Delta | null
    spark?: number[] | null
    /** Part-to-whole footer (e.g. links by status). */
    segments?: TileSegment[] | null
    /** Muted supporting line under the value. */
    hint?: string
  }>(),
  { delta: null, spark: null, segments: null, hint: '' }
)

const DELTA_TONE: Record<Delta['direction'], string> = {
  up: 'text-success-700',
  down: 'text-danger-600',
  flat: 'text-default-500'
}

const DELTA_ICON: Record<Delta['direction'], string> = {
  up: 'lucide:trending-up',
  down: 'lucide:trending-down',
  flat: 'lucide:minus'
}
</script>

<template>
  <KunCard padding="none" class="overflow-hidden">
    <div class="flex h-full flex-col justify-between gap-4">
      <div class="flex flex-col gap-3 p-4 pb-0 sm:p-5 sm:pb-0">
        <div class="flex items-center gap-2">
          <KunIcon :name="icon" class="text-base text-default-400" />
          <span class="text-xs text-default-500">{{ label }}</span>
        </div>

        <div class="flex items-end justify-between gap-3">
          <!-- Proportional figures: tabular digits make a display-size number
               look loose. Tabular is for columns, not headlines. -->
          <p class="text-3xl font-semibold leading-none">
            {{ formatCompact(value) }}
          </p>

          <span
            v-if="delta"
            class="flex shrink-0 items-center gap-1 text-xs font-medium"
            :class="DELTA_TONE[delta.direction]"
          >
            <KunIcon :name="DELTA_ICON[delta.direction]" />
            {{ delta.label }}
          </span>
        </div>

        <p v-if="hint" class="text-[11px] text-default-400">{{ hint }}</p>
      </div>

      <!-- Footer band. The sparkline runs flush to the card edge — it is
           texture for the number above it, not a chart in its own right. -->
      <ChartSparkline v-if="spark && spark.length > 1" :values="spark" />

      <div
        v-else-if="segments?.length"
        class="flex flex-col gap-2 px-4 pb-4 sm:px-5 sm:pb-5"
      >
        <div class="flex h-1.5 w-full gap-0.5 overflow-hidden rounded-full">
          <span
            v-for="s in segments.filter((seg) => seg.value > 0)"
            :key="s.key"
            class="h-full first:rounded-l-full last:rounded-r-full"
            :style="{
              width: `${(s.value / Math.max(1, segments.reduce((sum, seg) => sum + seg.value, 0))) * 100}%`,
              backgroundColor: s.color
            }"
          />
        </div>
        <div class="flex flex-wrap gap-x-3 gap-y-1">
          <span
            v-for="s in segments"
            :key="`legend-${s.key}`"
            class="flex items-center gap-1.5 text-[11px] text-default-500"
          >
            <span
              class="size-2 rounded-sm"
              :style="{ backgroundColor: s.color }"
            />
            {{ s.label }}
            <span class="tabular-nums text-default-400">{{ s.value }}</span>
          </span>
        </div>
      </div>

      <div v-else class="px-4 pb-4 sm:px-5 sm:pb-5">
        <slot name="footer" />
      </div>
    </div>
  </KunCard>
</template>
