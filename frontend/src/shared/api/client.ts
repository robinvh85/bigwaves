import createClient, { type Middleware } from 'openapi-fetch'

import { ApiError, isErrorBody } from './errors'
import type { paths } from './schema'

type UnauthorizedHandler = () => void
let onUnauthorized: UnauthorizedHandler = () => {}

/** The router registers how to react to a 401 (clear cache, go to /login). */
export function setUnauthorizedHandler(fn: UnauthorizedHandler) {
  onUnauthorized = fn
}

const throwOnError: Middleware = {
  async onResponse({ response }) {
    if (response.ok) return response
    const body: unknown = await response
      .clone()
      .json()
      .catch(() => null)
    const detail = isErrorBody(body)
      ? body.error
      : { code: `http_${response.status}`, message: response.statusText || 'Request failed' }
    if (response.status === 401) onUnauthorized()
    throw new ApiError(response.status, detail)
  },
}

export const api = createClient<paths>({ baseUrl: '/api/v1', credentials: 'same-origin' })
api.use(throwOnError)

/**
 * Unwraps an openapi-fetch result. Errors were already thrown by the middleware, so a missing
 * `data` here only happens for 204 responses.
 */
export async function unwrap<T>(p: Promise<{ data?: T; error?: unknown }>): Promise<T> {
  const { data } = await p
  return data as T
}
