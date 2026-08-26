<script setup lang="ts">
// Part-to-whole in one horizontal stacked bar. Segments are separated by a 2px
// gap cut in the surface color — never by a stroke, which would add ink that
// isn't data. Every segment is also direct-labeled below, so identity never
// depends on matching a color to a legend swatch.
export interface ShareSegment {
  key: string
  label: string
  value: number
  /** Optional muted second line, e.g. how many links the source owns. */
  hint?: string
}

const props = defineProps<{
  segments: ShareSegment[]
  emptyText?: string
}>()

const total = computed(() =>
  props.segments.reduce((sum, s) => sum + s.value, 0)
)

// Color follows the ENTITY, never its rank: switching the dashboard's time
// range must not repaint the segments, so a reader who learned "moyu is green"
// keeps that after moyu overtakes kungal.
//
// Segments past the palette's slot count fold into one neutral "other" bucket
// rather than growing the hue count past what a reader can tell apart.
const painted = computed(() => {
  const sorted = [...props.segments].sort((a, b) => b.value - a.value)
  const head = sorted.slice(0, CHART_SLOTS.length)
  const tail = sorted.slice(CHART_SLOTS.length)

  // Slots are handed out over the painted keys in NAME order. Size decides who
  // gets painted; the name decides which hue they get, so re-ranking within the
  // set never repaints it.
  const order = head.map((s) => s.key).sort()
  const rows = head.map((s) => ({ ...s, color: slotColor(order.indexOf(s.key)) }))
  if (tail.length) {
    rows.push({
      key: '__other',
      label: CHART_OTHER_LABEL,
      value: tail.reduce((sum, s) => sum + s.value, 0),
      color: CHART_OTHER_COLOR
    })
  }
  return rows.filter((s) => s.value > 0)
})
</script>

<template>
  <div class="flex flex-col gap-3">
    <KunNull
      v-if="!total"
      :description="emptyText ?? '本期没有访问'"
      :is-show-sticker="false"
    />

    <template v-else>
      <div class="flex h-2.5 w-full gap-0.5 overflow-hidden rounded-full">
        <span
          v-for="s in painted"
          :key="s.key"
          class="h-full first:rounded-l-full last:rounded-r-full"
          :style="{
            width: `${(s.value / total) * 100}%`,
            backgroundColor: s.color
          }"
        />
      </div>

      <ul class="flex flex-col gap-2">
        <li
          v-for="s in painted"
          :key="s.key"
          class="flex items-center gap-2 text-sm"
        >
          <span
            class="size-2.5 shrink-0 rounded-sm"
            :style="{ backgroundColor: s.color }"
          />
          <span class="flex min-w-0 flex-1 flex-col">
            <span class="truncate text-default-700">{{ s.label }}</span>
            <span v-if="s.hint" class="truncate text-[11px] text-default-400">
              {{ s.hint }}
            </span>
          </span>
          <span class="shrink-0 font-semibold tabular-nums">
            {{ formatNumber(s.value) }}
          </span>
          <span class="w-12 shrink-0 text-right text-xs tabular-nums text-default-400">
            {{ formatPercent(s.value / total) }}
          </span>
        </li>
      </ul>
    </template>
  </div>
</template>
