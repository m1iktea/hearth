interface ApiResponse<T> {
  success: boolean
  data: T
  error?: string
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  const env = (await res.json()) as ApiResponse<T>
  if (!res.ok || !env.success) {
    throw new Error(env.error ?? `HTTP ${res.status}`)
  }
  return env.data
}

export const apiGet = <T>(path: string) => request<T>('GET', path)
export const apiPost = <T>(path: string, body: unknown) => request<T>('POST', path, body)
export const apiPut = <T>(path: string, body: unknown) => request<T>('PUT', path, body)
export const apiDelete = <T>(path: string) => request<T>('DELETE', path)
