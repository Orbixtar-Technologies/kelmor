export interface APIError {
	error: { code: string; message: string; request_id: string }
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

export function asList<T> (r: { items?: T[] | null } | null | undefined): T[] {
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

export async function download (path: string, filename: string): Promise<void> {
	const headers = new Headers()
	const token = getToken()
	if (token) headers.set('Authorization', `Bearer ${token}`)
	const res = await fetch(path, { headers, credentials: 'include' })
	if (!res.ok) {
		const data = await res.json().catch(() => ({})) as APIError
		throw new Error(data.error?.message || res.statusText)
	}
	const blob = await res.blob()
	const objectUrl = URL.createObjectURL(blob)
	const link = document.createElement('a')
	link.href = objectUrl
	link.download = filename
	link.click()
	URL.revokeObjectURL(objectUrl)
}
