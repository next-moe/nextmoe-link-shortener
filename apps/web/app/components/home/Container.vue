<script setup lang="ts">
// Home is the service's front page, not a login form: the login action lives
// in the top bar (ShellNav) where it sits on every route. An already-signed-in
// admin never reads this page — they are sent straight to the dashboard.
const { user, fetched, fetchMe } = useAuth()
const router = useRouter()

onMounted(async () => {
  if (!fetched.value) {
    await fetchMe()
  }
  if (user.value?.isAdmin) {
    await router.replace('/dash')
  }
})

// What the console actually does — the reason someone is looking at this page.
const capabilities = [
  {
    icon: 'lucide:link',
    title: '统一短链',
    text: '整个 NextMoe 生态共用一套 /s/ 短链，别名一经发布不再变更。'
  },
  {
    icon: 'lucide:chart-area',
    title: '访问统计',
    text: '每次跳转都记录访问、独立访客与引荐来源，按小时聚合。'
  },
  {
    icon: 'lucide:key-round',
    title: 'S2S 接入',
    text: '兄弟站点用 API key 直接创建短链，无需人工操作。'
  }
]
</script>

<template>
  <main class="mx-auto flex w-full max-w-4xl grow flex-col justify-center gap-10 px-4 py-16 sm:px-6">
    <section class="flex flex-col items-center gap-5 text-center">
      <span
        class="flex w-fit items-center gap-2 rounded-full bg-primary-100 px-3 py-1 text-xs font-medium text-primary-700"
      >
        <KunIcon name="lucide:shield-check" />
        管理员专用控制台
      </span>

      <h1 class="text-3xl font-semibold sm:text-4xl">NextMoe 短链服务</h1>

      <p class="max-w-xl text-balance text-default-500">
        生态共享的短链接服务：在这里创建和管理 <span class="font-mono">/s/</span>
        短链，查看每条链接的访问情况，并为兄弟站点签发 S2S API key。
      </p>

      <ClientOnly>
        <!-- The IdP account exists but lacks the admin role. This service has
             no self-serve signup, so the only way forward is a human. -->
        <KunInfo
          v-if="user && !user.isAdmin"
          class="max-w-xl text-left"
          color="warning"
          variant="flat"
          icon="lucide:shield-alert"
          title="没有控制台权限"
          :description="`账号 ${user.name} 已登录，但没有管理员角色。本服务不开放自助注册，请联系生态管理员开通。`"
        />
        <p v-else-if="fetched" class="text-sm text-default-400">
          使用右上角的「登录」按钮，通过 NextMoe 统一身份进入控制台。
        </p>
      </ClientOnly>
    </section>

    <ul class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <li v-for="item in capabilities" :key="item.title">
        <KunCard class="h-full">
          <div class="flex flex-col gap-2">
            <span
              class="flex size-9 items-center justify-center rounded-lg bg-default-100"
              aria-hidden="true"
            >
              <KunIcon :name="item.icon" class="text-default-500" />
            </span>
            <span class="text-sm font-medium">{{ item.title }}</span>
            <span class="text-sm text-default-500">{{ item.text }}</span>
          </div>
        </KunCard>
      </li>
    </ul>
  </main>
</template>
