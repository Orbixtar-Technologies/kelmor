import { canonicalAccountToolPath } from '../account-tool-routes'
import type { ResourceItem } from '../types'

export interface GroupableService {
	id: string
	label: string
}

export interface ServiceGroup<T extends GroupableService = GroupableService> {
	id: string
	label: string
	items: T[]
}

const GROUP_ORDER: Array<{ id: string; label: string; serviceIds: string[] }> = [
	{ id: 'web', label: 'Web', serviceIds: ['websites', 'domains', 'certificates', 'applications'] },
	{ id: 'email', label: 'Email', serviceIds: ['mail-domains', 'mailboxes', 'aliases', 'lists'] },
	{ id: 'data', label: 'Data', serviceIds: ['databases', 'files', 'backups'] },
	{ id: 'access', label: 'Access', serviceIds: ['ssh', 'ftp', 'tokens'] },
	{ id: 'automation', label: 'Automation', serviceIds: ['cron'] },
]

const ACTION_LABELS: Record<string, string> = {
	websites: 'Create website',
	domains: 'Add domain',
	databases: 'Create database',
	'mail-domains': 'Update routing',
	mailboxes: 'Create mailbox',
	aliases: 'Create alias',
	lists: 'Create mailing list',
	certificates: 'Request certificate',
	files: 'Write file',
	backups: 'Queue encrypted backup',
	cron: 'Add scheduled task',
	ssh: 'Add SSH key',
	ftp: 'Create FTP user',
	tokens: 'Create token',
	applications: 'Deploy application',
}

const NAME_KEYS = ['name', 'hostname', 'ascii_fqdn', 'fqdn', 'local_part', 'address', 'username', 'label', 'path', 'command']

export function groupAccountServices<T extends GroupableService> (services: T[]): Array<ServiceGroup<T>> {
	const byId = new Map(services.map((service) => [service.id, service]))
	return GROUP_ORDER.flatMap((group) => {
		const items = group.serviceIds.flatMap((id) => {
			const match = byId.get(id)
			return match ? [match] : []
		})
		return items.length ? [{ id: group.id, label: group.label, items }] : []
	})
}

export function resourcePrimaryLabel (serviceId: string, item: ResourceItem): string {
	for (const key of NAME_KEYS) {
		const value = item[key]
		if (typeof value === 'string' && value) return value
	}
	if (serviceId === 'certificates' && typeof item.hostname === 'string') return item.hostname
	return String(item.id)
}

export function serviceActionLabel (serviceId: string): string {
	return ACTION_LABELS[serviceId] || 'Create'
}

export function resourceManagePath (serviceId: string, accountId: string): string | undefined {
	return canonicalAccountToolPath(serviceId, accountId)
}
