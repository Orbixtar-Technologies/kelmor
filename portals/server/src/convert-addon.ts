import type { ResourceItem } from './types'

export function domainFqdn (item: ResourceItem) {
	const ascii = item.ascii_fqdn
	if (typeof ascii === 'string' && ascii) return ascii
	const fqdn = item.fqdn
	if (typeof fqdn === 'string' && fqdn) return fqdn
	return ''
}

export function addonDomainsFrom (items: ResourceItem[]) {
	return items.filter((item) => String(item.type || '') === 'addon')
}

export function convertAddonDomainsHref (accountId: string) {
	const params = new URLSearchParams({ account: accountId, view: 'addon' })
	return `/domains?${params.toString()}`
}
