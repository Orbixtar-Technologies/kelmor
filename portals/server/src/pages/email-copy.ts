import { valueOf } from '../helpers'
import type { ResourceItem } from '../types'

export function mailDomainLabel (item: ResourceItem) {
	const fqdn = valueOf(item, 'ascii_fqdn')
	if (fqdn !== '—') return fqdn
	const name = valueOf(item, 'fqdn')
	if (name !== '—') return name
	return String(item.id || '')
}

export function mailboxAddress (mailbox: ResourceItem, domains: ResourceItem[], fallbackDomain: string) {
	const domainId = String(mailbox.domain_id || '')
	const domain = domains.find((entry) => entry.id === domainId)
	if (!domain) return `${valueOf(mailbox, 'local_part')}@${fallbackDomain}`
	const host = mailDomainLabel(domain) || fallbackDomain
	return `${valueOf(mailbox, 'local_part')}@${host}`
}
