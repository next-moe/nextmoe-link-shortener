// Chart tokens.
//
// The categorical slots are KunUI's `-500` steps, which sit at OKLCH L 0.62 in
// BOTH themes (the ramp mirrors around it) — so one `var()` reference is
// correct on the light and the dark surface alike, with no per-theme table.
//
// The ORDER is the colorblind-safety mechanism and must not be reshuffled:
// it was picked by enumerating every ordering of KunUI's hues and keeping the
// one with the widest worst adjacent pair. Verified with the data-viz
// validator against both surfaces (white / content1-dark):
//
//   worst adjacent pair  info↔success  ΔE 15.0 (deuteranopia)  — target is 8
//   normal-vision floor  info↔success  ΔE 16.3                 — floor is 15
//   contrast vs surface  all 5 ≥ 3:1
//
// `danger` is deliberately NOT a categorical slot: it stays reserved for
// destructive actions and error states, so a series can never impersonate one.
export const CHART_SLOTS = [
  'oklch(var(--primary-500))',
  'oklch(var(--success-500))',
  'oklch(var(--info-500))',
  'oklch(var(--warning-500))',
  'oklch(var(--secondary-500))'
] as const

// Anything past the last slot folds into one neutral "other" bucket. Generating
// a sixth hue would be indistinguishable from an existing one under CVD.
export const CHART_OTHER_COLOR = 'oklch(var(--default-400))'
export const CHART_OTHER_LABEL = '其他'

// slotColor assigns a categorical color by index, folding the tail.
export const slotColor = (index: number): string =>
  CHART_SLOTS[index] ?? CHART_OTHER_COLOR

// Series identity for the traffic chart. Visits and unique visitors share one
// unit and one axis (unique ⊆ visits), so they are two slots of one scale —
// never a second y-axis.
export const SERIES_VISITS = CHART_SLOTS[0]
export const SERIES_UNIQUE = CHART_SLOTS[1]

// Recessive chrome: gridlines and axis rules sit one step off the surface and
// stay hairline-solid (a dashed grid reads as a threshold that isn't there).
export const CHART_GRID_COLOR = 'oklch(var(--default-200))'

// The surface the 2px mark gaps and marker rings are cut out of. Charts live
// inside a KunCard, whose fill is `content1`.
export const CHART_SURFACE = 'oklch(var(--content1))'
