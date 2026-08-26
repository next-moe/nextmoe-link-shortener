<script setup lang="ts">
// The console's top bar: brand, primary navigation with an active state, the
// theme switch, and the signed-in identity.
//
// The identity is resolved client-side (the session cookie is read by the BFF,
// and every dashboard fetch is client-only), so the auth-dependent half of the
// bar is wrapped in <ClientOnly>. Rendering it unguarded is what produced the
// hydration mismatch this component used to log on every page load: the server
// had no user and emitted a different node count than the client.
const { user, fetched, fetchMe, login, logout } = useAuth()
const { mode, toggle } = useTheme()
const route = useRoute()
const router = useRouter()

onMounted(async () => {
  if (!fetched.value) {
    await fetchMe()
  }
})

// Login lives here rather than on the home page: it is the same action from
// every route, so it belongs in the persistent bar next to the identity it
// replaces.
const starting = ref(false)

const handleLogin = async () => {
  starting.value = true
  try {
    await login()
  } catch {
    starting.value = false
    useKunMessage('无法开始登录，请稍后重试', 'error')
  }
}

const handleLogout = async () => {
  await logout()
  await router.push('/')
}

const links = [
  { to: '/dash', label: '控制台', icon: 'lucide:layout-dashboard' },
  { to: '/keys', label: 'API Keys', icon: 'lucide:key-round' }
]

const isActive = (to: string) => route.path === to
</script>

<template>
  <header
    class="sticky top-0 z-30 border-b border-kun bg-background/85 backdrop-blur"
  >
    <nav
      class="mx-auto flex h-16 max-w-[1400px] items-center gap-2 px-4 sm:px-6"
      aria-label="主导航"
    >
      <NuxtLink
        to="/"
        class="flex shrink-0 items-center gap-2.5 rounded-lg py-1 pr-2 transition-opacity hover:opacity-80"
      >
        <img
          src="/apple-touch-icon.png"
          alt=""
          width="32"
          height="32"
          class="size-8 rounded-lg"
        >
        <span class="flex flex-col leading-none">
          <span class="text-sm font-semibold">NextMoe 短链</span>
          <span class="mt-0.5 hidden text-[11px] text-default-400 sm:block">
            生态共享短链接服务
          </span>
        </span>
      </NuxtLink>

      <ClientOnly>
        <div v-if="user?.isAdmin" class="ml-1 flex items-center gap-1 sm:ml-4">
          <NuxtLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2.5 py-2 text-sm transition-colors sm:px-3"
            :class="
              isActive(link.to)
                ? 'bg-primary-100 font-medium text-primary-700'
                : 'text-default-600 hover:bg-default-100'
            "
            :aria-current="isActive(link.to) ? 'page' : undefined"
            :aria-label="link.label"
          >
            <KunIcon :name="link.icon" class="text-base" />
            <!-- Narrow viewports keep the icon and drop the label: two wrapped
                 two-character labels are less legible than two clear icons. -->
            <span class="hidden sm:inline">{{ link.label }}</span>
          </NuxtLink>
        </div>
      </ClientOnly>

      <div class="grow" />

      <KunTooltip :text="mode === 'dark' ? '切换到浅色' : '切换到深色'">
        <KunButton
          variant="light"
          size="sm"
          is-icon-only
          :aria-label="mode === 'dark' ? '切换到浅色主题' : '切换到深色主题'"
          @click="toggle"
        >
          <KunIcon :name="mode === 'dark' ? 'lucide:sun' : 'lucide:moon'" />
        </KunButton>
      </KunTooltip>

      <ClientOnly>
        <div v-if="user" class="flex items-center gap-2">
          <span
            class="hidden items-center gap-2 rounded-full bg-default-100 py-1 pl-1 pr-3 sm:flex"
          >
            <span
              class="flex size-6 items-center justify-center rounded-full bg-primary text-[11px] font-semibold text-primary-foreground"
              aria-hidden="true"
            >{{ user.name.slice(0, 1).toUpperCase() }}</span>
            <span class="max-w-32 truncate text-xs">{{ user.name }}</span>
          </span>
          <KunTooltip text="退出登录">
            <KunButton
              variant="light"
              size="sm"
              color="danger"
              is-icon-only
              aria-label="退出登录"
              @click="handleLogout"
            >
              <KunIcon name="lucide:log-out" />
            </KunButton>
          </KunTooltip>
        </div>

        <!-- Rendered only once the identity is known, so the bar never flashes
             a login button at someone who is already signed in. -->
        <KunButton
          v-else-if="fetched"
          size="sm"
          icon
          :loading="starting"
          @click="handleLogin"
        >
          <template #icon>
            <KunIcon name="lucide:log-in" />
          </template>
          登录
        </KunButton>
      </ClientOnly>
    </nav>
  </header>
</template>
