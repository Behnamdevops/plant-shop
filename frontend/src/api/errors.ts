// ApiError carries the HTTP status code alongside the message so callers can
// branch on specific statuses (401/403/404/409) instead of parsing message
// text. Existing api modules that already throw plain Error are left as-is
// to avoid a broad rewrite; new admin endpoints use this.
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export async function readErrorMessage(response: Response): Promise<string> {
  const text = await response.text()
  return text.trim() || `Request failed with status ${response.status}`
}

export async function throwApiError(response: Response): Promise<never> {
  throw new ApiError(response.status, await readErrorMessage(response))
}
