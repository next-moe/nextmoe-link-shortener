// Route middleware for the dashboard pages: require an admin. Anonymous users
// land on the home (login) page; a logged-in non-admin sees the home page's
// "not authorized" notice.
//
// The identity is already resolved by plugins/auth.ts — plugins finish before
// the first navigation — so this runs on the SERVER too: an anonymous request
// for /dash is redirected before any dashboard HTML is produced, instead of
// rendering the whole console and bouncing after hydration. The API re-checks
// the role on every call regardless.
export default defineNuxtRouteMiddleware(() => {
  const { user } = useAuth()
  if (!user.value?.isAdmin) {
    return navigateTo('/', { replace: true })
  }
})
