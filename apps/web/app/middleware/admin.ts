// Route middleware for the dashboard pages: resolve the identity once, then
// require an admin. Anonymous users land on the home (login) page; a
// logged-in non-admin sees the home page's "not authorized" notice. Client
// only — the API re-checks the role on every call regardless.
export default defineNuxtRouteMiddleware(async () => {
  if (import.meta.server) return

  const { user, fetched, fetchMe } = useAuth()
  if (!fetched.value) {
    await fetchMe()
  }
  if (!user.value?.isAdmin) {
    return navigateTo('/', { replace: true })
  }
})
