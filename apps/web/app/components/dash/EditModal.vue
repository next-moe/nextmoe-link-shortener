<script setup lang="ts">
// Edit modal: submits the full editable field set (the API's PUT is a
// whole-row update, not a patch). The alias itself is immutable — advertised
// URLs must never break — so it is shown as context, not as a field.
import type { LinkDTO } from '~~/shared/types/shortlink'

const props = defineProps<{ modelValue: boolean; link: LinkDTO | null }>()
const emit = defineEmits<{
  'update:modelValue': [boolean]
  updated: [LinkDTO]
}>()

const { updateLink } = useApi()

const open = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const destinationUrl = ref('')
const description = ref('')
const status = ref(LINK_STATUS_ACTIVE)
const expiresAt = ref('')
// KunNumberInput's model is number | null (a cleared input is null).
const maxVisits = ref<number | null>(0)
const forwardParams = ref(false)
const submitting = ref(false)

// toLocalInput renders an ISO timestamp into the datetime-local value shape
// (local time, minute precision).
const toLocalInput = (iso: string | null): string => {
  if (!iso) {
    return ''
  }
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

watch(open, (v) => {
  if (v && props.link) {
    destinationUrl.value = props.link.destination_url
    description.value = props.link.description
    status.value = props.link.status
    expiresAt.value = toLocalInput(props.link.expires_at)
    maxVisits.value = props.link.max_visits
    forwardParams.value = props.link.forward_params
  }
})

const destinationError = computed(() => {
  const raw = destinationUrl.value.trim()
  if (!raw) {
    return '目标地址不能为空'
  }
  try {
    const url = new URL(raw)
    return url.protocol === 'http:' || url.protocol === 'https:'
      ? ''
      : '必须是 http(s) 开头的绝对地址'
  } catch {
    return '请填写完整的目标地址'
  }
})

// Changing status away from active takes the link out of service immediately,
// so say so instead of letting the operator discover it from a 410.
const statusNotice = computed(() =>
  status.value === LINK_STATUS_ACTIVE
    ? ''
    : `保存后访问 /s/${props.link?.alias} 将返回 410（${statusMeta(status.value).description}）。`
)

const submit = async () => {
  if (!props.link || submitting.value || destinationError.value) {
    return
  }
  submitting.value = true
  try {
    const updated = await updateLink(props.link.id, {
      destination_url: destinationUrl.value.trim(),
      description: description.value.trim(),
      status: status.value,
      expires_at: expiresAt.value
        ? new Date(expiresAt.value).toISOString()
        : null,
      max_visits: maxVisits.value ?? 0,
      forward_params: forwardParams.value
    })
    useKunMessage('短链已更新', 'success')
    open.value = false
    emit('updated', updated)
  } catch (err) {
    const code = (err as { statusCode?: number }).statusCode
    if (code === 422) {
      useKunMessage('参数不合法：检查目标地址', 'warn')
    } else {
      useKunMessage('更新失败，请重试', 'error')
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <KunModal v-model="open" size="lg">
    <div class="flex flex-col gap-5">
      <div class="flex flex-col gap-1">
        <h2 class="text-lg font-bold">编辑短链</h2>
        <p class="text-sm text-default-500">
          别名
          <span class="font-mono text-primary">/s/{{ link?.alias }}</span>
          不可修改，避免已经发出去的链接失效。
        </p>
      </div>

      <KunInput
        v-model="destinationUrl"
        label="目标地址"
        :error="destinationError"
        required
      />
      <KunInput v-model="description" label="备注" />

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <KunSelect
          v-model="status"
          label="状态"
          :options="LINK_STATUS_OPTIONS"
        />
        <KunNumberInput
          v-model="maxVisits"
          label="最大访问次数"
          description="0 = 不限"
          :min="0"
        />
      </div>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <DashDateTimeField
          v-model="expiresAt"
          label="过期时间"
          description="留空 = 永不过期"
        />
        <div class="flex items-end pb-1">
          <KunSwitch
            v-model="forwardParams"
            label="透传查询参数"
            description="把访问时的查询串追加到目标地址"
          />
        </div>
      </div>

      <KunInfo
        v-if="statusNotice"
        color="warning"
        variant="flat"
        icon="lucide:triangle-alert"
        :description="statusNotice"
      />

      <div class="flex justify-end gap-2">
        <KunButton variant="flat" @click="open = false">取消</KunButton>
        <KunButton
          :loading="submitting"
          :disabled="!!destinationError"
          @click="submit"
        >
          保存
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
