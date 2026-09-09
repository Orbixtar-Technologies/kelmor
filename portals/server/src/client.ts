export interface APIError {
	error: { code: string; message: string; request_id: string }
}

export interface ListResponse<T> {
	items?: T[] | null
	total?: number
	offset?: number
	limit?: number
}

const tokenKey = 'server_portal_token'

export function setToken (token: string) {
	localStorage.setItem(tokenKey, token)
}
export function clearToken () {
	localStorage.removeItem(tokenKey)
}
export function getToken () {
	return localStorage.getItem(tokenKey) || ''
}

export function asList<T> (r: ListResponse<T> | null | undefined): T[] {
	if (!r || !Array.isArray(r.items)) return []
	return r.items
}

export async function api<T> (path: string, init: RequestInit = {}): Promise<T> {
	const headers = new Headers(init.headers)
	if (!headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
	const token = getToken()
	if (token) headers.set('Authorization', `Bearer ${token}`)
	const res = await fetch(path, { ...init, headers, credentials: 'include' })
	const data = await res.json().catch(() => ({}))
	if (!res.ok) {
		const err = data as APIError
		throw new Error(err.error?.message || res.statusText)
	}
	return data as T
}

export function post<T> (path: string, body: unknown = {}, headers?: HeadersInit): Promise<T> {
	return api<T>(path, { method: 'POST', body: JSON.stringify(body), headers })
}

export function patch<T> (path: string, body: unknown): Promise<T> {
	return api<T>(path, { method: 'PATCH', body: JSON.stringify(body) })
}

export function put<T> (path: string, body: unknown): Promise<T> {
	return api<T>(path, { method: 'PUT', body: JSON.stringify(body) })
}

export function del<T> (path: string): Promise<T> {
	return api<T>(path, { method: 'DELETE' })
}

/** Writes that queue a job need an idempotency key so retries do not double-run. */
export function idempotent (): HeadersInit {
	return { 'Idempotency-Key': crypto.randomUUID() }
}

/** Builds `?a=1&b=2`, dropping empty values so filters stay out of the URL. */
export function query (params: Record<string, string | number | boolean | undefined | null>) {
	const qs = new URLSearchParams()
	for (const [key, value] of Object.entries(params)) {
		if (value === undefined || value === null || value === '') continue
		qs.set(key, String(value))
	}
	const s = qs.toString()
	return s ? `?${s}` : ''
}

export function listOf<T> (path: string): Promise<T[]> {
	return api<ListResponse<T>>(path).then(asList)
}
