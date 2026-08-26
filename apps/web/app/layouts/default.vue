<script setup lang="ts">
// Default layout: the sticky top-nav shell over every routed page. Theme
// resolution lives here so it is installed once per app, before any page
// renders. The single root <div> keeps the page transition intact.
useTheme()
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <ShellNav />
    <slot />
  </div>
</template>
