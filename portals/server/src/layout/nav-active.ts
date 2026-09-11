import type { ToolDefinition } from '../types'

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

export function isDirectorToolActive (tool: ToolDefinition, pathname: string, search = ''): boolean {
	const target = new URL(tool.path, 'https://director.local')
	const params = new URLSearchParams(search.startsWith('?') ? search : search ? `?${search}` : '')
	const view = params.get('view')
	const task = params.get('task')

	if (target.pathname === '/') return pathname === '/'

	if (tool.id === 'create-account') return pathname === '/accounts/create'

	if (tool.id === 'account-summary') {
		return pathname.startsWith('/accounts/') && pathname !== '/accounts/create'
	}

	if (target.pathname === '/accounts') {
		if (pathname !== '/accounts') return false
		if (target.searchParams.has('view')) return view === target.searchParams.get('view')
		if (target.searchParams.has('task')) return task === target.searchParams.get('task')
		return !view && !task
	}

	if (target.pathname === '/domains') {
		if (pathname !== '/domains') return false
		if (target.searchParams.has('view')) return view === target.searchParams.get('view')
		return !view
	}

	if (target.pathname.startsWith('/tools/')) {
		return pathname === target.pathname
	}

	return pathname === target.pathname || pathname.startsWith(`${target.pathname}/`)
}

export function navScopeForCategory (category: string): 'account' | 'host' {
	return ACCOUNT_CATEGORIES.has(category) ? 'account' : 'host'
}
