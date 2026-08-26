<script setup lang="ts">
// S2S API key management. Minting shows the plaintext exactly once in a modal
// (the API stores only the SHA-256 hash); afterwards only the prefix is
// visible, so the copy step in that modal is the operator's only chance.
import type { KeyDTO } from '~~/shared/types/shortlink'

const { listKeys, createKey, setKeyDisabled, deleteKey } = useApi()

const keys = ref<KeyDTO[]>([])
const loading = ref(true)
const name = ref('')
const creating = ref(false)
const plaintext = ref('')
const showPlaintext = ref(false)
const showCreate = ref(false)
const deleting = ref<KeyDTO | null>(null)
const showDelete = ref(false)
const deleteBusy = ref(false)
const busyId = ref<number | null>(null)

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

const nameError = computed(() => {
  const raw = name.value.trim()
  if (!raw) {
    return ''
  }
  return keys.value.some((k) => k.name === raw) ? '已经有同名的 key 了' : ''
})

const create = async () => {
  if (creating.value || !name.value.trim() || nameError.value) {
    return
  }
  creating.value = true
  try {
    const res = await createKey(name.value.trim())
    plaintext.value = res.plaintext
    showCreate.value = false
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
  busyId.value = key.id
  try {
    await setKeyDisabled(key.id, !key.disabled)
    useKunMessage(key.disabled ? 'key 已启用' : 'key 已停用', 'success')
    await refresh()
  } catch {
    useKunMessage('操作失败，请重试', 'error')
  } finally {
    busyId.value = null
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

const activeCount = computed(() => keys.value.filter((k) => !k.disabled).length)

// The example call, kept next to the key list so an integrator does not have to
// leave the page to find the shape of the request.
const sampleRequest = `curl -X POST https://s.kungal.com/s2s/links \\
  -H "Authorization: Bearer slk_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{"destination_url":"https://www.kungal.com/topic/1024"}'`

const copySample = async () => {
  await navigator.clipboard.writeText(sampleRequest)
  useKunMessage('示例请求已复制', 'success')
}
</script>

<template>
  <main class="mx-auto flex w-full max-w-5xl flex-col gap-6 px-4 py-6 sm:px-6 sm:py-8">
    <header class="flex flex-wrap items-end justify-between gap-4">
      <div class="flex flex-col gap-1">
        <h1 class="text-2xl font-semibold">S2S API Keys</h1>
        <p class="text-sm text-default-500">
          生态站点（kungal / moyu / letmoe …）用这些 key 调用短链接口
        </p>
        <p v-if="!loading" class="text-xs text-default-400">
          共 {{ keys.length }} 个 key · {{ activeCount }} 个启用中
        </p>
      </div>
      <KunButton icon @click="showCreate = true">
        <template #icon>
          <KunIcon name="lucide:plus" />
        </template>
        新建 key
      </KunButton>
    </header>

    <KunCard padding="none" class="overflow-hidden">
      <div class="flex flex-col">
        <div v-if="loading" class="flex flex-col gap-3 p-6">
          <KunSkeleton v-for="i in 3" :key="i" height="2.75rem" />
        </div>

        <div v-else-if="!keys.length" class="p-6">
          <KunNull description="还没有 API key，点击右上角创建第一个" />
        </div>

        <ul v-else class="divide-y divide-kun">
          <li
            v-for="key in keys"
            :key="key.id"
            class="flex flex-wrap items-center gap-3 p-4 sm:px-5"
          >
            <span
              class="flex size-9 shrink-0 items-center justify-center rounded-lg"
              :class="key.disabled ? 'bg-default-100' : 'bg-primary-100'"
              aria-hidden="true"
            >
              <KunIcon
                name="lucide:key-round"
                :class="key.disabled ? 'text-default-400' : 'text-primary'"
              />
            </span>

            <div class="flex min-w-0 grow flex-col gap-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-medium">{{ key.name }}</span>
                <KunChip
                  :color="key.disabled ? 'warning' : 'success'"
                  size="sm"
                  variant="flat"
                >
                  <template #start>
                    <KunIcon
                      :name="key.disabled ? 'lucide:circle-pause' : 'lucide:circle-check'"
                    />
                  </template>
                  {{ key.disabled ? '已停用' : '启用中' }}
                </KunChip>
              </div>
              <p class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-default-400">
                <span class="font-mono">{{ key.key_prefix }}…</span>
                <span>最近使用：{{ formatRelative(key.last_used_at) }}</span>
                <span>创建于 {{ formatDateTime(key.created_at) }}</span>
              </p>
            </div>

            <div class="flex shrink-0 items-center gap-1">
              <KunButton
                variant="flat"
                size="sm"
                :loading="busyId === key.id"
                @click="toggle(key)"
              >
                {{ key.disabled ? '启用' : '停用' }}
              </KunButton>
              <KunTooltip text="删除">
                <KunButton
                  variant="light"
                  size="sm"
                  color="danger"
                  is-icon-only
                  aria-label="删除"
                  @click="openDelete(key)"
                >
                  <KunIcon name="lucide:trash-2" />
                </KunButton>
              </KunTooltip>
            </div>
          </li>
        </ul>
      </div>
    </KunCard>

    <KunCard>
      <div class="flex flex-col gap-3">
        <div class="flex items-center justify-between gap-3">
          <div class="flex flex-col gap-0.5">
            <h2 class="font-semibold">怎么用</h2>
            <p class="text-xs text-default-400">
              key 通过 Bearer 头传递，响应里带上生成的短链地址
            </p>
          </div>
          <KunButton variant="flat" size="sm" icon @click="copySample">
            <template #icon>
              <KunIcon name="lucide:copy" />
            </template>
            复制
          </KunButton>
        </div>
        <pre
          class="overflow-x-auto rounded-kun-md bg-default-100 p-4 font-mono text-xs leading-relaxed"
        ><code>{{ sampleRequest }}</code></pre>
      </div>
    </KunCard>

    <KunModal v-model="showCreate" size="md">
      <div class="flex flex-col gap-5">
        <div class="flex flex-col gap-1">
          <h2 class="text-lg font-bold">新建 API key</h2>
          <p class="text-sm text-default-500">
            名称用来在列表里认出是哪个站点在用，创建后不可修改。
          </p>
        </div>
        <KunInput
          v-model="name"
          label="名称"
          placeholder="站点名，如 kungal"
          :error="nameError"
          required
          autofocus
          @keyup.enter="create"
        />
        <div class="flex justify-end gap-2">
          <KunButton variant="flat" @click="showCreate = false">取消</KunButton>
          <KunButton
            :loading="creating"
            :disabled="!name.trim() || !!nameError"
            @click="create"
          >
            创建
          </KunButton>
        </div>
      </div>
    </KunModal>

    <KunModal v-model="showPlaintext" size="md" :is-dismissable="false">
      <div class="flex flex-col gap-4">
        <h2 class="text-lg font-bold">保存这个 API key</h2>
        <KunInfo
          color="danger"
          variant="flat"
          icon="lucide:triangle-alert"
          description="key 只显示这一次，关闭后无法再查看 — 现在就复制到目标站点的密钥管理里。"
        />
        <code
          class="break-all rounded-kun-md bg-default-100 p-3 font-mono text-sm"
        >{{ plaintext }}</code>
        <div class="flex justify-end gap-2">
          <KunButton variant="flat" icon @click="copyPlaintext">
            <template #icon>
              <KunIcon name="lucide:copy" />
            </template>
            复制
          </KunButton>
          <KunButton @click="showPlaintext = false">我已保存</KunButton>
        </div>
      </div>
    </KunModal>

    <KunModal v-model="showDelete" size="sm">
      <div class="flex flex-col gap-4">
        <h2 class="text-lg font-bold">删除 API key</h2>
        <p class="text-sm text-default-500">
          确定删除
          <span class="font-medium text-foreground">{{ deleting?.name }}</span>
          吗？使用它的站点会立即失去 S2S 访问权限，已经生成的短链不受影响。
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
