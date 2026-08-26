// Light / dark switching.
//
// KunUI's dark palette hangs off a single `.kun-dark-mode` class, so the whole
// feature is "put a class on <html>". The choice rides in a cookie rather than
// localStorage so the SERVER already knows it and renders the right class —
// no first-paint flash, and no hydration mismatch to paper over.

export type ThemeMode = 'light' | 'dark'

const THEME_COOKIE = 'shortlink_theme'
const DARK_CLASS = 'kun-dark-mode'

export const useTheme = () => {
  const cookie = useCookie<ThemeMode>(THEME_COOKIE, {
    default: () => 'light',
    sameSite: 'lax',
    // A year: the preference should outlive the session, and it is not a
    // credential — nothing here needs httpOnly.
    maxAge: 60 * 60 * 24 * 365
  })

  const mode = computed<ThemeMode>(() => (cookie.value === 'dark' ? 'dark' : 'light'))

  useHead({
    htmlAttrs: { class: computed(() => (mode.value === 'dark' ? DARK_CLASS : '')) }
  })

  const toggle = () => {
    cookie.value = mode.value === 'dark' ? 'light' : 'dark'
  }

  return { mode, toggle }
}
