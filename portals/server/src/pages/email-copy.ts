import { valueOf } from '../helpers'
import type { ResourceItem } from '../types'

export function mailboxAddress (mailbox: ResourceItem, domains: ResourceItem[], fallbackDomain: string) {
	const domainId = String(mailbox.domain_id || '')
	const domain = domains.find((entry) => entry.id === domainId)
	if (!domain) return `${valueOf(mailbox, 'local_part')}@${fallbackDomain}`
	const fqdn = valueOf(domain, 'ascii_fqdn')
	const named = fqdn !== '—' ? fqdn : valueOf(domain, 'domain_id')
	const host = named !== '—' ? named : fallbackDomain
	return `${valueOf(mailbox, 'local_part')}@${host}`
}
