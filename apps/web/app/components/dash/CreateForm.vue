<script setup lang="ts">
// Create form: destination is the only required field. Alias left empty =
// random; expiry uses the native datetime-local input (converted to RFC3339
// on submit).
import type { LinkDTO } from '~~/shared/types/shortlink'

const emit = defineEmits<{ created: [LinkDTO] }>()

const { createLink } = useApi()

const destinationUrl = ref('')
const alias = ref('')
const description = ref('')
const expiresAt = ref('')
// KunNumberInput's model is number | null (cleared input = null).
const maxVisits = ref<number | null>(0)
const forwardParams = ref(false)
const submitting = ref(false)

const submit = async () => {
  if (submitting.value) {
    return
  }
  if (!destinationUrl.value.trim()) {
    useKunMessage('请填写目标 URL', 'warn')
    return
  }
  submitting.value = true
  try {
    const link = await createLink({
      destination_url: destinationUrl.value.trim(),
      alias: alias.value.trim() || undefined,
      description: description.value.trim() || undefined,
      expires_at: expiresAt.value ? new Date(expiresAt.value).toISOString() : undefined,
      max_visits: maxVisits.value && maxVisits.value > 0 ? maxVisits.value : undefined,
      forward_params: forwardParams.value || undefined
    })
    useKunMessage(`短链 /s/${link.alias} 已创建`, 'success')
    destinationUrl.value = ''
    alias.value = ''
    description.value = ''
    expiresAt.value = ''
    maxVisits.value = 0
    forwardParams.value = false
    emit('created', link)
  } catch (err) {
    const status = (err as { statusCode?: number }).statusCode
    if (status === 409) {
      useKunMessage('别名已被占用，换一个试试', 'warn')
    } else if (status === 422) {
      useKunMessage('参数不合法：检查 URL 与别名格式（4-32 位字母数字_-）', 'warn')
    } else {
      useKunMessage('创建失败，请重试', 'error')
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <KunCard class="flex flex-col gap-4">
    <KunInput
      v-model="destinationUrl"
      label="目标 URL"
      placeholder="https://www.kungal.com/topic/..."
      required
    />

    <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
      <KunInput
        v-model="alias"
        label="自定义别名（可选）"
        placeholder="留空 = 随机 6 位"
        description="4-32 位，字母 / 数字 / _ / -"
      />
      <KunInput
        v-model="description"
        label="备注（可选）"
        placeholder="这条短链是干什么的"
      />
    </div>

    <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
      <label class="flex flex-col gap-1 text-sm">
        <span class="text-default-500">过期时间（可选）</span>
        <input
          v-model="expiresAt"
          type="datetime-local"
          class="rounded-lg border border-default-200 bg-transparent px-3 py-2 text-sm"
        >
      </label>
      <KunNumberInput v-model="maxVisits" label="最大访问次数（0 = 不限）" :min="0" />
      <KunSwitch v-model="forwardParams" label="透传查询参数" />
    </div>

    <div class="flex justify-end">
      <KunButton :loading="submitting" @click="submit">创建短链</KunButton>
    </div>
  </KunCard>
</template>
