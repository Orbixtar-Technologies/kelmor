import { ACCOUNT_TOOL_SEGMENTS, dedicatedToolPaths } from '../dedicated-tool-routes'
import { hubById } from '../nav-hubs'
import { featureById, whmFeatures } from '../whm-catalog'

export interface BreadcrumbItem {
	label: string
	to?: string
}

export const directorCrumbLabels: Record<string, string> = {
	accounts: 'Accounts', create: 'Create Account', services: 'Account Services',
	server: 'Server Configuration',
	packages: 'Packages', resellers: 'Resellers', dns: 'DNS Management',
	status: 'Service Status', security: 'Security', transfers: 'Transfers & Backups',
	jobs: 'Jobs', audit: 'Audit Trail', usage: 'Account Usage',
	files: 'File Manager', sql: 'Database Manager', email: 'Email Management',
	ssl: 'SSL / TLS', webmail: 'Webmail', updates: 'Software Updates',
	domains: 'List Domains', websites: 'MultiPHP Manager', features: 'Feature Manager',
	ftp: 'FTP Accounts', cron: 'Cron Jobs', deliverability: 'Email Deliverability',
	redirects: 'Redirects', git: 'Git Version Control',
	processes: 'Process Manager',
	mail: 'Email',
	'delivery-reports': 'Mail Delivery Reports',
	'track-delivery': 'Track Delivery',
	'ip-usage': 'IP Address Usage',
	section: 'Section',
	tools: 'Tools',
	...Object.fromEntries(Object.entries(dedicatedToolPaths).map(([id, path]) => {
		const key = path.replace(/^\//, '')
		return [key, featureById(id)?.label || key]
	})),
	...Object.fromEntries(whmFeatures.filter((feature) => feature.path.startsWith('/tools/')).map((feature) => [feature.id, feature.label])),
}

function dedicatedLabel (pathname: string): string | undefined {
	const match = Object.entries(dedicatedToolPaths).find(([, path]) => path === pathname)
	if (!match) return undefined
	return featureById(match[0])?.label
}

function crumbLabel (parts: string[], index: number, labels: Record<string, string>): string {
	const pathname = `/${parts.slice(0, index + 1).join('/')}`
	const pathKey = parts.slice(0, index + 1).join('/')
	return dedicatedLabel(pathname) || labels[pathKey] || labels[parts[index]] || parts[index]
}

export function directorBreadcrumbs (
	pathname: string,
	labels: Record<string, string>,
	accountName?: string,
	toolAccountName?: string,
): BreadcrumbItem[] {
	const parts = pathname.split('/').filter(Boolean)
	const crumbs: BreadcrumbItem[] = [{ label: 'Home', to: '/' }]
	if (!parts.length) return crumbs

	if (parts[0] === 'section') {
		const hub = hubById(parts[1] || '')
		crumbs.push({ label: hub?.label || labels[parts[1]] || parts[1] })
		return crumbs
	}

	if (parts[0] === 'accounts') {
		crumbs.push({ label: 'Accounts', to: '/accounts' })
		if (parts[1] && ACCOUNT_TOOL_SEGMENTS.has(parts[1])) {
			crumbs.push({ label: crumbLabel(parts, 1, labels) })
			return crumbs
		}
		if (parts[1]) {
			crumbs.push({ label: accountName || parts[1], to: `/accounts/${parts[1]}` })
			if (parts[2] === 'services') crumbs.push({ label: 'Services' })
		}
		return crumbs
	}

	parts.forEach((_, index) => {
		const isLast = index === parts.length - 1
		const label = crumbLabel(parts, index, labels)
		const to = `/${parts.slice(0, index + 1).join('/')}`
		crumbs.push(isLast ? { label } : { label, to })
	})
	if (toolAccountName) crumbs.push({ label: toolAccountName })
	return crumbs
}
