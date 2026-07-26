<script setup lang="ts">
// Dashboard: create form + link table + stats panel. All data flows through
// useApi (same-origin /api proxy); the admin gate is the route middleware
// plus the API's own role check on every call.
import type { LinkDTO } from '~~/shared/types/shortlink'

const { listLinks, deleteLink } = useApi()

const links = ref<LinkDTO[]>([])
const loading = ref(true)
const showCreate = ref(false)
const selectedAlias = ref('')
const editing = ref<LinkDTO | null>(null)
const showEdit = ref(false)
const deleting = ref<LinkDTO | null>(null)
const showDelete = ref(false)
const deleteBusy = ref(false)

const refresh = async () => {
  loading.value = true
  try {
    const res = await listLinks()
    links.value = res.links
    if (links.value.length && !links.value.some((l) => l.alias === selectedAlias.value)) {
      selectedAlias.value = links.value[0]!.alias
    }
  } catch {
    useKunMessage('加载短链列表失败', 'error')
  } finally {
    loading.value = false
  }
}

onMounted(refresh)

const handleCreated = async (link: LinkDTO) => {
  showCreate.value = false
  selectedAlias.value = link.alias
  await refresh()
}

const copyShortURL = async (link: LinkDTO) => {
  await navigator.clipboard.writeText(link.short_url)
  useKunMessage('短链已复制', 'success')
}

const openEdit = (link: LinkDTO) => {
  editing.value = link
  showEdit.value = true
}

const handleUpdated = async () => {
  showEdit.value = false
  await refresh()
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
    await refresh()
  } catch {
    useKunMessage('删除失败，请重试', 'error')
  } finally {
    deleteBusy.value = false
  }
}

const formatDate = (iso: string | null) =>
  iso ? new Date(iso).toLocaleString('zh-CN', { hour12: false }) : '—'
</script>

<template>
  <main class="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-8">
    <div class="flex items-center gap-3">
      <h1 class="text-xl font-semibold">短链管理</h1>
      <div class="grow" />
      <KunButton size="sm" @click="showCreate = !showCreate">
        {{ showCreate ? '收起' : '新建短链' }}
      </KunButton>
    </div>

    <DashCreateForm v-if="showCreate" @created="handleCreated" />

    <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
      <KunCard class="lg:col-span-2" padding="none">
        <div v-if="loading" class="flex flex-col gap-3 p-6">
          <KunSkeleton v-for="i in 4" :key="i" class="h-10 w-full" />
        </div>

        <KunNull v-else-if="!links.length" description="还没有短链，点击右上角新建" />

        <ul v-else class="divide-y divide-default-200">
          <li
            v-for="link in links"
            :key="link.id"
            class="flex cursor-pointer flex-col gap-1 p-4 transition-colors hover:bg-default-100"
            :class="link.alias === selectedAlias ? 'bg-default-100' : ''"
            @click="selectedAlias = link.alias"
          >
            <div class="flex items-center gap-2">
              <span class="font-mono font-medium text-primary">/s/{{ link.alias }}</span>
              <KunChip :color="LINK_STATUS_COLOR[link.status]" size="sm">
                {{ LINK_STATUS_LABEL[link.status] }}
              </KunChip>
              <KunChip v-if="link.forward_params" color="info" size="sm">透传参数</KunChip>
              <div class="grow" />
              <span class="text-xs text-default-400">{{ link.visit_count }} 次访问</span>
            </div>

            <p class="truncate text-sm text-default-500">{{ link.destination_url }}</p>

            <div class="flex items-center gap-2">
              <span v-if="link.description" class="truncate text-xs text-default-400">
                {{ link.description }}
              </span>
              <span class="text-xs text-default-300">
                {{ link.created_via }} · {{ formatDate(link.created_at) }}
              </span>
              <div class="grow" />
              <KunButton
                variant="light"
                size="xs"
                is-icon-only
                aria-label="复制短链"
                @click.stop="copyShortURL(link)"
              >
                <KunIcon name="lucide:copy" />
              </KunButton>
              <KunButton
                variant="light"
                size="xs"
                is-icon-only
                aria-label="编辑"
                @click.stop="openEdit(link)"
              >
                <KunIcon name="lucide:pencil" />
              </KunButton>
              <KunButton
                variant="light"
                size="xs"
                color="danger"
                is-icon-only
                aria-label="删除"
                @click.stop="openDelete(link)"
              >
                <KunIcon name="lucide:trash-2" />
              </KunButton>
            </div>
          </li>
        </ul>
      </KunCard>

      <DashStatsPanel :alias="selectedAlias" />
    </div>

    <DashEditModal v-model="showEdit" :link="editing" @updated="handleUpdated" />

    <KunModal v-model="showDelete" size="sm">
      <div class="flex flex-col gap-4">
        <h2 class="text-lg font-bold">删除短链</h2>
        <p class="text-sm text-default-500">
          确定删除
          <span class="font-mono text-foreground">/s/{{ deleting?.alias }}</span>
          吗？访问记录会一并删除，此操作不可撤销。
        </p>
        <div class="flex justify-end gap-2">
          <KunButton variant="flat" @click="showDelete = false">取消</KunButton>
          <KunButton color="danger" :loading="deleteBusy" @click="confirmDelete">
            删除
          </KunButton>
        </div>
      </div>
    </KunModal>
  </main>
</template>
