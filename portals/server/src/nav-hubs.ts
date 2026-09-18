import { isDirectorToolActive } from './layout/nav-active'
import type { ToolDefinition } from './types'
import { featureById, toToolDefinition, whmFeatures, type WhmFeature } from './whm-catalog'

export interface NavHub {
	id: string
	label: string
	description: string
	icon: string
	scope: 'account' | 'host'
	categories: string[]
	defaultToolId: string
}

export const navHubs: NavHub[] = [
	{
		id: 'home',
		label: 'Home',
		description: 'Host overview and shortcuts.',
		icon: 'home',
		scope: 'host',
		categories: ['Kelmor Director'],
		defaultToolId: 'home',
	},
	{
		id: 'accounts',
		label: 'Accounts',
		description: 'Provisioning, inventory, quotas, and multi-account actions.',
		icon: 'users',
		scope: 'account',
		categories: ['Account Information', 'Account Functions', 'Multi Account Functions'],
		defaultToolId: 'list-accounts',
	},
	{
		id: 'packages',
		label: 'Packages & Resellers',
		description: 'Resource packages, feature sets, and reseller privileges.',
		icon: 'box',
		scope: 'account',
		categories: ['Packages', 'Resellers'],
		defaultToolId: 'packages',
	},
	{
		id: 'dns',
		label: 'DNS',
		description: 'Zones, templates, forwarding, and cluster DNS policy.',
		icon: 'globe',
		scope: 'account',
		categories: ['DNS Functions'],
		defaultToolId: 'dns',
	},
	{
		id: 'email',
		label: 'Email',
		description: 'Mailboxes, filters, deliverability, and mail queue.',
		icon: 'mail',
		scope: 'account',
		categories: ['Email'],
		defaultToolId: 'email',
	},
	{
		id: 'websites',
		label: 'Websites',
		description: 'PHP runtimes, nginx policy, and application installers.',
		icon: 'code',
		scope: 'account',
		categories: ['Software'],
		defaultToolId: 'websites',
	},
	{
		id: 'files',
		label: 'Files',
		description: 'File manager and virtual FTP accounts.',
		icon: 'files',
		scope: 'account',
		categories: ['Files'],
		defaultToolId: 'file-manager',
	},
	{
		id: 'sql',
		label: 'Databases',
		description: 'MariaDB, PostgreSQL, phpMyAdmin, and SQL host settings.',
		icon: 'database',
		scope: 'account',
		categories: ['SQL Services'],
		defaultToolId: 'sql',
	},
	{
		id: 'ssl',
		label: 'SSL / TLS',
		description: 'Certificates, AutoSSL, and service TLS.',
		icon: 'lock',
		scope: 'account',
		categories: ['SSL/TLS'],
		defaultToolId: 'ssl',
	},
	{
		id: 'backups',
		label: 'Backups',
		description: 'Backup policy, transfers, and restores.',
		icon: 'transfer',
		scope: 'account',
		categories: ['Backup', 'Transfers'],
		defaultToolId: 'transfers',
	},
	{
		id: 'server',
		label: 'Server Configuration',
		description: 'Host, network, IP, service, contact, and cluster settings.',
		icon: 'edit',
		scope: 'host',
		categories: [
			'Server Configuration',
			'Server Contacts',
			'Networking Setup',
			'IP Functions',
			'Service Configuration',
			'Clusters',
		],
		defaultToolId: 'basic-setup',
	},
	{
		id: 'security',
		label: 'Security',
		description: 'Hardening, access policy, audit, and host reboot.',
		icon: 'shield',
		scope: 'host',
		categories: ['Security Center', 'System Reboot'],
		defaultToolId: 'security',
	},
	{
		id: 'status',
		label: 'Server Status',
		description: 'Vitals, processes, and service restarts.',
		icon: 'pulse',
		scope: 'host',
		categories: ['Server Status', 'System Health', 'Restart Services'],
		defaultToolId: 'services',
	},
	{
		id: 'system',
		label: 'System',
		description: 'Jobs, updates, themes, plugins, and support tools.',
		icon: 'jobs',
		scope: 'host',
		categories: [
			'System Tools',
			'Themes',
			'Locales',
			'Development',
			'Support',
			'Plugins',
			'Market',
			'cPanel',
		],
		defaultToolId: 'jobs',
	},
]

