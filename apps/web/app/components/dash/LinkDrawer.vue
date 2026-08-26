<script setup lang="ts">
// Per-link detail. A drawer rather than a sibling column: the stats panel used
// to sit beside a full-height list, so it stretched to the list's height and
// left a screen of dead space. Here it gets its own full-height surface and the
// list keeps the page width.
//
// It inherits the page's time range — one filter row scopes everything, so the
// drawer's numbers always agree with the table row behind it.
import type { LinkDTO, LinkStatsDTO } from '~~/shared/types/shortlink'

const props = defineProps<{
  modelValue: boolean
  link: LinkDTO | null
  range: number
  granularity: 'hour' | 'day'
}>()

const emit = defineEmits<{
  'update:modelValue': [boolean]
  edit: [LinkDTO]
  remove: [LinkDTO]
}>()

const { linkStats } = useApi()

const open = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const stats = ref<LinkStatsDTO | null>(null)
const pending = ref(false)

const load = async () => {
  if (!props.link || !props.modelValue) {
    return
  }
  pending.value = true
  try {
    stats.value = await linkStats(props.link.alias, props.range)
  } catch {
    stats.value = null
    useKunMessage('加载访问统计失败', 'error')
  } finally {
    pending.value = false
  }
}

// Reload whenever the drawer opens, switches link, or the page's range changes
// underneath it. Stats for the previous link are dropped first so the panel
// never shows one link's numbers under another's title.
watch(
  () => [props.modelValue, props.link?.alias, props.range] as const,
  ([isOpen, alias]) => {
    if (!isOpen || !alias) {
      return
    }
    if (stats.value && stats.value.link.alias !== alias) {
      stats.value = null
    }
    load()
  }
)

const points = computed(() =>
  stats.value
    ? buildSeries(stats.value.buckets, stats.value.range_days, props.granularity)
    : []
)

const referrerRows = computed(() =>
  (stats.value?.referrers ?? []).map((r) => ({
    key: r.host || '__direct',
    label: r.host || '直接访问',
    value: r.visits
  }))
)

const copyShortURL = async () => {
  if (!props.link) {
    return
  }
  await navigator.clipboard.writeText(props.link.short_url)
  useKunMessage('短链已复制', 'success')
}

const rangeLabel = computed(() => rangeMeta(props.range).label)
</script>

