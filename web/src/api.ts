export interface InventoryItem {
  sku: string
  name: string
  quantity: number
  updated_at?: string
}

interface ApiErrorBody {
  error?: string
}

function createIdempotencyKey(): string {
  if (typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }

  return Array.from(crypto.getRandomValues(new Uint8Array(16)), (byte) =>
    byte.toString(16).padStart(2, '0'),
  ).join('')
}

export class ApiError extends Error {
  readonly status: number

  constructor(
    message: string,
    status: number,
  ) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')

  let response: Response
  try {
    response = await fetch(path, { ...init, headers, credentials: 'include' })
  } catch {
    throw new Error('No fue posible conectar con el servidor. Verifica tu conexión e inténtalo de nuevo.')
  }

  if (!response.ok) {
    const body = await response.json().catch(() => undefined) as ApiErrorBody | undefined
    throw new ApiError(body?.error ?? `La solicitud falló (${response.status}).`, response.status)
  }

  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const api = {
  session: () => request<{ status: string }>('/health/live'),
  login: (credentials: { username: string; password: string }) =>
    request<{ expires_in: number }>('/auth/session', {
      method: 'POST',
      body: JSON.stringify(credentials),
    }),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  listItems: () => request<InventoryItem[]>('/items'),
  createItem: (item: InventoryItem) =>
    request<InventoryItem>('/items', {
      method: 'POST',
      headers: { 'Idempotency-Key': createIdempotencyKey() },
      body: JSON.stringify(item),
    }),
  updateItem: (sku: string, item: InventoryItem) =>
    request<InventoryItem>(`/items/${encodeURIComponent(sku)}`, {
      method: 'PUT',
      body: JSON.stringify(item),
    }),
  deleteItem: (sku: string) =>
    request<void>(`/items/${encodeURIComponent(sku)}`, { method: 'DELETE' }),
}
