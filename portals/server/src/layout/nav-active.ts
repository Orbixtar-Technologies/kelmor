import type { ToolDefinition } from '../types'
import { whmFeatures } from '../whm-catalog'

const ACCOUNT_CATEGORIES = new Set([
	'Account Information',
	'Account Functions',
	'Packages',
	'Resellers',
	'DNS Functions',
	'Files',
	'SQL Services',
	'Email Functions',
	'Email',
	'SSL/TLS',
	'Software',
	'Backup',
	'Transfers',
	'Multi Account Functions',
	'cPanel',
])

const DISTINGUISH_KEYS = ['view', 'task', 'tab', 'q', 'mode'] as const

const PATH_DEFAULTS: Record<string, Partial<Record<typeof DISTINGUISH_KEYS[number], string>>> = {
	'/ssl': { task: 'inventory' },
	'/email': { tab: 'mailboxes' },
}

function currentParams (search: string): URLSearchParams {
	return new URLSearchParams(search.startsWith('?') ? search : search ? `?${search}` : '')
}

function paramOrDefault (
	pathname: string,
	params: URLSearchParams,
	key: typeof DISTINGUISH_KEYS[number],
): string {
	return params.get(key) || PATH_DEFAULTS[pathname]?.[key] || ''
}

function siblingClaims (
	toolId: string,
	pathname: string,
	key: typeof DISTINGUISH_KEYS[number],
	value: string,
): boolean {
	if (!value) return false
	return whmFeatures.some((feature) => {
		if (feature.id === toolId) return false
		const other = new URL(feature.path, 'https://director.local')
		return other.pathname === pathname && other.searchParams.get(key) === value
	})
}

export function isDirectorToolActive (tool: ToolDefinition, pathname: string, search = ''): boolean {
	const target = new URL(tool.path, 'https://director.local')
	const params = currentParams(search)

	if (target.pathname === '/') return pathname === '/'

	if (tool.id === 'create-account') return pathname === '/accounts/create'

	if (tool.id === 'account-summary') {
		return pathname.startsWith('/accounts/') && pathname !== '/accounts/create'
	}

	if (pathname.startsWith('/section/')) {
		return params.get('tool') === tool.id
	}

	if (target.pathname.startsWith('/tools/')) {
		return pathname === target.pathname
	}

	if (pathname !== target.pathname) return false

	for (const key of DISTINGUISH_KEYS) {
		const wanted = target.searchParams.get(key) || ''
		const current = paramOrDefault(pathname, params, key)
		if (wanted) {
			if (current !== wanted) return false
			continue
		}
		if (!current) continue
		if (current === (PATH_DEFAULTS[pathname]?.[key] || '')) continue
		if (siblingClaims(tool.id, pathname, key, current)) return false
	}
	return true
}

export function navScopeForCategory (category: string): 'account' | 'host' {
	return ACCOUNT_CATEGORIES.has(category) ? 'account' : 'host'
}
