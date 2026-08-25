<script setup lang="ts">
// Home = the login gate. Anonymous → OIDC login button; logged-in admin →
// straight to the dashboard; logged-in non-admin → a "not authorized" notice
// (the IdP account exists but lacks the admin role).
const { user, fetched, fetchMe, login } = useAuth()
const router = useRouter()
const starting = ref(false)

onMounted(async () => {
  if (!fetched.value) {
    await fetchMe()
  }
  if (user.value?.isAdmin) {
    await router.replace('/dash')
  }
})

const handleLogin = async () => {
  starting.value = true
  try {
    await login()
  } catch {
    starting.value = false
  }
}
</script>

<template>
  <main class="flex min-h-[80vh] items-center justify-center px-6">
    <KunCard class="flex w-full max-w-md flex-col items-center gap-6 p-10 text-center">
      <img
        src="/apple-touch-icon.png"
        alt=""
        width="80"
        height="80"
        class="size-20 rounded-2xl"
      />
      <div class="flex flex-col gap-2">
        <h1 class="text-xl font-semibold">KunGal Link Shortener</h1>
        <p class="text-sm text-default-500">
          NextMoe 生态共享短链接服务 · 管理员专用控制台
        </p>
      </div>

      <template v-if="user && !user.isAdmin">
        <p class="text-sm text-danger">
          当前账号（{{ user.name }}）没有管理员权限，无法使用本控制台。
        </p>
      </template>
      <template v-else>
        <KunButton :loading="starting" @click="handleLogin">
          使用鲲 Galgame 账号登录
        </KunButton>
      </template>
    </KunCard>
  </main>
</template>
