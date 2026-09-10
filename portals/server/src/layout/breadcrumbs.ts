export interface BreadcrumbItem {
	label: string
	to?: string
}

export function directorBreadcrumbs (
	pathname: string,
	labels: Record<string, string>,
	accountName?: string,
): BreadcrumbItem[] {
	const parts = pathname.split('/').filter(Boolean)
	const crumbs: BreadcrumbItem[] = [{ label: 'Home', to: '/' }]
	if (!parts.length) return crumbs

	if (parts[0] === 'accounts') {
		crumbs.push({ label: 'Accounts', to: '/accounts' })
		if (parts[1] === 'create') {
			crumbs.push({ label: 'Create Account' })
			return crumbs
		}
		if (parts[1]) {
			crumbs.push({ label: accountName || parts[1], to: `/accounts/${parts[1]}` })
			if (parts[2] === 'services') crumbs.push({ label: 'Services' })
		}
		return crumbs
	}

	parts.forEach((part, index) => {
		const isLast = index === parts.length - 1
		const label = labels[part] || part
		const to = `/${parts.slice(0, index + 1).join('/')}`
		crumbs.push(isLast ? { label } : { label, to })
	})
	return crumbs
}
