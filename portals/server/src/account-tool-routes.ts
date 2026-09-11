const HUB_SERVICES = [
	'websites',
	'domains',
	'certificates',
	'databases',
	'mailboxes',
	'mail-domains',
	'aliases',
	'files',
	'cron',
	'ftp',
] as const

export type HubAccountService = (typeof HUB_SERVICES)[number]

export function isHubAccountService (serviceId: string): serviceId is HubAccountService {
	return (HUB_SERVICES as readonly string[]).includes(serviceId)
}

export function canonicalAccountToolPath (serviceId: string, accountId: string): string | undefined {
	switch (serviceId) {
		case 'websites':
			return `/websites?account=${accountId}`
		case 'domains':
			return `/domains?account=${accountId}`
		case 'certificates':
			return `/ssl?account=${accountId}`
		case 'databases':
			return `/sql?account=${accountId}`
		case 'mailboxes':
			return `/email?account=${accountId}&tab=mailboxes`
		case 'mail-domains':
			return `/email?account=${accountId}&tab=domains`
		case 'aliases':
			return `/email?account=${accountId}&tab=aliases`
		case 'files':
			return `/files?account=${accountId}`
		case 'cron':
			return `/cron?account=${accountId}`
		case 'ftp':
			return `/ftp?account=${accountId}`
		case 'dns':
			return `/dns?account=${accountId}`
		case 'deliverability':
			return `/deliverability?account=${accountId}`
		case 'backups':
			return `/accounts/${accountId}/services?service=backups`
		case 'ssh':
			return `/accounts/${accountId}/services?service=ssh`
		case 'tokens':
			return `/accounts/${accountId}/services?service=tokens`
		case 'applications':
			return `/accounts/${accountId}/services?service=applications`
		default:
			return undefined
	}
}

export function sslToolPath (accountId: string, task: 'inventory' | 'request' | 'status' | 'autossl' | 'service' = 'inventory') {
	const params = new URLSearchParams({ account: accountId, task })
	return `/ssl?${params}`
}
