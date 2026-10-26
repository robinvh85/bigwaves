import type { components } from './schema'

type ErrorDetail = components['schemas']['ErrorDetail']

/** Every non-2xx API response is thrown as ApiError, so TanStack Query sees it as an error. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details?: Record<string, unknown>

  constructor(status: number, detail: ErrorDetail) {
    super(detail.message)
    this.name = 'ApiError'
    this.status = status
    this.code = detail.code
    this.details = detail.details
  }

  get isUnauthorized() {
    return this.status === 401
  }
}

/** Network failures (server down, offline) are not ApiErrors; give them a readable message. */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof TypeError) return 'Cannot reach the server'
  if (err instanceof Error) return err.message
  return 'Something went wrong'
}

export function isErrorBody(body: unknown): body is { error: ErrorDetail } {
  return typeof body === 'object' && body !== null && 'error' in body && typeof body.error === 'object'
}
