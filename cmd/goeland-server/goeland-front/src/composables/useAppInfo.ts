import { ref } from 'vue'

/** The build identity the server publishes at GET /goAppInfo (no authentication needed). */
export interface AppInfo {
  version?: string
  revision?: string
  build?: string
}

const info = ref<AppInfo>({})
let requested = false

/**
 * The running server version (GLD-063): shown in the app bar so that a report
 * names the exact version. Fetched once per page load.
 */
export function useAppInfo () {
  if (!requested) {
    requested = true
    fetch('/goAppInfo')
      .then(async res => (res.ok ? (await res.json()) as AppInfo : {}))
      .then(value => {
        info.value = value
      })
      .catch(() => {
        requested = false // retried by the next caller
      })
  }
  return info
}

/** The revision and build of a build identity, leaving out the values a local build lacks ("unknown"). */
export function buildHint (info: AppInfo, t: (key: string, values: Record<string, string>) => string): string {
  const known = (v?: string) => (v && v !== 'unknown' ? v : '')
  const revision = known(info.revision)
  const build = known(info.build)
  if (!revision && !build) {
    return ''
  }
  return t('app.versionHint', { revision: revision || '—', build: build || '—' })
}
