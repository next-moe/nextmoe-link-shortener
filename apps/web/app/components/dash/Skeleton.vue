<script setup lang="ts">
// The console's shape, held open while its numbers are in flight.
//
// This is not decoration, it is the layout reservation. The overview cannot be
// server-rendered: the series is bucketed against the VIEWER's midnight (see
// shared/utils/series.ts) and its axis is formatted in the viewer's timezone,
// so a server render would be built on the server's clock and the browser
// would hydrate a different axis. What can be done is refuse to reflow — every
// block below stands exactly where the card it replaces will be, so the real
// content drops into the same box instead of shoving the page around when the
// fetch lands.
//
// The heights are measured, not guessed, and they step on the same breakpoints
// the real grid does. The tiles and the chart are exact: both are fixed-shape
// cards, and the chart's SVG has a fixed viewBox height, so 396px holds at
// every width. The breakdown pair and the table are sized by their own data,
// so those two are the typical case rather than a guarantee — and the table
// deliberately reserves LESS than it usually needs, because content growing
// downwards past the fold is calmer to read than a block collapsing upwards
// under the reader's eyes.

// A tile's height is set by its footer band: the first two end in a sparkline,
// the last two in a legend / meter that runs a line taller. Once the grid is
// four columns wide they all stretch to the tallest, which is why the lg step
// converges.
const TILE_SIZES = [
  'h-[156px] sm:h-[160px] lg:h-[175px]',
  'h-[156px] sm:h-[160px] lg:h-[175px]',
  'h-[167px] sm:h-[175px]',
  'h-[167px] sm:h-[175px]'
]
</script>

<template>
  <div
    class="flex flex-col gap-6"
    role="status"
    aria-label="正在加载控制台数据"
  >
    <section class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <div v-for="(size, i) in TILE_SIZES" :key="`tile-${i}`" :class="size">
        <KunSkeleton height="100%" rounded="lg" />
      </div>
    </section>

    <div class="h-[396px]">
      <KunSkeleton height="100%" rounded="lg" />
    </div>

    <section class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <div
        v-for="i in 2"
        :key="`split-${i}`"
        class="h-[326px] lg:h-[400px]"
      >
        <KunSkeleton height="100%" rounded="lg" />
      </div>
    </section>

    <div class="h-[640px]">
      <KunSkeleton height="100%" rounded="lg" />
    </div>
  </div>
</template>
