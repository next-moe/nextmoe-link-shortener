// Resolve the signed-in identity ONCE per page load, before anything renders.
//
// Plugins run ahead of route middleware and ahead of the first render, and on
// the server `useState` is serialized into the payload — so the client
// hydrates with the identity already known and skips the fetch (`fetched` is
// already true). Without this the top bar had nothing to draw until an
// after-mount round-trip came back, which is what made the login button and
// the identity chip pop in and shift the page.
export default defineNuxtPlugin({
  name: 'shortlink-auth',
  enforce: 'pre',
  async setup() {
    const { fetched, fetchMe } = useAuth()
    if (!fetched.value) {
      await fetchMe()
    }
  }
})
