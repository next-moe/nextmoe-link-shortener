<script setup lang="ts">
// Creating a link. Destination is the only required field; everything else has
// a sane default, so the form opens as one input and the lifetime controls stay
// folded away until they are wanted.
//
// This used to be an inline panel that pushed the whole dashboard down when it
// expanded. A modal keeps the page behind it still.
import type { LinkDTO } from '~~/shared/types/shortlink'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [boolean]
  created: [LinkDTO]
}>()

const { createLink } = useApi()

const open = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const destinationUrl = ref('')
const alias = ref('')
const description = ref('')
const expiresAt = ref('')
// KunNumberInput's model is number | null (a cleared input is null).
const maxVisits = ref<number | null>(0)
const forwardParams = ref(false)
const showAdvanced = ref(false)
const submitting = ref(false)

const reset = () => {
  destinationUrl.value = ''
  alias.value = ''
  description.value = ''
  expiresAt.value = ''
  maxVisits.value = 0
  forwardParams.value = false
  showAdvanced.value = false
}

watch(open, (isOpen) => {
  if (isOpen) {
    reset()
  }
})

// Validate the destination in the form rather than only on the 422 that comes
// back, so the operator is told before the round trip.
const destinationError = computed(() => {
  const raw = destinationUrl.value.trim()
  if (!raw) {
    return ''
  }
  try {
    const url = new URL(raw)
    return url.protocol === 'http:' || url.protocol === 'https:'
      ? ''
      : '必须是 http(s) 开头的绝对地址'
  } catch {
    return '请填写完整的目标地址，例如 https://www.kungal.com/…'
  }
})

const aliasError = computed(() => {
  const raw = alias.value.trim()
  if (!raw) {
    return ''
  }
  return /^[A-Za-z0-9_-]{4,32}$/.test(raw)
    ? ''
    : '4-32 位，仅限字母 / 数字 / _ / -'
})

const canSubmit = computed(
  () =>
    destinationUrl.value.trim() !== '' &&
    !destinationError.value &&
    !aliasError.value
)

const submit = async () => {
  if (submitting.value || !canSubmit.value) {
    return
  }
  submitting.value = true
  try {
    const link = await createLink({
      destination_url: destinationUrl.value.trim(),
      alias: alias.value.trim() || undefined,
      description: description.value.trim() || undefined,
      expires_at: expiresAt.value
        ? new Date(expiresAt.value).toISOString()
        : undefined,
      max_visits:
        maxVisits.value && maxVisits.value > 0 ? maxVisits.value : undefined,
      forward_params: forwardParams.value || undefined
    })
    useKunMessage(`短链 /s/${link.alias} 已创建`, 'success')
    open.value = false
    emit('created', link)
  } catch (err) {
    const status = (err as { statusCode?: number }).statusCode
    if (status === 409) {
      useKunMessage('别名已被占用，换一个试试', 'warn')
    } else if (status === 422) {
      useKunMessage('参数不合法：检查目标地址与别名格式', 'warn')
    } else {
      useKunMessage('创建失败，请重试', 'error')
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
        <h2 class="text-lg font-bold">新建短链</h2>
        <p class="text-sm text-default-500">
          留空别名会随机生成 6 位；创建后别名不可修改。
        </p>
      </div>

      <KunInput
        v-model="destinationUrl"
        label="目标地址"
        placeholder="https://www.kungal.com/topic/1024"
        :error="destinationError"
        required
        autofocus
        @keyup.enter="submit"
      />

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <KunInput
          v-model="alias"
          label="自定义别名"
          placeholder="留空 = 随机 6 位"
          :error="aliasError"
          :description="aliasError ? '' : '4-32 位，字母 / 数字 / _ / -'"
        />
        <KunInput
          v-model="description"
          label="备注"
          placeholder="这条短链是干什么的"
          description="只在控制台展示"
        />
      </div>

      <div class="rounded-kun-md border border-kun">
        <button
          type="button"
          class="flex w-full items-center gap-2 px-4 py-3 text-sm transition-colors hover:bg-default-100"
          :aria-expanded="showAdvanced"
          @click="showAdvanced = !showAdvanced"
        >
          <KunIcon
            name="lucide:chevron-right"
            class="text-default-400 transition-transform"
            :class="showAdvanced ? 'rotate-90' : ''"
          />
          <span>生命周期与参数</span>
          <span class="text-xs text-default-400">（可选）</span>
        </button>

        <div
          v-if="showAdvanced"
          class="flex flex-col gap-4 border-t border-kun p-4"
        >
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
            <DashDateTimeField
              v-model="expiresAt"
              label="过期时间"
              description="留空 = 永不过期"
            />
            <KunNumberInput
              v-model="maxVisits"
              label="最大访问次数"
              description="0 = 不限"
              :min="0"
            />
          </div>
          <KunSwitch
            v-model="forwardParams"
            label="透传查询参数"
            description="访问 /s/alias?a=1 时把 ?a=1 追加到目标地址"
          />
        </div>
      </div>

      <div class="flex justify-end gap-2">
        <KunButton variant="flat" @click="open = false">取消</KunButton>
        <KunButton
          :loading="submitting"
          :disabled="!canSubmit"
          @click="submit"
        >
          创建短链
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
