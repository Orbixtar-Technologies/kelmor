export interface ControlHubTab {
	id: string
	label: string
	cap: string
}

export interface ControlHub {
	id: string
	label: string
	path: string
	caps: string[]
	tabs: ControlHubTab[]
}

export const controlHubs: ControlHub[] = [
	{ id: 'dashboard', label: 'Dashboard', path: '/', caps: [], tabs: [] },
	{
		id: 'websites',
		label: 'Websites',
		path: '/websites',
		caps: ['websites.read'],
		tabs: [
			{ id: 'sites', label: 'Sites', cap: 'websites.read' },
			{ id: 'ssl', label: 'SSL/TLS', cap: 'websites.read' },
		],
	},
	{
		id: 'domains',
		label: 'Domains',
		path: '/domains',
		caps: ['domains.read', 'dns.read'],
		tabs: [
			{ id: 'domains', label: 'Domains', cap: 'domains.read' },
			{ id: 'dns', label: 'DNS', cap: 'dns.read' },
		],
	},
	{ id: 'email', label: 'Email', path: '/email', caps: ['mail.read'], tabs: [] },
	{ id: 'databases', label: 'Databases', path: '/databases', caps: ['databases.read'], tabs: [] },
	{ id: 'files', label: 'Files', path: '/files', caps: ['files.read'], tabs: [] },
	{
		id: 'backups',
		label: 'Backups',
		path: '/backups',
		caps: ['backups.read', 'cron.read'],
		tabs: [
			{ id: 'backups', label: 'Backups', cap: 'backups.read' },
			{ id: 'cron', label: 'Cron', cap: 'cron.read' },
		],
	},
]

export function canSeeControlHub (hub: ControlHub, capabilities: Record<string, boolean>): boolean {
	return hub.caps.length === 0 || hub.caps.some((cap) => capabilities[cap])
}

export function controlHubEntryPath (hub: ControlHub, capabilities: Record<string, boolean>): string {
	if (!hub.tabs.length) return hub.path
	const first = hub.tabs.find((tab) => capabilities[tab.cap])
	if (!first || first.id === hub.tabs[0].id) return hub.path
	return `${hub.path}?tab=${encodeURIComponent(first.id)}`
}

export function visibleControlTabs (hub: ControlHub, capabilities: Record<string, boolean>): ControlHubTab[] {
	return hub.tabs.filter((tab) => capabilities[tab.cap])
}

export function activeControlTab (hub: ControlHub, tabParam: string | null, capabilities: Record<string, boolean>): ControlHubTab | undefined {
	const allowed = visibleControlTabs(hub, capabilities)
	return allowed.find((tab) => tab.id === tabParam) || allowed[0]
}
