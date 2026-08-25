<script setup lang="ts">
// Root component. Pages render through <NuxtLayout><NuxtPage> so the default
// layout (top nav shell) wraps every route; per-page business UI lives in
// components/<page>/ per the CLAUDE.md splitting rules. KunMessageProvider
// hosts the toast surface (useKunMessage).
useHead({
  title: 'NextMoe Link Shortener',
  htmlAttrs: { lang: 'zh-CN' }
})
</script>

<template>
  <div>
    <NuxtRouteAnnouncer />
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>
    <KunMessageProvider />
  </div>
</template>
