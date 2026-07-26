<script setup lang="ts">
// Stats panel for the selected link: summary numbers, a daily bar breakdown
// aggregated from the hourly buckets, and the recent visits. Pure CSS bars —
// no chart library for an internal tool.
import type { LinkStatsDTO } from '~~/shared/types/shortlink'

const props = defineProps<{ alias: string }>()

const { linkStats } = useApi()

const stats = ref<LinkStatsDTO | null>(null)
const range = ref<number>(7)
const loading = ref(false)

const load = async () => {
  if (!props.alias) {
    stats.value = null
    return
  }
  loading.value = true
  try {
    stats.value = await linkStats(props.alias, range.value)
  } catch {
    stats.value = null
    useKunMessage('加载统计失败', 'error')
  } finally {
    loading.value = false
  }
}

watch([() => props.alias, range], load, { immediate: true })

// days aggregates hourly buckets into per-day rows for the bar list.
const days = computed(() => {
  if (!stats.value) {
    return []
  }
  const byDay = new Map<string, { visits: number; unique: number }>()
  for (const b of stats.value.buckets) {
    const day = new Date(b.bucket_start).toLocaleDateString('zh-CN')
    const row = byDay.get(day) ?? { visits: 0, unique: 0 }
    row.visits += b.visits
    row.unique += b.unique_ips
    byDay.set(day, row)
  }
  const max = Math.max(1, ...[...byDay.values()].map((r) => r.visits))
  return [...byDay.entries()].map(([day, row]) => ({
    day,
    ...row,
    percent: Math.round((row.visits / max) * 100)
  }))
})

const rangeOptions = STATS_RANGES.map((d) => ({ value: d, label: `${d} 天` }))

const formatTime = (iso: string) =>
  new Date(iso).toLocaleString('zh-CN', { hour12: false })
</script>

<template>
  <KunCard class="flex flex-col gap-4">
    <div class="flex items-center gap-2">
      <h2 class="font-semibold">访问统计</h2>
      <div class="grow" />
      <KunSelect v-model="range" :options="rangeOptions" size="sm" class-name="w-24" />
    </div>

    <KunNull v-if="!alias" description="选择左侧的短链查看统计" />
    <div v-else-if="loading && !stats" class="flex flex-col gap-3">
      <KunSkeleton v-for="i in 3" :key="i" class="h-8 w-full" />
    </div>

    <template v-else-if="stats">
      <div class="grid grid-cols-2 gap-3 text-center">
        <div class="rounded-lg bg-default-100 p-3">
          <p class="text-2xl font-semibold">{{ stats.link.visit_count }}</p>
          <p class="text-xs text-default-500">总访问</p>
        </div>
        <div class="rounded-lg bg-default-100 p-3">
          <p class="text-2xl font-semibold">{{ stats.unique_visitors }}</p>
          <p class="text-xs text-default-500">独立访客（全期）</p>
        </div>
      </div>

      <div v-if="days.length" class="flex flex-col gap-2">
        <div v-for="d in days" :key="d.day" class="flex items-center gap-2 text-xs">
          <span class="w-20 shrink-0 text-default-500">{{ d.day }}</span>
          <div class="h-4 grow rounded-sm bg-default-100">
            <div
              class="h-full rounded-sm bg-primary-400"
              :style="{ width: `${d.percent}%` }"
            />
          </div>
          <span class="w-14 shrink-0 text-right text-default-500">
            {{ d.visits }} / {{ d.unique }}
          </span>
        </div>
        <p class="text-right text-xs text-default-300">访问 / 独立 IP</p>
      </div>
      <KunNull v-else description="所选时间范围内没有访问" />

      <div v-if="stats.recent.length" class="flex flex-col gap-2">
        <h3 class="text-sm font-medium text-default-500">最近访问</h3>
        <ul class="flex flex-col gap-1">
          <li
            v-for="v in stats.recent"
            :key="v.id"
            class="flex items-center gap-2 text-xs text-default-500"
          >
            <KunIcon
              :name="v.is_unique ? 'lucide:user-plus' : 'lucide:user'"
              :class="v.is_unique ? 'text-success' : 'text-default-300'"
            />
            <span class="font-mono">{{ v.ip || '—' }}</span>
            <span class="grow truncate text-default-300">{{ v.referer || '直接访问' }}</span>
            <span class="shrink-0">{{ formatTime(v.created_at) }}</span>
          </li>
        </ul>
      </div>
    </template>
  </KunCard>
</template>
