<script setup lang="ts">
// Default layout: the minimal top nav shell over every routed page. The
// single root <div> keeps the page transition intact.
</script>

<template>
  <div class="min-h-screen">
    <ShellNav />
    <slot />
  </div>
</template>
