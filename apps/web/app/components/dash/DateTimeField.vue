<script setup lang="ts">
// Expiry needs a date AND a time (a link that dies at midnight is rarely what
// the operator means), and KunDatePicker is date-only — so this is the one
// control with no KunUI equivalent. It wraps the native datetime-local input
// and borrows KunUI's own control tokens so it sits flush with the KunInputs
// beside it instead of looking like a browser default dropped into the form.
const model = defineModel<string>({ required: true })

defineProps<{
  label: string
  description?: string
}>()

const id = useKunUniqueId('datetime')
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <label :for="id" class="text-sm text-default-700">{{ label }}</label>
    <input
      :id="id"
      v-model="model"
      type="datetime-local"
      class="h-10 w-full rounded-kun-md border border-kun bg-transparent px-3 text-sm transition-colors focus-visible:border-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
    >
    <p v-if="description" class="text-xs text-default-400">{{ description }}</p>
  </div>
</template>
