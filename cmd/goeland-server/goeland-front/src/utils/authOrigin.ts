/** Helpers for signing in through the external auth service. */

const LOOPBACK_HOSTS = new Set(['localhost', '127.0.0.1', '[::1]'])

/**
 * When the SPA and the auth service both run on the local machine under different
 * loopback names (e.g. 127.0.0.1 vs localhost), the auth service's redirect
 * allowlist, CORS and cookies treat them as different origins and sign-in fails.
 * Returns the current URL rewritten to the auth service's host name, or undefined
 * when there is no such mismatch.
 */
export function loopbackMismatchUrl (currentHref: string, authBaseUrl: string): string | undefined {
  let current: URL
  let auth: URL
  try {
    current = new URL(currentHref)
    auth = new URL(authBaseUrl)
  } catch {
    return undefined
  }
  if (current.hostname === auth.hostname) {
    return undefined
  }
  if (!LOOPBACK_HOSTS.has(current.hostname) || !LOOPBACK_HOSTS.has(auth.hostname)) {
    return undefined
  }
  current.hostname = auth.hostname
  return current.toString()
}
