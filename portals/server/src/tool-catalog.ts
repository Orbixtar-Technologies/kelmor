import type { ToolDefinition } from './types'

export const toolCatalog: ToolDefinition[] = [
	{ id: 'home', label: 'Home', description: 'Host overview, favorites, status, and recent activity', category: 'Kelmor Director', path: '/', icon: 'home', capabilities: ['server.read', 'accounts.read'] },
	{ id: 'accounts', label: 'List Accounts', description: 'Search, sort, suspend, and manage accounts', category: 'Account Information', path: '/accounts', icon: 'users', capabilities: ['accounts.read'] },
	{ id: 'account-summary', label: 'Account Summary', description: 'Open an account hub for status, usage, and linked services', category: 'Account Information', path: '/accounts?task=summary', icon: 'account', capabilities: ['accounts.read'] },
	{ id: 'suspended', label: 'Suspended Accounts', description: 'Review suspended hosting identities', category: 'Account Information', path: '/accounts?view=suspended', icon: 'pause', capabilities: ['accounts.read'] },
	{ id: 'over-quota', label: 'Over Quota Accounts', description: 'Find accounts over package disk or bandwidth quota', category: 'Account Information', path: '/accounts?view=over-quota', icon: 'meter', capabilities: ['accounts.read', 'billing.usage.read'], matchAll: true },
	{ id: 'create-account', label: 'Create Account', description: 'Provision a Kelmor account using a reviewed wizard', category: 'Account Functions', path: '/accounts/create', icon: 'plus', capabilities: ['accounts.create'] },
	{ id: 'modify-account', label: 'Modify an Account', description: 'Change domain, address, ownership, and login access', category: 'Account Functions', path: '/accounts?task=modify', icon: 'edit', capabilities: ['accounts.modify'] },
	{ id: 'change-package', label: 'Change Account Package', description: 'Move an account to a different resource package', category: 'Account Functions', path: '/accounts?task=package', icon: 'box', capabilities: ['accounts.modify'] },
	{ id: 'suspend-account', label: 'Suspend or Unsuspend', description: 'Change account availability with a queued operation', category: 'Account Functions', path: '/accounts?task=suspension', icon: 'pause', capabilities: ['accounts.suspend'] },
	{ id: 'terminate-account', label: 'Terminate an Account', description: 'Review and permanently remove an account', category: 'Account Functions', path: '/accounts?task=terminate', icon: 'trash', capabilities: ['accounts.terminate'] },
	{ id: 'force-password', label: 'Force Password Change', description: 'Rotate owner credentials and require a change at sign-in', category: 'Account Functions', path: '/accounts?task=password', icon: 'key', capabilities: ['accounts.modify'] },
	{ id: 'packages', label: 'Packages', description: 'Manage reusable account limits and assignments', category: 'Packages', path: '/packages', icon: 'box', capabilities: ['packages.read'] },
	{ id: 'resellers', label: 'Resellers', description: 'Manage delegated operators and privileges', category: 'Resellers', path: '/resellers', icon: 'briefcase', capabilities: ['resellers.read'] },
	{ id: 'dns', label: 'DNS Management', description: 'Select an account and edit zones, records, and DNSSEC', category: 'DNS Functions', path: '/dns', icon: 'globe', capabilities: ['accounts.read', 'dns.read'], matchAll: true },
	{ id: 'sql', label: 'SQL Services', description: 'Select an account to manage databases and users', category: 'SQL Services', path: '/accounts?task=databases', icon: 'database', capabilities: ['accounts.read', 'databases.read'], matchAll: true },
	{ id: 'email', label: 'Email Services', description: 'Select an account to manage domains, mailboxes, and aliases', category: 'Email Functions', path: '/accounts?task=email', icon: 'mail', capabilities: ['accounts.read', 'mail.read'], matchAll: true },
	{ id: 'ssl', label: 'SSL Certificates', description: 'Select an account to inspect and request certificates', category: 'SSL/TLS', path: '/accounts?task=certificates', icon: 'lock', capabilities: ['accounts.read', 'websites.read'], matchAll: true },
	{ id: 'services', label: 'Service Status', description: 'Inspect server health, services, vitals, and processes', category: 'Server Status', path: '/status', icon: 'pulse', capabilities: ['server.read'] },
	{ id: 'security', label: 'Security & Host Configuration', description: 'Audit, firewall configuration, and host reboot', category: 'Security Center', path: '/security', icon: 'shield', capabilities: ['server.read'] },
	{ id: 'transfers', label: 'Transfers & Backups', description: 'Native transfer, extracted archive import, backup, and restore', category: 'Transfers', path: '/transfers', icon: 'transfer', capabilities: ['accounts.create', 'backups.create'] },
	{ id: 'jobs', label: 'Jobs', description: 'Inspect and retry background operations', category: 'System Tools', path: '/jobs', icon: 'jobs', capabilities: ['server.read', 'accounts.read'] },
	{ id: 'audit', label: 'Audit Trail', description: 'Search privileged actions and before/after state', category: 'Security Center', path: '/audit', icon: 'audit', capabilities: ['security.audit.read'] },
	{ id: 'usage', label: 'Account Usage', description: 'Compare disk, bandwidth, process, and memory usage', category: 'Account Information', path: '/usage', icon: 'chart', capabilities: ['billing.usage.read'] },
]

export function discoverTools (tools: ToolDefinition[], capabilities: Record<string, boolean>): ToolDefinition[] {
	return tools.filter((tool) => {
		if (tool.matchAll) return tool.capabilities.every((capability) => capabilities[capability])
		return tool.capabilities.some((capability) => capabilities[capability])
	})
}

export function groupTools (tools: ToolDefinition[]): Map<string, ToolDefinition[]> {
	const groups = new Map<string, ToolDefinition[]>()
	for (const tool of tools) groups.set(tool.category, [...(groups.get(tool.category) ?? []), tool])
	return groups
}

export function accountTaskTarget (task: string, accountId: string): string {
	const serviceByTask: Record<string, string> = {
		databases: 'databases',
		email: 'mailboxes',
		certificates: 'certificates',
	}
	const service = serviceByTask[task]
	if (service) return `/accounts/${accountId}/services?service=${service}`
	if (['password', 'terminate', 'package', 'modify', 'suspension', 'summary'].includes(task)) {
		return `/accounts/${accountId}?task=${task}`
	}
	return `/accounts/${accountId}`
}
