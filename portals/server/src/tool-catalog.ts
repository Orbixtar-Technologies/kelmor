import type { ToolDefinition } from './types'

export const toolCatalog: ToolDefinition[] = [
	{ id: 'home', label: 'Home', description: 'Host overview, favorites, status, and recent activity', category: 'Kelmor Director', path: '/', icon: 'home', capabilities: ['server.read', 'accounts.read'] },
	{ id: 'accounts', label: 'List Accounts', description: 'Search, sort, suspend, and manage accounts', category: 'Account Information', path: '/accounts', icon: 'users', capabilities: ['accounts.read'] },
	{ id: 'account-summary', label: 'Account Summary', description: 'Open an account hub for status, usage, and linked services', category: 'Account Information', path: '/accounts?task=summary', icon: 'account', capabilities: ['accounts.read'] },
	{ id: 'suspended', label: 'Suspended Accounts', description: 'Review suspended hosting identities', category: 'Account Information', path: '/accounts?view=suspended', icon: 'pause', capabilities: ['accounts.read'] },
	{ id: 'over-quota', label: 'Over Quota Accounts', description: 'Find accounts over package disk or bandwidth quota', category: 'Account Information', path: '/accounts?view=over-quota', icon: 'meter', capabilities: ['accounts.read', 'billing.usage.read', 'packages.read'], matchAll: true },
	{ id: 'create-account', label: 'Create Account', description: 'Provision a Kelmor account using a reviewed wizard', category: 'Account Functions', path: '/accounts/create', icon: 'plus', capabilities: ['accounts.create', 'packages.read'], matchAll: true },
	{ id: 'modify-account', label: 'Modify an Account', description: 'Change domain, address, ownership, and login access', category: 'Account Functions', path: '/accounts?task=modify', icon: 'edit', capabilities: ['accounts.read', 'accounts.modify'], matchAll: true },
	{ id: 'change-package', label: 'Change Account Package', description: 'Move an account to a different resource package', category: 'Account Functions', path: '/accounts?task=package', icon: 'box', capabilities: ['accounts.read', 'accounts.modify'], matchAll: true },
	{ id: 'suspend-account', label: 'Suspend or Unsuspend', description: 'Change account availability with a queued operation', category: 'Account Functions', path: '/accounts?task=suspension', icon: 'pause', capabilities: ['accounts.read', 'accounts.suspend'], matchAll: true },
	{ id: 'terminate-account', label: 'Terminate an Account', description: 'Review and permanently remove an account', category: 'Account Functions', path: '/accounts?task=terminate', icon: 'trash', capabilities: ['accounts.read', 'accounts.terminate'], matchAll: true },
	{ id: 'force-password', label: 'Force Password Change', description: 'Rotate owner credentials and require a change at sign-in', category: 'Account Functions', path: '/accounts?task=password', icon: 'key', capabilities: ['accounts.read', 'accounts.modify'], matchAll: true },
	{ id: 'login-control', label: 'Login to Kelmor Control', description: 'Open an audited Control session as the account owner', category: 'Account Functions', path: '/accounts?task=login', icon: 'key', capabilities: ['accounts.read', 'accounts.impersonate'], matchAll: true },
	{ id: 'list-domains', label: 'List Domains', description: 'List addon, subdomain, and parked domains across accounts', category: 'Account Information', path: '/domains', icon: 'globe', capabilities: ['accounts.read', 'domains.read'], matchAll: true },
	{ id: 'list-subdomains', label: 'List Subdomains', description: 'Filter the domain inventory to subdomains', category: 'Account Information', path: '/domains?view=subdomain', icon: 'globe', capabilities: ['accounts.read', 'domains.read'], matchAll: true },
	{ id: 'list-parked', label: 'List Parked Domains', description: 'Filter the domain inventory to parked alias domains', category: 'Account Information', path: '/domains?view=alias', icon: 'globe', capabilities: ['accounts.read', 'domains.read'], matchAll: true },
	{ id: 'packages', label: 'Packages', description: 'Manage reusable account limits and assignments', category: 'Packages', path: '/packages', icon: 'box', capabilities: ['packages.read'] },
	{ id: 'features', label: 'Feature Manager', description: 'Review package feature sets and assigned packages', category: 'Packages', path: '/features', icon: 'box', capabilities: ['packages.read'] },
	{ id: 'resellers', label: 'Resellers', description: 'Manage delegated operators and privileges', category: 'Resellers', path: '/resellers', icon: 'briefcase', capabilities: ['resellers.read'] },
	{ id: 'dns', label: 'DNS Management', description: 'Select an account and edit zones, records, and DNSSEC', category: 'DNS Functions', path: '/dns', icon: 'globe', capabilities: ['accounts.read', 'dns.read'], matchAll: true },
	{ id: 'websites', label: 'MultiPHP Manager', description: 'Review websites and queue PHP version changes', category: 'Software', path: '/websites', icon: 'code', capabilities: ['accounts.read', 'websites.read'], matchAll: true },
	{ id: 'files', label: 'File Manager', description: 'Browse, edit, and manage account files and web content', category: 'Files', path: '/files', icon: 'files', capabilities: ['accounts.read', 'files.read'], matchAll: true },
	{ id: 'ftp', label: 'FTP Accounts', description: 'Create and manage virtual FTP users for an account', category: 'Files', path: '/ftp', icon: 'files', capabilities: ['accounts.read', 'files.read'], matchAll: true },
	{ id: 'cron', label: 'Cron Jobs', description: 'Create and remove account-scoped scheduled tasks', category: 'Account Functions', path: '/cron', icon: 'jobs', capabilities: ['accounts.read', 'cron.read'], matchAll: true },
	{ id: 'sql', label: 'Database Manager', description: 'Create and manage MariaDB, MySQL, and PostgreSQL databases', category: 'SQL Services', path: '/sql', icon: 'database', capabilities: ['accounts.read', 'databases.read'], matchAll: true },
	{ id: 'email', label: 'Email Management', description: 'Manage mail domains, mailboxes, aliases, and routing', category: 'Email Functions', path: '/email', icon: 'mail', capabilities: ['accounts.read', 'mail.read'], matchAll: true },
	{ id: 'deliverability', label: 'Email Deliverability', description: 'Inspect SPF, DKIM, and DMARC records for mail domains', category: 'Email Functions', path: '/deliverability', icon: 'mail', capabilities: ['accounts.read', 'mail.read', 'dns.read'], matchAll: true },
	{ id: 'webmail', label: 'Webmail', description: 'Launch webmail and review IMAP/SMTP settings for mailboxes', category: 'Email Functions', path: '/webmail', icon: 'webmail', capabilities: ['accounts.read', 'mail.read'], matchAll: true },
	{ id: 'ssl', label: 'SSL / TLS', description: 'Review certificate inventory and request AutoSSL certificates', category: 'SSL/TLS', path: '/ssl', icon: 'lock', capabilities: ['accounts.read', 'websites.read'], matchAll: true },
	{ id: 'services', label: 'Service Status', description: 'Inspect server health, services, vitals, and processes', category: 'Server Status', path: '/status', icon: 'pulse', capabilities: ['server.read'] },
	{ id: 'processes', label: 'Process Manager', description: 'Inspect the live control-plane process snapshot', category: 'Server Status', path: '/processes', icon: 'pulse', capabilities: ['server.read'] },
	{ id: 'security', label: 'Security & Host Configuration', description: 'Audit, firewall configuration, and host reboot', category: 'Security Center', path: '/security', icon: 'shield', capabilities: ['server.read'] },
	{ id: 'transfers', label: 'Transfers & Backups', description: 'Native transfer, extracted archive import, backup, and restore', category: 'Transfers', path: '/transfers', icon: 'transfer', capabilities: ['accounts.read'] },
	{ id: 'jobs', label: 'Jobs', description: 'Inspect and retry background operations', category: 'System Tools', path: '/jobs', icon: 'jobs', capabilities: ['server.read', 'accounts.read'] },
	{ id: 'updates', label: 'Software Updates', description: 'Check, install, and schedule verified Kelmor releases', category: 'System Tools', path: '/updates', icon: 'box', capabilities: ['server.read'] },
	{ id: 'audit', label: 'Audit Trail', description: 'Search privileged actions and before/after state', category: 'Security Center', path: '/audit', icon: 'audit', capabilities: ['security.audit.read'] },
	{ id: 'usage', label: 'Account Usage', description: 'Compare disk, bandwidth, process, and memory usage', category: 'Account Information', path: '/usage', icon: 'chart', capabilities: ['billing.usage.read', 'accounts.read', 'packages.read'], matchAll: true },
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
		files: 'files',
		cron: 'cron',
		ftp: 'ftp',
		domains: 'domains',
		websites: 'websites',
	}
	const hubByTask: Record<string, string> = {
		databases: '/sql',
		email: '/email',
		certificates: '/ssl',
		files: '/files',
		cron: '/cron',
		ftp: '/ftp',
		domains: '/domains',
		websites: '/websites',
		deliverability: '/deliverability',
	}
	if (hubByTask[task]) return `${hubByTask[task]}?account=${accountId}`
	const service = serviceByTask[task]
	if (service) return `/accounts/${accountId}/services?service=${service}`
	if (['password', 'terminate', 'package', 'modify', 'suspension', 'summary', 'login'].includes(task)) {
		return `/accounts/${accountId}?task=${task}`
	}
	return `/accounts/${accountId}`
}
