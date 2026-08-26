// The front page is for people who are not in the console yet. An admin has
// nothing to read there, so they go straight to the dashboard — server-side on
// a fresh load, which is what keeps the browser from painting the front page
// first and replacing it a moment later.
export default defineNuxtRouteMiddleware(() => {
  const { user } = useAuth()
  if (user.value?.isAdmin) {
    return navigateTo('/dash', { replace: true })
  }
})
