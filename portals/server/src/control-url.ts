import { isLiteralIPHost, isLoopbackHost, upgradePublicHttps } from './admin-tool-url'

const VITE_DIRECTOR_PORT = '18443'
const VITE_CONTROL_PORT = '18444'
const CONTROL_HTTPS_PORT = '2083'

export interface ControlOriginHint {
	controlUrl?: string
	hostname?: string
}

export function preferredControlHostname (browserHost: string, advertised?: string): string {
	const configured = advertised?.trim() || ''
	if (configured && !isLiteralIPHost(configured) && !isLoopbackHost(configured))
		return configured
	if (browserHost && !isLiteralIPHost(browserHost) && !isLoopbackHost(browserHost))
		return browserHost
	if (configured) return configured
	return browserHost
}

export function controlPortalOrigin (
	location: Pick<Location, 'protocol' | 'hostname' | 'port'>,
	hint: ControlOriginHint = {},
): string {
	if (hint.controlUrl) {
		const advertised = advertisedControlOrigin(hint.controlUrl, location)
		if (advertised) return advertised
	}
	const hostname = preferredControlHostname(location.hostname, hint.hostname)
	if (location.port === VITE_DIRECTOR_PORT && isLoopbackHost(hostname))
		return `${location.protocol}//${hostname}:${VITE_CONTROL_PORT}`
	const protocol = isLoopbackHost(hostname) ? location.protocol : 'https:'
	return `${protocol}//${hostname}:${CONTROL_HTTPS_PORT}`
}

function advertisedControlOrigin (
	raw: string,
	location: Pick<Location, 'protocol' | 'hostname' | 'port'>,
): string {
	try {
		const parsed = new URL(upgradePublicHttps(raw) || raw)
		if (isLoopbackHost(parsed.hostname)) {
			if (location.port === VITE_DIRECTOR_PORT)
				return `${location.protocol}//${parsed.hostname}:${VITE_CONTROL_PORT}`
			return parsed.origin
		}
		parsed.protocol = 'https:'
		parsed.port = CONTROL_HTTPS_PORT
		parsed.pathname = '/'
		parsed.search = ''
		parsed.hash = ''
		return parsed.origin
	} catch {
		return ''
	}
}

export function controlImpersonationUrl (origin: string, token: string): string {
	return `${origin.replace(/\/$/, '')}/#session=${encodeURIComponent(token)}`
}

export function assignOpenedWindow (popup: Window | null, url: string) {
	if (popup && !popup.closed) {
		try { popup.opener = null } catch { /* ignore cross-origin */ }
		popup.location.replace(url)
		return
	}
	window.open(url, '_blank', 'noopener,noreferrer')
}