const hubByIdMap = new Map(navHubs.map((hub) => [hub.id, hub]))
const hubByCategory = new Map<string, NavHub>()
for (const hub of navHubs) {
	for (const category of hub.categories) hubByCategory.set(category, hub)
}

export function hubById (id: string): NavHub | undefined {
	return hubByIdMap.get(id)
}

export function hubForCategory (category: string): NavHub | undefined {
	return hubByCategory.get(category)
}

export function hubForToolId (toolId: string): NavHub | undefined {
	const feature = featureById(toolId)
	return feature ? hubForCategory(feature.category) : undefined
}

export function isCatalogToolPath (path: string): boolean {
	return path.startsWith('/tools/')
}

export function usesDedicatedManager (feature: WhmFeature): boolean {
	return Boolean(feature.dedicated && !isCatalogToolPath(feature.path))
}

export function hubEntryPath (hub: NavHub): string {
	if (hub.id === 'home') return '/'
	const feature = featureById(hub.defaultToolId)
	if (feature && usesDedicatedManager(feature)) {
		const target = new URL(feature.path, 'https://director.local')
		return `${target.pathname}${target.search}`
	}
	return `/section/${hub.id}`
}

export function hubToolHref (hub: NavHub, feature: WhmFeature): string {
	if (hub.id === 'home') return '/'
	if (usesDedicatedManager(feature)) {
		const target = new URL(feature.path, 'https://director.local')
		return `${target.pathname}${target.search}`
	}
	return `/section/${hub.id}?tool=${encodeURIComponent(feature.id)}`
}

export function hrefForFeature (feature: WhmFeature): string {
	const hub = hubForCategory(feature.category)
	return hub ? hubToolHref(hub, feature) : feature.path
}

export function toolsForHub (hub: NavHub, tools: ToolDefinition[]): ToolDefinition[] {
	const categories = new Set(hub.categories)
	return tools.filter((tool) => categories.has(tool.category) && tool.id !== 'home')
}

export function visibleHubs (tools: ToolDefinition[]): NavHub[] {
	const categories = new Set(tools.map((tool) => tool.category))
	return navHubs.filter((hub) => hub.categories.some((category) => categories.has(category)))
}

export function featureMatchScore (feature: WhmFeature, pathname: string, search = ''): number {
	if (!isDirectorToolActive(toToolDefinition(feature), pathname, search)) return -1
	const target = new URL(feature.path, 'https://director.local')
	const current = new URLSearchParams(search.startsWith('?') ? search : search ? `?${search}` : '')
	let score = target.pathname.length
	for (const [key, value] of target.searchParams) {
		if (current.get(key) === value) score += 80
		else score -= 40
	}
	return score
}

export function hubForLocation (pathname: string, search = ''): NavHub | undefined {
	if (pathname === '/') return hubById('home')
	const sectionId = pathname.match(/^\/section\/([^/]+)/)?.[1]
	if (sectionId) return hubById(sectionId)
	if (pathname.startsWith('/tools/')) {
		return hubForToolId(pathname.split('/')[2] || '')
	}
	if (pathname.startsWith('/accounts/') && pathname !== '/accounts/create') {
		return hubById('accounts')
	}

	const matches = whmFeatures
		.map((feature) => ({ feature, score: featureMatchScore(feature, pathname, search) }))
		.filter((entry) => entry.score >= 0)
		.sort((left, right) => {
			if (right.score !== left.score) return right.score - left.score
			return hubIndex(left.feature) - hubIndex(right.feature)
		})
	if (!matches.length) return undefined
	return hubForToolId(matches[0].feature.id)
}

export function isDirectorHubActive (hub: NavHub, pathname: string, search = ''): boolean {
	return hubForLocation(pathname, search)?.id === hub.id
}

export function isAccountDetailPath (pathname: string): boolean {
	return /^\/accounts\/(?!create(?:\/|$))[^/]+/.test(pathname)
}

function hubIndex (feature: WhmFeature): number {
	const hub = hubForCategory(feature.category)
	return hub ? navHubs.findIndex((entry) => entry.id === hub.id) : navHubs.length
}
