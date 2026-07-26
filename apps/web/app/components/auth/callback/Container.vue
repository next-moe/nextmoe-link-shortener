<script setup lang="ts">
// Handles the OP redirect: validate state → POST {code, code_verifier} to the
// BFF → refresh identity → land on the dashboard. On failure, show a readable
// message.
const router = useRouter()
const { fetchMe } = useAuth()
const error = ref('')

onMounted(async () => {
  const cb = readOidcCallback()
  if (!cb) {
    error.value = '登录校验失败：state 不匹配或回调参数缺失，请重新登录。'
    return
  }
  try {
    await $fetch('/api/auth/session', {
      method: 'POST',
      body: { code: cb.code, code_verifier: cb.codeVerifier }
    })
  } catch {
    error.value = '登录失败，请返回首页重试。'
    return
  }
  const me = await fetchMe()
  await router.replace(me?.isAdmin ? '/dash' : '/')
})
</script>

<template>
  <main class="flex min-h-[80vh] items-center justify-center px-6">
    <KunCard class="flex max-w-md flex-col items-center gap-4 p-10 text-center">
      <p v-if="!error" class="text-default-500">正在登录...</p>
      <template v-else>
        <p class="text-danger">{{ error }}</p>
        <KunButton variant="light" href="/">返回首页</KunButton>
      </template>
    </KunCard>
  </main>
</template>
