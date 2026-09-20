import { hrefForFeature } from '../nav-hubs'
import type { FeatureSet } from '../types'
import { featureById } from '../whm-catalog'

export interface ShowcaseHostApp {
	id: string
	label: string
	status: string
	description: string
}

export interface ShowcaseEntry {
	id: string
	name: string
	status: string
	enabled: boolean
	description: string
	href?: string
	toolLabel?: string
	group: 'package' | 'host'
	detail: string
}

const PACKAGE_FEATURE_TOOLS: Record<string, string> = {
	websites: 'websites',
	dns: 'dns',
	email: 'email',
	databases: 'sql',
	files: 'file-manager',
	ftp: 'ftp',
	applications: 'market',
	wordpress: 'wp-toolkit',
	ssl: 'ssl',
	backups: 'transfers',
	cron: 'cron-jobs',
	ssh: 'manage-shell',
	api: 'api-tokens-whm',
}

const HOST_APP_TOOLS: Record<string, string> = {
	phpmyadmin: 'phpmyadmin',
	roundcube: 'webmail',
	wordpress: 'wp-toolkit',
	rspamd: 'plugins',
}

export function assembleFeatureShowcase ({
	featureSets,
	packages,
	hostApps,
}: {
	featureSets: FeatureSet[]
	packages: Array<{ id: string; name: string; feature_set_id: string }>
	hostApps: ShowcaseHostApp[]
}): ShowcaseEntry[] {
	const packageEntries = packageCapabilityEntries(featureSets, packages)
	const hostEntries = hostApps
		.map((app) => hostAppEntry(app))
		.sort((left, right) => left.name.localeCompare(right.name))
	return [...packageEntries, ...hostEntries]
}

function packageCapabilityEntries (featureSets: FeatureSet[], packages: Array<{ id: string; name: string; feature_set_id: string }>): ShowcaseEntry[] {
	const keys = new Set<string>()
	for (const set of featureSets) {
		for (const key of Object.keys(set.features || {})) keys.add(key)
	}
	return [...keys].sort((left, right) => left.localeCompare(right)).map((key) => {
		const enabledSets = featureSets.filter((set) => set.features?.[key])
		const assigned = packages.filter((pkg) => enabledSets.some((set) => set.id === pkg.feature_set_id))
		const tool = toolRef(PACKAGE_FEATURE_TOOLS[key])
		const enabled = enabledSets.length > 0
		return {
			id: `package:${key}`,
			name: tool?.label || humanizeKey(key),
			status: enabled ? 'enabled' : 'disabled',
			enabled,
			description: tool?.description || 'Package capability from Feature Manager.',
			href: tool?.href || '/features',
			toolLabel: tool?.label || 'Feature Manager',
			group: 'package',
			detail: enabled
				? `Enabled in ${enabledSets.length} of ${featureSets.length} feature set${featureSets.length === 1 ? '' : 's'} · ${assigned.length} package${assigned.length === 1 ? '' : 's'}`
				: `Disabled in every feature set · ${assigned.length} package${assigned.length === 1 ? '' : 's'}`,
		}
	})
}

function hostAppEntry (app: ShowcaseHostApp): ShowcaseEntry {
	const tool = toolRef(HOST_APP_TOOLS[app.id])
	const status = (app.status || 'unknown').toLocaleLowerCase()
	return {
		id: `host:${app.id}`,
		name: app.label || humanizeKey(app.id),
		status,
		enabled: status === 'installed' || status === 'available' || status === 'running',
		description: app.description || tool?.description || 'Host application reported by the Kelmor Agent.',
		href: tool?.href,
		toolLabel: tool?.label,
		group: 'host',
		detail: status === 'installed' ? 'Installed on this host' : status === 'available' ? 'Available to install on this host' : `Host status: ${status}`,
	}
}

function toolRef (id: string | undefined) {
	if (!id) return undefined
	const feature = featureById(id)
	if (!feature) return undefined
	return {
		label: feature.label,
		description: feature.description,
		href: hrefForFeature(feature),
	}
}

function humanizeKey (key: string) {
	return key.replaceAll(/[_-]+/g, ' ').trim() || key
}
