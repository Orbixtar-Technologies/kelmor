export interface FindFunction {
	label: string
	to: string
	group: string
	keywords: string[]
	cap: string
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
		label: 'Host operations',
		to: '/',
		group: 'Host/Service Status',
		keywords: ['dashboard', 'host', 'service status', 'metrics'],
		cap: 'server.read',
	},
	{
		label: 'List Accounts',
		to: '/accounts',
		group: 'Account Functions',
		keywords: ['list accounts', 'customers', 'tenants'],
		cap: 'accounts.read',
	},
	{
		label: 'Create Account',
		to: '/accounts/create',
		group: 'Account Functions',
		keywords: ['create account', 'provision', 'new account'],
		cap: 'accounts.create',
	},
	{
		label: 'Packages',
		to: '/packages',
		group: 'Packages',
		keywords: ['packages', 'limits', 'quota'],
		cap: 'packages.read',
	},
	{
		label: 'Jobs',
		to: '/jobs',
		group: 'Jobs/Audit',
		keywords: ['jobs', 'queue', 'failed jobs'],
		cap: 'server.read',
	},
	{
		label: 'Audit',
		to: '/audit',
		group: 'Jobs/Audit',
		keywords: ['audit', 'trail', 'security'],
		cap: 'security.audit.read',
	},
	{
		label: 'Import',
		to: '/import',
		group: 'Import',
		keywords: ['import', 'cpmove', 'native export'],
		cap: 'accounts.create',
	},
	{
		label: 'Usage',
		to: '/monitor',
		group: 'Usage',
		keywords: ['usage', 'disk', 'bandwidth'],
		cap: 'billing.usage.read',
	},
	{
		label: 'Resellers',
		to: '/resellers',
		group: 'Resellers',
		keywords: ['resellers', 'delegate'],
		cap: 'resellers.read',
	},
]

export const NAV_GROUPS = [
	'Account Functions',
	'Packages',
	'Host/Service Status',
	'Jobs/Audit',
	'Import',
	'Usage',
	'Resellers',
] as const

export function visibleFunctions (
	caps: Record<string, boolean>,
): FindFunction[] {
	return DIRECTOR_FUNCTIONS.filter((fn) => {
		if (fn.to === '/jobs')
			return !!(caps['server.read'] || caps['accounts.read'])
		return !!caps[fn.cap]
	})
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
