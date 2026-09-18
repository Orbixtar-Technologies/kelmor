import { isSidebarToolActive } from './nav-hubs'
import type { ToolDefinition } from './types'

export interface OperatorNavGroup {
	id: string
	label: string
	tools: ToolDefinition[]
}

interface OperatorGroupMeta {
	id: string
	label: string
}

const GROUP_ORDER = [
	'account-functions',
	'account-information',
	'multi-account-functions',
	'packages',
	'resellers',
	'dns',
	'email',
	'websites',
	'files',
	'databases',
	'ssl',
	'backups',
	'transfers',
	'service-status',
	'jobs-audit',
	'security',
	'server-configuration',
	'server-contacts',
	'networking',
	'ip-functions',
	'service-configuration',
	'clusters',
	'system',
	'themes',
	'locales',
	'development',
	'support',
	'plugins',
	'addons',
	'control',
] as const

const GROUP_BY_CATEGORY: Record<string, OperatorGroupMeta> = {
	'Account Functions': { id: 'account-functions', label: 'Account Functions' },
	'Account Information': { id: 'account-information', label: 'Account Information' },
	'Multi Account Functions': { id: 'multi-account-functions', label: 'Multi Account Functions' },
	Packages: { id: 'packages', label: 'Packages' },
	Resellers: { id: 'resellers', label: 'Resellers' },
	'DNS Functions': { id: 'dns', label: 'DNS' },
	Email: { id: 'email', label: 'Email' },
	'Email Functions': { id: 'email', label: 'Email' },
	Software: { id: 'websites', label: 'Websites' },
	Files: { id: 'files', label: 'Files' },
	'SQL Services': { id: 'databases', label: 'Databases' },
	'SSL/TLS': { id: 'ssl', label: 'SSL / TLS' },
	Backup: { id: 'backups', label: 'Backups' },
	Transfers: { id: 'transfers', label: 'Transfers' },
	'Server Status': { id: 'service-status', label: 'Service / Server Status' },
	'System Health': { id: 'service-status', label: 'Service / Server Status' },
	'Restart Services': { id: 'service-status', label: 'Service / Server Status' },
	'Security Center': { id: 'security', label: 'Security' },
	'System Reboot': { id: 'security', label: 'Security' },
	'Server Configuration': { id: 'server-configuration', label: 'Server Configuration' },
	'Server Contacts': { id: 'server-contacts', label: 'Server Contacts' },
	'Networking Setup': { id: 'networking', label: 'Networking Setup' },
	'IP Functions': { id: 'ip-functions', label: 'IP Functions' },
	'Service Configuration': { id: 'service-configuration', label: 'Service Configuration' },
	Clusters: { id: 'clusters', label: 'Clusters' },
	'System Tools': { id: 'system', label: 'System Tools' },
	Themes: { id: 'themes', label: 'Themes' },
	Locales: { id: 'locales', label: 'Locales' },
	Development: { id: 'development', label: 'Development' },
	Support: { id: 'support', label: 'Support' },
	Plugins: { id: 'plugins', label: 'Plugins' },
	Market: { id: 'addons', label: 'Add-ons' },
	cPanel: { id: 'control', label: 'Kelmor Control' },
	'Kelmor Director': { id: 'home', label: 'Home' },
}

const JOBS_AUDIT: OperatorGroupMeta = { id: 'jobs-audit', label: 'Jobs & Audit' }
const JOBS_AUDIT_IDS = new Set(['jobs', 'audit', 'task-queue'])

export function operatorGroupForTool (tool: ToolDefinition): OperatorGroupMeta {
	if (JOBS_AUDIT_IDS.has(tool.id)) return JOBS_AUDIT
	return GROUP_BY_CATEGORY[tool.category] || {
		id: tool.category.toLocaleLowerCase().replace(/[^a-z0-9]+/g, '-'),
		label: tool.category,
	}
}

export function operatorNavGroups (tools: ToolDefinition[]): OperatorNavGroup[] {
	const groups = new Map<string, OperatorNavGroup>()
	for (const tool of tools) {
		if (tool.id === 'home') continue
		const meta = operatorGroupForTool(tool)
		const existing = groups.get(meta.id)
		if (existing) existing.tools.push(tool)
		else groups.set(meta.id, { ...meta, tools: [tool] })
	}
	const ordered = GROUP_ORDER
		.map((id) => groups.get(id))
		.filter((group): group is OperatorNavGroup => Boolean(group))
	for (const group of groups.values()) {
		if (!ordered.some((entry) => entry.id === group.id)) ordered.push(group)
	}
	return ordered
}

export function isOperatorGroupActive (
	group: OperatorNavGroup,
	pathname: string,
	search = '',
): boolean {
	return group.tools.some((tool) => isSidebarToolActive(tool, pathname, search))
}

export function measuredVital (available: boolean, formatted: string): string {
	return available ? formatted : 'Not reported'
}

export function failedJobDisplay (failedJobs: number | undefined): { value: string; detail: string } {
	if (typeof failedJobs !== 'number') {
		return { value: 'Not reported', detail: 'Open Jobs for history' }
	}
	return {
		value: String(failedJobs),
		detail: failedJobs === 1 ? '1 failed job' : `${failedJobs} failed jobs`,
	}
}
