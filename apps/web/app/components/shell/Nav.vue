<script setup lang="ts">
const { user, fetched, fetchMe, logout } = useAuth()
const router = useRouter()

onMounted(async () => {
  if (!fetched.value) {
    await fetchMe()
  }
})

const handleLogout = async () => {
  await logout()
  await router.push('/')
}
</script>

<template>
  <header class="border-b border-default-200">
    <nav class="mx-auto flex h-14 max-w-5xl items-center gap-4 px-4">
      <NuxtLink to="/" class="flex items-center gap-2 font-semibold">
        <img
          src="/apple-touch-icon.png"
          alt=""
          width="28"
          height="28"
          class="size-7 rounded-lg"
        />
        <span>KunGal Link Shortener</span>
      </NuxtLink>

      <div class="grow" />

      <template v-if="user?.isAdmin">
        <KunButton variant="light" size="sm" href="/dash"> 短链管理 </KunButton>
        <KunButton variant="light" size="sm" href="/keys"> API Keys </KunButton>
      </template>

      <template v-if="user">
        <span class="text-sm text-default-500">{{ user.name }}</span>
        <KunButton variant="light" size="sm" color="danger" @click="handleLogout">
          退出
        </KunButton>
      </template>
    </nav>
  </header>
</template>
