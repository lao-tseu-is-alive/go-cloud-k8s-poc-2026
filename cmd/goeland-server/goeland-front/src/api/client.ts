/**
 * Minimal REST client for the Goéland server.
 *
 * Every RPC is exposed by the Vanguard transcoder under /api/... . This wrapper
 * attaches the bearer token, serializes JSON, appends query params, and turns
 * non-2xx responses into a typed ApiError carrying the server's Connect-style
 * error code + message (and any buf.validate violation details).
 */

// The current bearer token lives in a module-level variable so the plain fetch
// layer stays decoupled from Pinia. The auth store is the single writer.
let authToken = ''

export function setAuthToken (token: string): void {
  authToken = token
}

export interface ApiViolation {
  field?: string
  constraint?: string
  message?: string
}

/** Normalized API failure. `code` is the Connect error code string when present. */
export class ApiError extends Error {
  readonly status: number
  readonly code?: string
  readonly violations: ApiViolation[]

  constructor (message: string, status: number, code?: string, violations: ApiViolation[] = []) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.violations = violations
  }
}

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'
  query?: Record<string, unknown>
  body?: unknown
  formData?: FormData
  signal?: AbortSignal
}

function buildUrl (path: string, query?: Record<string, unknown>): string {
  if (!query) {
    return path
  }
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (([undefined, null, '', false] as unknown[]).includes(value)) {
      continue
    }
    params.append(key, String(value))
  }
  const qs = params.toString()
  return qs ? `${path}?${qs}` : path
}

/** Maps one buf.validate violation of a Connect error detail. */
function toViolation (raw: unknown): ApiViolation {
  const v = raw as Record<string, unknown>
  return {
    field: typeof v.fieldPath === 'string' ? v.fieldPath : (v.field as string | undefined),
    constraint: v.constraintId as string | undefined,
    message: v.message as string | undefined,
  }
}

/** Collects the buf.validate violations of Connect / google.rpc.Status details. */
function collectViolations (details: unknown): ApiViolation[] {
  if (!Array.isArray(details)) {
    return []
  }
  return details.flatMap(d => {
    const nested = (d as Record<string, unknown>).violations
    return Array.isArray(nested) ? nested.map(v => toViolation(v)) : []
  })
}

/** Picks the first string among the candidate error message fields. */
function firstString (...candidates: unknown[]): string | undefined {
  return candidates.find((c): c is string => typeof c === 'string')
}

async function toApiError (res: Response): Promise<ApiError> {
  const fallback = res.statusText || `HTTP ${res.status}`
  let data: Record<string, unknown>
  try {
    data = await res.json() as Record<string, unknown>
  } catch {
    // non-JSON body; keep the status-derived message
    return new ApiError(fallback, res.status, undefined, [])
  }
  return new ApiError(
    firstString(data.message, data.error) ?? fallback,
    res.status,
    firstString(data.code),
    collectViolations(data.details),
  )
}

export async function apiFetch<T> (path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {}
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`
  }

  let body: BodyInit | undefined
  if (opts.formData) {
    body = opts.formData // browser sets multipart Content-Type + boundary
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }

  const res = await fetch(buildUrl(path, opts.query), {
    method: opts.method ?? 'GET',
    headers,
    body,
    signal: opts.signal,
  })

  if (!res.ok) {
    throw await toApiError(res)
  }
  if (res.status === 204) {
    return undefined as T
  }
  const contentType = res.headers.get('content-type') ?? ''
  if (contentType.includes('application/json')) {
    return await res.json() as T
  }
  return undefined as T
}

/** GETs a binary resource with the bearer token attached and returns the Blob. */
export async function apiFetchBlob (path: string): Promise<Blob> {
  const headers: Record<string, string> = {}
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`
  }
  const res = await fetch(path, { headers })
  if (!res.ok) {
    throw await toApiError(res)
  }
  return await res.blob()
}
