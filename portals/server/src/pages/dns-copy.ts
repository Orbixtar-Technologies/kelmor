export function zoneSyncLabel (zone: { [key: string]: unknown }): string {
	const desired = Number(zone.desired_revision || 0)
	const observed = Number(zone.observed_revision || 0)
	if (observed < desired) return 'Zone sync pending'
	return 'Zone in sync'
}

export function dnssecStateLabel (enabled: unknown): string {
	return enabled ? 'DNSSEC enabled' : 'DNSSEC disabled'
}

export function validateDnsRecord (type: string, content: string, priority?: number): string {
	const value = content.trim()
	if (!value) return 'Content is required.'
	if (type === 'A' && !/^\d{1,3}(\.\d{1,3}){3}$/.test(value)) return 'A records need an IPv4 address.'
	if (type === 'AAAA' && !value.includes(':')) return 'AAAA records need an IPv6 address.'
	if (type === 'MX' && (priority === undefined || Number.isNaN(priority))) return 'MX records need a priority.'
	return ''
}

export function describeDnsChange (action: 'add' | 'replace' | 'delete', record: { name: string; type: string; content: string }): string {
	if (action === 'delete') return `Delete ${record.type} ${record.name} → ${record.content}`
	if (action === 'replace') return `Replace ${record.type} ${record.name} with ${record.content}`
	return `Add ${record.type} ${record.name} → ${record.content}`
}
