import tailwindcss from '@tailwindcss/vite'

// The Go API origin the /api and /s prefixes are proxied to. Same-origin BFF
// posture: the browser only ever calls /api on this origin (dev AND prod), so
// there is zero CORS surface and the httpOnly session cookie is always
// first-party. /s/** rides the same proxy so short links share the site
// origin. In dev, Nitro's route-rule proxy forwards to the local API; in prod
// the same rule points at the internal API service (override via
// SHORTLINK_API_PROXY_TARGET).
const apiProxyTarget =
  process.env.SHORTLINK_API_PROXY_TARGET || 'http://127.0.0.1:7845'

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2026-07-15',

  // Dev server runs on 7844 (the project's historical dev port). Bind
  // 127.0.0.1 explicitly: the OIDC redirect_uri speaks IPv4 loopback, so the
  // browser must reach the dev server there.
  devServer: { port: 7844, host: '127.0.0.1' },

  // Strip the /api prefix and forward to the Go API; forward /s/** verbatim
  // (the API serves the redirect at /s/{alias}).
  routeRules: {
    '/api/**': { proxy: `${apiProxyTarget}/**` },
    '/s/**': { proxy: `${apiProxyTarget}/s/**` }
  },

  // apiBase is where SSR data fetches would hit the Go API directly. The
  // dashboard is fully client-gated (admin-only), so it is currently unused
  // by pages, but the composable keeps the standard shape.
  runtimeConfig: {
    apiBase: apiProxyTarget
  },

  // KunUI is consumed as the published Nuxt layer (@kungal/ui-nuxt): it
  // auto-imports every <Kun*> component + composable from @kungal/ui-vue and
  // wires @nuxt/icon / @nuxt/image / NuxtLink. The layer ships NO Tailwind
  // entry of its own — this app owns that in app/styles/index.css.
  extends: ['@kungal/ui-nuxt'],

  modules: ['@nuxt/eslint'],

  // Auto-import app/constants (project convention: all constants live there).
  imports: {
    dirs: ['constants']
  },

  // @nuxt/icon (wired by the KunUI layer) defaults its client endpoint to
  // /api/_nuxt_icon — which our '/api/**' proxy would forward to the Go API
  // (404). Move it off the proxied prefix so lucide icons resolve locally.
  icon: {
    localApiEndpoint: '/_nuxt_icon'
  },

  css: ['~/styles/index.css'],

  // Tailwind v4 runs through its Vite plugin (no PostCSS config needed).
  vite: {
    plugins: [tailwindcss()]
  },

  devtools: { enabled: false }
})
