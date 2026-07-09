import type { FrontendConfig, TokenResponse, TokenUser } from '@/api/types'
/**
 * Authentication store (setup style).
 *
 * Bootstraps from GET /config, then supports two modes :
 *   - dev:  a static token is entered manually for local hacking.
 *   - jwt:  short-lived JWTs are minted silently from the SSO session cookie
 *           at ${authBaseUrl}/auth/token and re-minted before expiry.
 *
 * The token is kept in memory only (never localStorage) and mirrored into the
 * REST client via setAuthToken so every apiFetch carries it.
 */
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { setAuthToken } from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const mode = ref<'dev' | 'jwt'>('jwt')
  const authBaseUrl = ref('')
  const token = ref('')
  const user = ref<TokenUser | null>(null)
  const ready = ref(false)
  const error = ref('')
  let remintTimer: ReturnType<typeof setTimeout> | null = null

  const isAuthenticated = computed(() => token.value !== '')
  const displayName = computed(() =>
    user.value?.name || user.value?.email || (mode.value === 'dev' ? 'Local dev user' : ''))

  function setToken (newToken: string, newUser: TokenUser | null): void {
    token.value = newToken
    user.value = newUser
    setAuthToken(newToken)
  }

  function clear (): void {
    token.value = ''
    user.value = null
    setAuthToken('')
    if (remintTimer) {
      clearTimeout(remintTimer)
      remintTimer = null
    }
  }

  function scheduleRemint (expiresInSeconds: number): void {
    if (remintTimer) {
      clearTimeout(remintTimer)
    }
    // Re-mint at ~80% of the token lifetime, but never after it expires: for
    // short-lived tokens we cap the delay to at least 5s before expiry, and keep
    // a 1s floor so a zero/degenerate lifetime cannot schedule a tight loop.
    const lifetimeMs = Math.max(expiresInSeconds, 0) * 1000
    const delayMs = Math.max(Math.min(lifetimeMs * 0.8, lifetimeMs - 5_000), 1_000)
    remintTimer = setTimeout(() => {
      void mintToken()
    }, delayMs)
  }

  /** jwt mode: silently mint a JWT from the SSO session cookie. */
  async function mintToken (): Promise<boolean> {
    try {
      const res = await fetch(`${authBaseUrl.value}/auth/token`, { credentials: 'include' })
      if (res.status === 401) {
        clear()
        return false
      }
      if (!res.ok) {
        error.value = `Token mint failed (HTTP ${res.status})`
        return false
      }
      const data = await res.json() as TokenResponse
      setToken(data.token, data.user)
      scheduleRemint(data.expires_in_seconds)
      error.value = ''
      return true
    } catch {
      error.value = `Auth service unreachable at ${authBaseUrl.value}`
      return false
    }
  }

  /** Loads /config and, in jwt mode, attempts a silent token mint. */
  async function bootstrap (): Promise<void> {
    try {
      const res = await fetch('/config')
      if (res.ok) {
        const cfg = await res.json() as FrontendConfig
        mode.value = cfg.authMode === 'dev' ? 'dev' : 'jwt'
        authBaseUrl.value = cfg.authBaseUrl ?? ''
      }
    } catch {
      // assume jwt mode if /config is unreachable
    }
    if (mode.value === 'jwt') {
      await mintToken()
    }
    ready.value = true
  }

  /** dev mode: apply a manually entered static token. */
  function applyDevToken (rawToken: string): void {
    const trimmed = rawToken.trim()
    if (!trimmed) {
      return
    }
    setToken(trimmed, { name: 'Local dev user', email: 'dev@localhost' })
    error.value = ''
  }

  /** jwt mode: redirect to the external auth service login page. */
  function signIn (): void {
    const redirect = encodeURIComponent(window.location.href)
    window.location.href = `${authBaseUrl.value}/auth/login?redirect_uri=${redirect}`
  }

  async function signOut (): Promise<void> {
    if (mode.value === 'jwt') {
      try {
        await fetch(`${authBaseUrl.value}/auth/logout`, { method: 'POST', credentials: 'include' })
      } catch {
        // ignore network errors on logout
      }
    }
    clear()
  }

  return {
    mode, authBaseUrl, token, user, ready, error,
    isAuthenticated, displayName,
    bootstrap, applyDevToken, mintToken, signIn, signOut, clear,
  }
})
