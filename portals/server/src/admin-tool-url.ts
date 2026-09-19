export type AdminToolKind = 'webmail' | 'phpmyadmin'

export function isLoopbackHost (hostname: string): boolean {
	const host = hostname.replace(/^\[|\]$/g, '').toLocaleLowerCase()
	return host === 'localhost' || host === '127.0.0.1' || host === '::1'
}

export function isLiteralIPHost (hostname: string): boolean {
	const host = hostname.replace(/^\[|\]$/g, '')
	if (/^\d{1,3}(?:\.\d{1,3}){3}$/.test(host)) return true
	return host.includes(':')
}

export function upgradePublicHttps (url: string): string {
	const trimmed = url.trim()
	if (!trimmed) return ''
	try {
		const parsed = new URL(trimmed)
		if (parsed.protocol === 'http:' && !isLoopbackHost(parsed.hostname))
			parsed.protocol = 'https:'
		return parsed.href
	} catch {
		return trimmed
	}
}

export function adminToolUrl (
	kind: AdminToolKind,
	fetched?: string,
	domain?: string,
): string {
	if (fetched) {
		const upgraded = upgradePublicHttps(fetched)
		if (upgraded && !upgraded.includes('<domain>')) return upgraded
	}
	const host = domain?.trim()
	if (!host) return ''
	return `https://${kind}.${host}/`
}
