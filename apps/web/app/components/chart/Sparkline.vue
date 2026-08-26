<script setup lang="ts">
// The trend line inside a stat tile. No axes, no tooltip — the tile's own value
// and delta carry the numbers; this only has to show the shape.
const props = withDefaults(
  defineProps<{
    values: number[]
    color?: string
    height?: number
  }>(),
  { color: undefined, height: 36 }
)

const stroke = computed(() => props.color ?? SERIES_VISITS)

const W = 200

const geometry = computed(() => {
  const values = props.values.length ? props.values : [0, 0]
  const max = Math.max(1, ...values)
  const h = props.height
  const step = values.length > 1 ? W / (values.length - 1) : W
  const points = values.map((v, i) => {
    // Inset by 2px top and bottom so a peak's 2px stroke is never clipped.
    const y = h - 2 - (v / max) * (h - 4)
    return { x: values.length > 1 ? i * step : W / 2, y }
  })
  const line = points.map((p, i) => `${i ? 'L' : 'M'}${p.x} ${p.y}`).join(' ')
  return { line, area: `${line} L${W} ${h} L0 ${h} Z`, last: points.at(-1)! }
})
</script>

<template>
  <svg
    :viewBox="`0 0 ${W} ${height}`"
    :style="{ height: `${height}px` }"
    class="w-full overflow-visible"
    preserveAspectRatio="none"
    aria-hidden="true"
  >
    <path :d="geometry.area" :fill="stroke" fill-opacity="0.1" />
    <path
      :d="geometry.line"
      fill="none"
      :stroke="stroke"
      stroke-width="2"
      stroke-linejoin="round"
      stroke-linecap="round"
      vector-effect="non-scaling-stroke"
    />
  </svg>
</template>
