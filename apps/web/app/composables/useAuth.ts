// Minimal auth composable: reads the current identity from the BFF
// (/auth/me), starts the OIDC login, and logs out. All calls go to the
// same-origin /api prefix (proxied to the Go API), so the httpOnly session
// cookie rides along and no token ever touches this code.
//
// The identity resolves during SSR (see plugins/auth.ts), so every render —
// server and client — already knows who is signed in. That is what keeps the
// top bar from swapping a login button for an avatar after hydration.

export interface AuthUser {
  userId: number
  name: string
  roles: string[]
  isAdmin: boolean
}

interface MeResponse {
  user_id: number
  name: string
  roles: string[]
  is_admin: boolean
}

interface LoginStartResponse {
  authorization_endpoint: string
  client_id: string
  redirect_uri: string
  scope: string
}

export const useAuth = () => {
  const user = useState<AuthUser | null>('auth-user', () => null)
  const fetched = useState<boolean>('auth-fetched', () => false)

  // useRequestFetch forwards the INCOMING request's headers on the server, so
  // the browser's session cookie reaches the BFF during SSR; on the client it
  // is plain $fetch. Capture it here, in setup context, because it needs the
  // current request event.
  const request = useRequestFetch()

  // fetchMe resolves the current identity, or null when anonymous (401).
  const fetchMe = async (): Promise<AuthUser | null> => {
    try {
      const me = await request<MeResponse>('/api/auth/me')
      user.value = {
        userId: me.user_id,
        name: me.name,
        roles: me.roles ?? [],
        isAdmin: me.is_admin
      }
    } catch {
      user.value = null
    }
    fetched.value = true
    return user.value
  }

  // login asks the backend for the discovery-derived authorize parameters,
  // then hands off to the PKCE redirect.
  const login = async (): Promise<void> => {
    const start = await $fetch<LoginStartResponse>('/api/auth/login')
    await startOidcLogin({
      authorizationEndpoint: start.authorization_endpoint,
      clientId: start.client_id,
      redirectUri: start.redirect_uri,
      scope: start.scope
    })
  }

  const logout = async (): Promise<void> => {
    await $fetch('/api/auth/session', { method: 'DELETE' })
    user.value = null
  }

  return { user, fetched, fetchMe, login, logout }
}
