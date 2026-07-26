// PKCE Authorization Code flow helpers (S256) for the NextMoe IdP. The
// verifier and an anti-CSRF state are stashed in sessionStorage and read back
// on the callback page. The confidential-client backend (the BFF) performs
// the code→token exchange, so the browser never sees tokens or the
// client_secret.

const base64url = (bytes: Uint8Array): string =>
  btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')

const randomVerifier = (): string => {
  const a = new Uint8Array(32)
  crypto.getRandomValues(a)
  return base64url(a)
}

const challengeFromVerifier = async (verifier: string): Promise<string> => {
  const digest = await crypto.subtle.digest(
    'SHA-256',
    new TextEncoder().encode(verifier)
  )
  return base64url(new Uint8Array(digest))
}

const randomState = (): string => {
  const a = new Uint8Array(16)
  crypto.getRandomValues(a)
  return Array.from(a, (b) => b.toString(16).padStart(2, '0')).join('')
}

const VERIFIER_KEY = 'shortlink_oauth_verifier'
const STATE_KEY = 'shortlink_oauth_state'

// LoginStart is the discovery-derived data the backend hands back from
// GET /auth/login. Endpoints come from the OP discovery document — never
// hardcoded on the client.
export interface LoginStart {
  authorizationEndpoint: string
  clientId: string
  redirectUri: string
  scope: string
}

// startOidcLogin generates a fresh verifier + state, stores them, and
// navigates to the OP authorization endpoint with the S256 challenge.
export const startOidcLogin = async (start: LoginStart): Promise<void> => {
  const verifier = randomVerifier()
  const challenge = await challengeFromVerifier(verifier)
  const state = randomState()

  sessionStorage.setItem(VERIFIER_KEY, verifier)
  sessionStorage.setItem(STATE_KEY, state)

  const params = new URLSearchParams({
    client_id: start.clientId,
    redirect_uri: start.redirectUri,
    response_type: 'code',
    scope: start.scope,
    state,
    code_challenge: challenge,
    code_challenge_method: 'S256'
  })
  window.location.href = `${start.authorizationEndpoint}?${params.toString()}`
}

// readOidcCallback validates the returned state against the stored one and
// returns the code + verifier, or null when the callback is invalid (state
// mismatch or missing params). It always clears the one-shot sessionStorage
// keys.
export const readOidcCallback = (): {
  code: string
  codeVerifier: string
} | null => {
  const url = new URLSearchParams(window.location.search)
  const code = url.get('code')
  const returnedState = url.get('state')
  const savedState = sessionStorage.getItem(STATE_KEY)
  const codeVerifier = sessionStorage.getItem(VERIFIER_KEY)

  sessionStorage.removeItem(STATE_KEY)
  sessionStorage.removeItem(VERIFIER_KEY)

  if (!code || !codeVerifier || !returnedState || returnedState !== savedState) {
    return null
  }
  return { code, codeVerifier }
}
