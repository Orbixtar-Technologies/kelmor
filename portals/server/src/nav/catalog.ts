import type { IconName } from '../components/icons'

export interface Tool {
	/** Route path. Also the stable key used for favourites. */
	path: string
	name: string
	description: string
	icon: IconName
	/** Capability that must be present for this tool to be listed or routed. */
	cap: string
	/** Extra words the global find should match on. */
	keywords?: string
}

export interface Category {
	id: string
	name: string
	icon: IconName
	tools: Tool[]
}

export const catalog: Category[] = [
	{
		id: 'account-functions',
		name: 'Account Functions',
		icon: 'userPlus',
		tools: [
			{ path: '/accounts/create', name: 'Create a New Account', description: 'Guided wizard from identity to provisioning job.', icon: 'userPlus', cap: 'accounts.create', keywords: 'new add provision customer signup wizard' },
			{ path: '/accounts/modify', name: 'Modify an Account', description: 'Change domain, dedicated IP, reseller owner and login state.', icon: 'edit', cap: 'accounts.modify', keywords: 'edit update rename ip owner' },
			{ path: '/accounts/change-package', name: 'Upgrade / Downgrade an Account', description: 'Move an account onto a different package and reconcile limits.', icon: 'layers', cap: 'accounts.modify', keywords: 'package plan upgrade downgrade change resources' },
			{ path: '/accounts/suspension', name: 'Manage Account Suspension', description: 'Suspend or restore hosting for one or many accounts.', icon: 'pause', cap: 'accounts.suspend', keywords: 'suspend unsuspend disable enable hold' },
			{ path: '/accounts/terminate', name: 'Terminate Accounts', description: 'Permanently remove accounts, with a typed confirmation.', icon: 'trash', cap: 'accounts.terminate', keywords: 'delete remove destroy cancel' },
			{ path: '/accounts/password', name: 'Force Password Change', description: 'Rotate the account owner password and require a reset at sign-in.', icon: 'key', cap: 'accounts.modify', keywords: 'password reset credentials force login' },
			{ path: '/accounts/quotas', name: 'Limit Bandwidth and Disk', description: 'Review consumption against package caps and adjust the package.', icon: 'scale', cap: 'accounts.modify', keywords: 'quota bandwidth disk limit cap transfer' },
		],
	},
	{
		id: 'account-information',
		name: 'Account Information',
		icon: 'users',
		tools: [
			{ path: '/accounts', name: 'List Accounts', description: 'Every hosting account with disk, transfer, package and status.', icon: 'users', cap: 'accounts.read', keywords: 'accounts customers list table search domains' },
			{ path: '/accounts/suspended', name: 'List Suspended Accounts', description: 'Accounts currently held out of service.', icon: 'userMinus', cap: 'accounts.read', keywords: 'suspended disabled held' },
			{ path: '/accounts/over-quota', name: 'Show Accounts Over Quota', description: 'Accounts at or above their disk or monthly transfer limit.', icon: 'alertTriangle', cap: 'accounts.read', keywords: 'over quota full disk bandwidth exceeded' },
			{ path: '/accounts/summary', name: 'Account Summary', description: 'Open the management hub for a single account.', icon: 'compass', cap: 'accounts.read', keywords: 'summary detail hub inspect account' },
		],
	},
	{
		id: 'packages',
		name: 'Packages',
		icon: 'box',
		tools: [
			{ path: '/packages', name: 'Packages', description: 'Reusable resource and feature limits applied to accounts.', icon: 'box', cap: 'packages.read', keywords: 'plans packages limits resources' },
			{ path: '/packages/new', name: 'Add a Package', description: 'Create a package with disk, transfer, mail and process limits.', icon: 'plus', cap: 'packages.write', keywords: 'new package plan create' },
			{ path: '/packages/features', name: 'Feature Manager', description: 'Feature sets that decide which tools an account can use.', icon: 'sliders', cap: 'packages.read', keywords: 'features feature manager toggles capabilities' },
		],
	},
	{
		id: 'resellers',
		name: 'Resellers',
		icon: 'briefcase',
		tools: [
			{ path: '/resellers', name: 'Resellers', description: 'Delegated brands, their accounts and their packages.', icon: 'briefcase', cap: 'resellers.read', keywords: 'reseller partner brand delegate' },
			{ path: '/resellers/new', name: 'Add a Reseller', description: 'Create a reseller brand with its own sign-in and nameservers.', icon: 'plus', cap: 'resellers.create', keywords: 'new reseller create partner' },
		],
	},
	{
		id: 'dns',
		name: 'DNS Functions',
		icon: 'globe',
		tools: [
			{ path: '/dns/zones', name: 'DNS Zone Manager', description: 'Every authoritative zone on this host with record counts.', icon: 'globe', cap: 'dns.read', keywords: 'dns zone records powerdns authoritative' },
			{ path: '/dns/add-zone', name: 'Add a DNS Zone', description: 'How zones are created on Kelmor and what to do instead.', icon: 'plus', cap: 'dns.read', keywords: 'add zone create dns new' },
			{ path: '/dns/dnssec', name: 'DNSSEC', description: 'Signing state and DS records to hand to each registrar.', icon: 'shield', cap: 'dns.read', keywords: 'dnssec ds signing keys registrar' },
		],
	},
	{
		id: 'sql',
		name: 'SQL Services',
		icon: 'database',
		tools: [
			{ path: '/sql', name: 'Databases', description: 'MariaDB and PostgreSQL databases grouped by account.', icon: 'database', cap: 'databases.read', keywords: 'sql mysql mariadb postgres database' },
		],
	},
	{
		id: 'email',
		name: 'Email',
		icon: 'mail',
		tools: [
			{ path: '/email', name: 'Mail Domains and Mailboxes', description: 'Mail routing, catch-all policy, mailboxes and aliases.', icon: 'mail', cap: 'mail.read', keywords: 'email mail mailbox alias catchall postfix dovecot' },
		],
	},
	{
		id: 'ssl',
		name: 'SSL/TLS',
		icon: 'lock',
		tools: [
			{ path: '/ssl', name: 'Certificates', description: 'Issued certificates, expiry, and new ACME requests.', icon: 'lock', cap: 'accounts.read', keywords: 'ssl tls certificate acme lets encrypt https' },
		],
	},
	{
		id: 'server-status',
		name: 'Server Status',
		icon: 'activity',
		tools: [
			{ path: '/server/status', name: 'Service Status', description: 'Observed health of every managed service on this host.', icon: 'activity', cap: 'server.read', keywords: 'services status health nginx postfix running' },
			{ path: '/server/vitals', name: 'Host Vitals', description: 'Load, memory, disk, inodes and uptime from the agent.', icon: 'server', cap: 'server.read', keywords: 'load memory disk inodes uptime vitals metrics' },
			{ path: '/server/processes', name: 'Process Manager', description: 'Processes observed by the privileged agent.', icon: 'terminal', cap: 'server.read', keywords: 'process ps top cpu memory' },
		],
	},
	{
		id: 'security',
		name: 'Security Center',
		icon: 'shield',
		tools: [
			{ path: '/security/firewall', name: 'Host Firewall', description: 'Apply the nftables policy that fronts this host.', icon: 'shield', cap: 'server.firewall.read', keywords: 'firewall nftables ports drop policy security' },
			{ path: '/security/audit', name: 'Audit Log', description: 'Filterable privileged action trail with full event detail.', icon: 'fileText', cap: 'security.audit.read', keywords: 'audit log trail history security events who did' },
		],
	},
	{
		id: 'transfers',
		name: 'Backup and Transfers',
		icon: 'archive',
		tools: [
			{ path: '/transfers/backups', name: 'Account Backups', description: 'Queue encrypted backups and review completed runs.', icon: 'archive', cap: 'backups.read', keywords: 'backup encrypted local sftp s3 snapshot' },
			{ path: '/transfers/restore', name: 'Restore a Backup', description: 'Restore an account in place from a completed backup run.', icon: 'refresh', cap: 'backups.restore', keywords: 'restore recover rollback backup' },
			{ path: '/transfers/migrate', name: 'Transfer or Migrate an Account', description: 'Copy an account onto a new username and primary domain.', icon: 'externalLink', cap: 'accounts.create', keywords: 'transfer migrate move copy rename' },
			{ path: '/transfers/import', name: 'Import an Account', description: 'Restore a Kelmor export or an extracted cPanel cpmove tree.', icon: 'upload', cap: 'accounts.create', keywords: 'import cpanel cpmove restore export migrate onboard' },
			{ path: '/transfers/export', name: 'Export Accounts', description: 'Download portable account exports for offsite storage.', icon: 'download', cap: 'accounts.read', keywords: 'export download backup portable json' },
		],
	},
	{
		id: 'server-configuration',
		name: 'Server Configuration',
		icon: 'sliders',
		tools: [
			{ path: '/server/reboot', name: 'Graceful Server Reboot', description: 'Record a reboot through the privileged agent.', icon: 'power', cap: 'server.settings.write', keywords: 'reboot restart shutdown power host' },
		],
	},
	{
		id: 'jobs',
		name: 'Job Queue',
		icon: 'list',
		tools: [
			{ path: '/jobs', name: 'Job Queue', description: 'Durable queue history with retry, cancel and drill-down.', icon: 'list', cap: 'server.read', keywords: 'jobs queue tasks background operations retry cancel' },
		],
	},
	{
		id: 'statistics',
		name: 'Statistics',
		icon: 'barChart',
		tools: [
			{ path: '/usage', name: 'Bandwidth and Disk Usage', description: 'Measured consumption per account against package caps.', icon: 'barChart', cap: 'billing.usage.read', keywords: 'usage bandwidth disk inodes statistics consumption' },
		],
	},
]

export const allTools: Tool[] = catalog.flatMap((category) => category.tools)

export function toolByPath (path: string) {
	return allTools.find((tool) => tool.path === path)
}

export function categoryOf (path: string) {
	return catalog.find((category) => category.tools.some((tool) => tool.path === path))
}

export function visibleCatalog (caps: Record<string, boolean>): Category[] {
	return catalog
		.map((category) => ({ ...category, tools: category.tools.filter((tool) => caps[tool.cap]) }))
		.filter((category) => category.tools.length > 0)
}

/** Ranks a tool against a find query. Returns 0 when it does not match. */
export function scoreTool (tool: Tool, query: string) {
	const q = query.trim().toLowerCase()
	if (!q) return 0
	const name = tool.name.toLowerCase()
	if (name === q) return 100
	if (name.startsWith(q)) return 80
	if (name.includes(q)) return 60
	if ((tool.keywords || '').includes(q)) return 40
	if (tool.description.toLowerCase().includes(q)) return 20
	return 0
}
