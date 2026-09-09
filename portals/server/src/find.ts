export interface FindFunction {
	label: string
	to: string
	group: string
	keywords: string[]
	cap: string
	anyCaps?: string[]
	end?: boolean
}

export interface FindAccount {
	id: string
	username: string
	primary_domain: string
}

export interface FindHit {
	kind: 'function' | 'account'
	label: string
	detail: string
	to: string
}

export const DIRECTOR_FUNCTIONS: FindFunction[] = [
	{
		label: 'List Accounts',
		to: '/accounts',
		group: 'Account Functions',
		keywords: ['list accounts', 'customers', 'tenants'],
		cap: 'accounts.read',
		end: true,
	},
	{
		label: 'Create Account',
		to: '/accounts/create',
		group: 'Account Functions',
		keywords: ['create account', 'provision', 'new account'],
		cap: 'accounts.create',
	},
	{
		label: 'Suspend / Unsuspend',
		to: '/accounts/suspend',
		group: 'Account Functions',
		keywords: ['suspend', 'unsuspend', 'disable account'],
		cap: 'accounts.suspend',
	},
	{
		label: 'Terminate',
		to: '/accounts/terminate',
		group: 'Account Functions',
		keywords: ['terminate', 'remove account'],
		cap: 'accounts.terminate',
	},
	{
		label: 'Change Package',
		to: '/accounts/package',
		group: 'Account Functions',
		keywords: ['change package', 'upgrade', 'downgrade'],
		cap: 'accounts.modify',
	},
	{
		label: 'Modify Account',
		to: '/accounts/modify',
		group: 'Account Functions',
		keywords: ['modify', 'edit account', 'primary domain'],
		cap: 'accounts.modify',
	},
	{
		label: 'Account Summary',
		to: '/accounts/summary',
		group: 'Account Functions',
		keywords: ['account summary', 'overview'],
		cap: 'accounts.read',
	},
	{
		label: 'Resellers',
		to: '/resellers',
		group: 'Resellers',
		keywords: ['resellers', 'delegate'],
		cap: 'resellers.read',
	},
	{
		label: 'Packages',
		to: '/packages',
		group: 'Packages',
		keywords: ['packages', 'limits', 'quota'],
		cap: 'packages.read',
	},
	{
		label: 'DNS / Domains',
		to: '/domains',
		group: 'DNS/Domains',
		keywords: ['dns', 'domains', 'zones'],
		cap: 'accounts.read',
		anyCaps: ['accounts.read', 'domains.read', 'dns.read'],
	},
	{
		label: 'Host operations',
		to: '/',
		group: 'Service Status/Host',
		keywords: [
			'dashboard', 'host', 'service status', 'metrics', 'agent',
			'privileged host actions', 'firewall', 'reboot',
		],
		cap: 'server.read',
		end: true,
	},
	{
		label: 'Jobs',
		to: '/jobs',
		group: 'Jobs & Audit',
		keywords: ['jobs', 'queue', 'failed jobs'],
		cap: 'server.read',
		anyCaps: ['server.read', 'accounts.read'],
	},
	{
		label: 'Audit',
		to: '/audit',
		group: 'Jobs & Audit',
		keywords: ['audit', 'trail', 'security'],
		cap: 'security.audit.read',
	},
	{
		label: 'Import / Migration',
		to: '/import',
		group: 'Import/Migration',
		keywords: ['import', 'cpmove', 'native export', 'migrate'],
		cap: 'accounts.create',
	},
	{
		label: 'Usage / Quotas',
		to: '/monitor',
		group: 'Usage/Quotas',
		keywords: ['usage', 'disk', 'bandwidth', 'quota'],
		cap: 'billing.usage.read',
	},
]

export const NAV_GROUPS = [
	'Account Functions',
	'Resellers',
	'Packages',
	'DNS/Domains',
	'Service Status/Host',
	'Jobs & Audit',
	'Import/Migration',
	'Usage/Quotas',
] as const

export function functionAllowed (
	fn: FindFunction,
	caps: Record<string, boolean>,
) {
	if (fn.anyCaps && fn.anyCaps.length)
		return fn.anyCaps.some((cap) => !!caps[cap])
	return !!caps[fn.cap]
}

export function visibleFunctions (
	caps: Record<string, boolean>,
): FindFunction[] {
	return DIRECTOR_FUNCTIONS.filter((fn) => functionAllowed(fn, caps))
}

export function matchFind (
	query: string,
	functions: FindFunction[],
	accounts: FindAccount[],
): FindHit[] {
	const q = query.trim().toLowerCase()
	if (!q) return []
	const hits: FindHit[] = []
	for (const fn of functions) {
		const hay = [fn.label, fn.group, ...fn.keywords]
			.join(' ')
			.toLowerCase()
		if (!hay.includes(q)) continue
		hits.push({
			kind: 'function',
			label: fn.label,
			detail: fn.group,
			to: fn.to,
		})
	}
	for (const acc of accounts) {
		const hay = `${acc.username} ${acc.primary_domain}`.toLowerCase()
		if (!hay.includes(q)) continue
		hits.push({
			kind: 'account',
			label: acc.username,
			detail: acc.primary_domain,
			to: `/accounts/${acc.id}`,
		})
	}
	return hits
}