<template>
  <KunDrawer v-model="open" placement="right" size="lg">
    <template #header>
      <div v-if="link" class="flex min-w-0 flex-col gap-1 pr-8">
        <div class="flex items-center gap-2">
          <span class="truncate font-mono text-base font-semibold text-primary">
            /s/{{ link.alias }}
          </span>
          <KunChip
            :color="statusMeta(link.status).color"
            size="sm"
            variant="flat"
          >
            <template #start>
              <KunIcon :name="statusMeta(link.status).icon" />
            </template>
            {{ statusMeta(link.status).label }}
          </KunChip>
        </div>
        <p v-if="link.description" class="truncate text-sm text-default-500">
          {{ link.description }}
        </p>
      </div>
    </template>

    <div v-if="link" class="flex flex-col gap-6">
      <!-- Destination + the two actions that act on the link itself. -->
      <div class="flex flex-col gap-3 rounded-xl bg-default-100 p-4">
        <div class="flex items-start gap-2">
          <KunIcon
            name="lucide:corner-down-right"
            class="mt-0.5 shrink-0 text-default-400"
          />
          <a
            :href="link.destination_url"
            target="_blank"
            rel="noopener noreferrer"
            class="min-w-0 break-all text-sm text-default-700 underline-offset-2 hover:underline"
          >
            {{ link.destination_url }}
          </a>
        </div>

        <div class="flex flex-wrap gap-2">
          <KunButton variant="flat" size="sm" icon @click="copyShortURL">
            <template #icon>
              <KunIcon name="lucide:copy" />
            </template>
            复制短链
          </KunButton>
          <KunButton
            variant="flat"
            size="sm"
            :href="link.short_url"
            target="_blank"
            icon
          >
            <template #icon>
              <KunIcon name="lucide:external-link" />
            </template>
            打开
          </KunButton>
          <div class="grow" />
          <KunButton variant="light" size="sm" icon @click="emit('edit', link)">
            <template #icon>
              <KunIcon name="lucide:pencil" />
            </template>
            编辑
          </KunButton>
          <KunButton
            variant="light"
            size="sm"
            color="danger"
            icon
            @click="emit('remove', link)"
          >
            <template #icon>
              <KunIcon name="lucide:trash-2" />
            </template>
            删除
          </KunButton>
        </div>
      </div>

      <div v-if="pending && !stats" class="flex flex-col gap-3">
        <KunSkeleton v-for="i in 4" :key="i" height="3rem" />
      </div>

      <template v-else-if="stats">
        <div
          class="transition-opacity duration-200"
          :class="pending ? 'opacity-50' : ''"
        >
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div class="flex flex-col gap-1 rounded-xl border border-kun p-3">
              <span class="text-[11px] text-default-500">{{ rangeLabel }}访问</span>
              <span class="text-xl font-semibold">
                {{ formatCompact(stats.range_visits) }}
              </span>
            </div>
            <div class="flex flex-col gap-1 rounded-xl border border-kun p-3">
              <span class="text-[11px] text-default-500">{{ rangeLabel }}独立</span>
              <span class="text-xl font-semibold">
                {{ formatCompact(stats.range_unique) }}
              </span>
            </div>
            <div class="flex flex-col gap-1 rounded-xl border border-kun p-3">
              <span class="text-[11px] text-default-500">总访问</span>
              <span class="text-xl font-semibold">
                {{ formatCompact(stats.link.visit_count) }}
              </span>
            </div>
            <div class="flex flex-col gap-1 rounded-xl border border-kun p-3">
              <span class="text-[11px] text-default-500">独立访客</span>
              <span class="text-xl font-semibold">
                {{ formatCompact(stats.unique_visitors) }}
              </span>
            </div>
          </div>
        </div>

        <section class="flex flex-col gap-3">
          <h3 class="text-sm font-medium">
            {{ rangeLabel }}访问分布
          </h3>
          <ChartColumns
            v-if="points.length"
            :points="points"
            :granularity="granularity"
          />
          <KunNull v-else description="所选时间范围内没有访问" />
        </section>

        <section class="flex flex-col gap-3">
          <h3 class="text-sm font-medium">引荐来源</h3>
          <ChartBarList
            :rows="referrerRows"
            unit="次"
            empty-text="本期没有引荐记录"
          />
        </section>

        <section class="flex flex-col gap-3">
          <h3 class="text-sm font-medium">最近访问</h3>
          <KunNull
            v-if="!stats.recent.length"
            description="还没有访问记录"
            :is-show-sticker="false"
          />
          <ul v-else class="flex flex-col divide-y divide-kun">
            <li
              v-for="v in stats.recent"
              :key="v.id"
              class="flex items-center gap-3 py-2 text-xs"
            >
              <KunTooltip :text="v.is_unique ? '该时段首次访问的 IP' : '重复访问'">
                <KunIcon
                  :name="v.is_unique ? 'lucide:user-plus' : 'lucide:user'"
                  :class="v.is_unique ? 'text-success' : 'text-default-300'"
                />
              </KunTooltip>
              <span class="w-32 shrink-0 font-mono tabular-nums text-default-600">
                {{ v.ip || '—' }}
              </span>
              <span class="min-w-0 grow truncate text-default-400">
                {{ v.referer ? hostOf(v.referer) : '直接访问' }}
              </span>
              <span class="shrink-0 tabular-nums text-default-400">
                {{ formatRelative(v.created_at) }}
              </span>
            </li>
          </ul>
        </section>

        <!-- Provenance and lifetime rules: rarely read, but the answer to
             "why did this link stop working" lives here. -->
        <dl class="grid grid-cols-2 gap-x-4 gap-y-3 border-t border-kun pt-4 text-xs">
          <div class="flex flex-col gap-0.5">
            <dt class="text-default-400">创建来源</dt>
            <dd>{{ sourceLabel(link.created_via) }}</dd>
          </div>
          <div class="flex flex-col gap-0.5">
            <dt class="text-default-400">创建时间</dt>
            <dd class="tabular-nums">{{ formatDateTime(link.created_at) }}</dd>
          </div>
          <div class="flex flex-col gap-0.5">
            <dt class="text-default-400">过期时间</dt>
            <dd class="tabular-nums">
              {{ link.expires_at ? formatDateTime(link.expires_at) : '永不过期' }}
            </dd>
          </div>
          <div class="flex flex-col gap-0.5">
            <dt class="text-default-400">访问上限</dt>
            <dd class="tabular-nums">
              {{ link.max_visits > 0 ? formatNumber(link.max_visits) : '不限' }}
            </dd>
          </div>
        </dl>
      </template>
    </div>
  </KunDrawer>
</template>
