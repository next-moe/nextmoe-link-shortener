<script setup lang="ts">
// S2S API key management. Minting shows the plaintext exactly once in a
// modal (the API stores only the SHA-256 hash); afterwards only the prefix
// is visible.
import type { KeyDTO } from '~~/shared/types/shortlink'

const { listKeys, createKey, setKeyDisabled, deleteKey } = useApi()

const keys = ref<KeyDTO[]>([])
const loading = ref(true)
const name = ref('')
const creating = ref(false)
const plaintext = ref('')
const showPlaintext = ref(false)
const deleting = ref<KeyDTO | null>(null)
const showDelete = ref(false)
const deleteBusy = ref(false)

const refresh = async () => {
  loading.value = true
  try {
    const res = await listKeys()
    keys.value = res.keys
  } catch {
    useKunMessage('加载 API key 列表失败', 'error')
  } finally {
    loading.value = false
  }
}

onMounted(refresh)

const create = async () => {
  if (creating.value) {
    return
  }
  if (!name.value.trim()) {
    useKunMessage('请填写 key 名称（如 kungal）', 'warn')
    return
  }
  creating.value = true
  try {
    const res = await createKey(name.value.trim())
    plaintext.value = res.plaintext
    showPlaintext.value = true
    name.value = ''
    await refresh()
  } catch (err) {
    const status = (err as { statusCode?: number }).statusCode
    if (status === 409) {
      useKunMessage('同名 key 已存在', 'warn')
    } else {
      useKunMessage('创建失败，请重试', 'error')
    }
  } finally {
    creating.value = false
  }
}

const copyPlaintext = async () => {
  await navigator.clipboard.writeText(plaintext.value)
  useKunMessage('API key 已复制', 'success')
}

const toggle = async (key: KeyDTO) => {
  try {
    await setKeyDisabled(key.id, !key.disabled)
    useKunMessage(key.disabled ? 'key 已启用' : 'key 已停用', 'success')
    await refresh()
  } catch {
    useKunMessage('操作失败，请重试', 'error')
  }
}

const openDelete = (key: KeyDTO) => {
  deleting.value = key
  showDelete.value = true
}

const confirmDelete = async () => {
  if (!deleting.value || deleteBusy.value) {
    return
  }
  deleteBusy.value = true
  try {
    await deleteKey(deleting.value.id)
    useKunMessage('key 已删除', 'success')
    showDelete.value = false
    await refresh()
  } catch {
    useKunMessage('删除失败，请重试', 'error')
  } finally {
    deleteBusy.value = false
  }
}

const formatDate = (iso: string | null) =>
  iso ? new Date(iso).toLocaleString('zh-CN', { hour12: false }) : '从未使用'
</script>

<template>
  <main class="mx-auto flex max-w-3xl flex-col gap-6 px-4 py-8">
    <div class="flex flex-col gap-1">
      <h1 class="text-xl font-semibold">S2S API Keys</h1>
      <p class="text-sm text-default-500">
        生态站点（kungal / moyu / letmoe …）用这些 key 调用
        <span class="font-mono">POST /s2s/links</span> 生成短链。
      </p>
    </div>

    <KunCard class="flex items-end gap-3">
      <KunInput
        v-model="name"
        label="新建 key"
        placeholder="站点名，如 kungal"
        class-name="grow"
        @keyup.enter="create"
      />
      <KunButton :loading="creating" @click="create">创建</KunButton>
    </KunCard>

    <KunCard padding="none">
      <div v-if="loading" class="flex flex-col gap-3 p-6">
        <KunSkeleton v-for="i in 3" :key="i" class="h-9 w-full" />
      </div>
      <KunNull v-else-if="!keys.length" description="还没有 API key" />
      <ul v-else class="divide-y divide-default-200">
        <li v-for="key in keys" :key="key.id" class="flex items-center gap-3 p-4">
          <div class="flex min-w-0 grow flex-col gap-0.5">
            <div class="flex items-center gap-2">
              <span class="font-medium">{{ key.name }}</span>
              <KunChip :color="key.disabled ? 'warning' : 'success'" size="sm">
                {{ key.disabled ? '已停用' : '启用中' }}
              </KunChip>
            </div>
            <p class="text-xs text-default-400">
              <span class="font-mono">{{ key.key_prefix }}…</span>
              · 最近使用：{{ formatDate(key.last_used_at) }}
            </p>
          </div>
          <KunButton variant="light" size="xs" @click="toggle(key)">
            {{ key.disabled ? '启用' : '停用' }}
          </KunButton>
          <KunButton
            variant="light"
            size="xs"
            color="danger"
            is-icon-only
            aria-label="删除"
            @click="openDelete(key)"
          >
            <KunIcon name="lucide:trash-2" />
          </KunButton>
        </li>
      </ul>
    </KunCard>

    <KunModal v-model="showPlaintext" size="md" :is-dismissable="false">
      <div class="flex flex-col gap-4">
        <h2 class="text-lg font-bold">保存这个 API key</h2>
        <p class="text-sm text-danger">
          key 只显示这一次，关闭后无法再查看 — 现在就复制到目标站点的密钥管理里。
        </p>
        <code
          class="break-all rounded-lg bg-default-100 p-3 font-mono text-sm"
        >{{ plaintext }}</code>
        <div class="flex justify-end gap-2">
          <KunButton variant="flat" @click="copyPlaintext">复制</KunButton>
          <KunButton @click="showPlaintext = false">我已保存</KunButton>
        </div>
      </div>
    </KunModal>

    <KunModal v-model="showDelete" size="sm">
      <div class="flex flex-col gap-4">
        <h2 class="text-lg font-bold">删除 API key</h2>
        <p class="text-sm text-default-500">
          确定删除 <span class="font-medium text-foreground">{{ deleting?.name }}</span>
          吗？使用它的站点会立即失去 S2S 访问权限。
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
