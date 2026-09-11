export function controlPortalOrigin (location: Pick<Location, 'protocol' | 'hostname' | 'port'>): string {
	const { protocol, hostname, port } = location
	if (port === '18443') return `${protocol}//${hostname}:18444`
	if (port === '8443' || port === '' || port === '80' || port === '443') return `${protocol}//${hostname}:8444`
	return `${protocol}//${hostname}:8444`
}

export function controlImpersonationUrl (origin: string, token: string): string {
	return `${origin.replace(/\/$/, '')}/#session=${encodeURIComponent(token)}`
}
