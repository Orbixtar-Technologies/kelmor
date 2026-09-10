import type { AuditEvent } from '../types'

const ACTION_LABELS: Record<string, string> = {
	'account.create': 'Create account',
	'account.modify': 'Modify account',
	'account.suspend': 'Suspend account',
	'account.terminate': 'Terminate account',
	'server.reboot': 'Reboot host',
	'server.firewall.apply': 'Apply firewall',
	'server.service.reload': 'Reload service',
	'server.service.restart': 'Restart service',
	'server.service.start': 'Start service',
	'server.service.stop': 'Stop service',
	'server.update.check': 'Check for updates',
	'server.update.install': 'Install update',
	'databases.credentials.read': 'Read database credentials',
	'dns.record.create': 'Create DNS record',
	'dns.record.delete': 'Delete DNS record',
	'dns.zone.dnssec': 'Change DNSSEC',
}

export interface GroupedAuditEvent extends AuditEvent {
	label: string
	reason: string
	grouped: boolean
}

export function formatAuditAction (action: string): string {
	if (ACTION_LABELS[action]) return ACTION_LABELS[action]
	const base = action.replace(/\.intent$/, '')
	if (ACTION_LABELS[base]) return ACTION_LABELS[base]
	return action.replaceAll('.', ' ')
}

export function auditFailureReason (event: AuditEvent): string {
	const metadata = event.metadata || {}
	const reason = metadata.error || metadata.message || metadata.reason || metadata.detail
	if (typeof reason === 'string' && reason) return reason
	if (event.success) return ''
	return 'Rejected or failed. Inspect for the recorded reason.'
}

export function maskSourceIp (ip?: string): string {
	if (!ip) return ''
	const parts = ip.split('.')
	if (parts.length === 4) return `${parts[0]}.${parts[1]}.*.*`
	if (ip.includes(':')) return ip.split(':').slice(0, 2).join(':') + ':*'
	return ip
}

export function auditActorLabel (event: AuditEvent): string {
	if (event.actor_id && event.actor_id.length > 20) return event.actor_type || 'operator'
	return event.actor_id || event.actor_type || 'system'
}

export function groupAuditEvents (events: AuditEvent[]): GroupedAuditEvent[] {
	const byRequest = new Map<string, AuditEvent[]>()
	for (const event of events) {
		const key = event.request_id || event.id
		const group = byRequest.get(key) || []
		group.push(event)
		byRequest.set(key, group)
	}
	const grouped: GroupedAuditEvent[] = []
	for (const group of byRequest.values()) {
		const sorted = [...group].sort((left, right) => left.occurred_at.localeCompare(right.occurred_at))
		const outcome = sorted.find((event) => !event.action.endsWith('.intent')) || sorted[sorted.length - 1]
		grouped.push({
			...outcome,
			label: formatAuditAction(outcome.action),
			reason: auditFailureReason(outcome),
			grouped: sorted.length > 1,
		})
	}
	return grouped.sort((left, right) => right.occurred_at.localeCompare(left.occurred_at))
}
