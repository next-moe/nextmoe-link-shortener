<script setup lang="ts">
// Ranked magnitude list — the honest form for "which of these got the most
// traffic". The categories are nominal (hosts, aliases), so every bar wears the
// SAME hue: bar length already encodes the value, and spending the identity
// channel to re-encode it would say nothing new.
export interface BarListRow {
  key: string
  label: string
  /** Optional muted line under the label. */
  hint?: string
  value: number
}

const props = withDefaults(
  defineProps<{
    rows: BarListRow[]
    /** Suffix for the value, e.g. "次". */
    unit?: string
    emptyText?: string
  }>(),
  { unit: '', emptyText: '本期没有数据' }
)

const emit = defineEmits<{ select: [BarListRow] }>()

const max = computed(() => Math.max(1, ...props.rows.map((r) => r.value)))
const total = computed(() => props.rows.reduce((sum, r) => sum + r.value, 0))
</script>

<template>
  <!-- Sticker-less empty state: this list lives inside a card or a drawer
       section, where the full mascot outweighs everything around it. -->
  <KunNull
    v-if="!rows.length"
    :description="emptyText"
    :is-show-sticker="false"
  />

  <ul v-else class="flex flex-col">
    <li v-for="row in rows" :key="row.key">
      <button
        type="button"
        class="group flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left transition-colors hover:bg-default-100"
        @click="emit('select', row)"
      >
        <span class="min-w-0 flex-1">
          <span class="flex items-baseline justify-between gap-3">
            <span class="truncate text-sm text-default-700">{{ row.label }}</span>
            <span class="shrink-0 text-sm font-semibold tabular-nums">
              {{ formatNumber(row.value) }}<span
                v-if="unit"
                class="ml-0.5 text-xs font-normal text-default-400"
              >{{ unit }}</span>
            </span>
          </span>

          <span class="mt-1.5 flex items-center gap-2">
            <!-- The track is a lighter step of the same neutral so the bar
                 reads as a share of the row, not as a floating mark. -->
            <span class="h-1.5 flex-1 overflow-hidden rounded-full bg-default-100">
              <span
                class="block h-full rounded-full transition-[width] duration-300"
                :style="{
                  width: `${Math.max(2, (row.value / max) * 100)}%`,
                  backgroundColor: SERIES_VISITS
                }"
              />
            </span>
            <span class="w-10 shrink-0 text-right text-[11px] tabular-nums text-default-400">
              {{ total ? formatPercent(row.value / total) : '—' }}
            </span>
          </span>

          <span
            v-if="row.hint"
            class="mt-1 block truncate text-[11px] text-default-400"
          >
            {{ row.hint }}
          </span>
        </span>
      </button>
    </li>
  </ul>
</template>
