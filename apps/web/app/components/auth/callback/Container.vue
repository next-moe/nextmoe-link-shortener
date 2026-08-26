<script setup lang="ts">
// Handles the OP redirect: validate state → POST {code, code_verifier} to the
// BFF → refresh identity → land on the dashboard. On failure, say which step
// failed, because "登录失败" alone leaves the operator with nowhere to go.
const router = useRouter()
const { fetchMe } = useAuth()
const error = ref('')

onMounted(async () => {
  const cb = readOidcCallback()
  if (!cb) {
    error.value = '登录校验失败：state 不匹配或回调参数缺失。请返回首页重新登录。'
    return
  }
  try {
    await $fetch('/api/auth/session', {
      method: 'POST',
      body: { code: cb.code, code_verifier: cb.codeVerifier }
    })
  } catch {
    error.value = '换取会话失败：授权码可能已过期。请返回首页重试。'
    return
  }
  const me = await fetchMe()
  await router.replace(me?.isAdmin ? '/dash' : '/')
})
</script>

<template>
  <main class="flex grow items-center justify-center px-6 py-16">
    <KunCard class="w-full max-w-md">
      <div class="flex flex-col items-center gap-5 py-4 text-center">
        <template v-if="!error">
          <KunLoading />
          <div class="flex flex-col gap-1">
            <p class="font-medium">正在完成登录</p>
            <p class="text-sm text-default-400">正在与身份服务交换会话…</p>
          </div>
        </template>

        <template v-else>
          <span
            class="flex size-12 items-center justify-center rounded-full bg-danger-100"
            aria-hidden="true"
          >
            <KunIcon name="lucide:triangle-alert" class="text-xl text-danger" />
          </span>
          <p class="text-sm text-default-600">{{ error }}</p>
          <KunButton variant="flat" href="/">返回首页</KunButton>
        </template>
      </div>
    </KunCard>
  </main>
</template>
