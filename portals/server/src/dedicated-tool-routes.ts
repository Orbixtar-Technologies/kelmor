export const dedicatedToolPaths = {
	'change-ownership': '/accounts/ownership',
	'modify-account': '/accounts/modify',
	'change-package': '/accounts/change-package',
	'force-password': '/accounts/force-password',
	'password-modification': '/accounts/password',
	'suspend-account': '/accounts/suspension',
	'terminate-account': '/accounts/terminate',
	'remove-terminated-account': '/accounts/remove',
	'login-control': '/accounts/login-control',
	'account-summary': '/accounts/summary',
	'api-tokens-whm': '/accounts/tokens',
	'add-package': '/packages/add',
	'delete-package': '/packages/delete',
	'add-dns-zone': '/domains/add',
	'delete-dns-zone': '/domains/delete',
	'park-domain': '/domains/park',
	'edit-dns-zone': '/dns/edit',
	'backup-restoration': '/transfers/restore',
	'copy-account': '/transfers/copy',
	'reseller-usage': '/resellers/usage',
	'generate-csr': '/ssl/request',
	'install-ssl': '/ssl/install',
	'manage-autossl': '/ssl/autossl',
	'ssl-storage': '/ssl/inventory',
	'ssl-tls-status': '/ssl/status',
	'service-ssl': '/ssl/service',
	'db-user-password': '/sql/password',
	'show-mysql-processes': '/sql/processes',
	'security-advisor': '/security/advisor',
	'graceful-reboot': '/security/reboot',
	'forceful-reboot': '/security/force-reboot',
	'server-information': '/status/info',
	'service-manager': '/status/services',
	'apache-status': '/status/http',
	'task-queue': '/jobs/queue',
	'daily-process-log': '/processes/daily',
	'update-preferences': '/updates/preferences',
	'change-log': '/updates/changelog',
	'disk-usage': '/usage/disk',
	'configure-email-client': '/webmail/client',
} as const

export type DedicatedToolId = keyof typeof dedicatedToolPaths

export const ACCOUNT_TOOL_SEGMENTS = new Set([
	'create',
	'ownership',
	'modify',
	'change-package',
	'password',
	'force-password',
	'suspension',
	'terminate',
	'remove',
	'login-control',
	'summary',
	'tokens',
])

export function dedicatedPath (id: DedicatedToolId): string {
	return dedicatedToolPaths[id]
}

export function isAccountDetailPath (pathname: string): boolean {
	const segment = pathname.match(/^\/accounts\/([^/]+)/)?.[1] || ''
	return Boolean(segment) && !ACCOUNT_TOOL_SEGMENTS.has(segment)
}

export function isDedicatedToolPath (pathname: string): boolean {
	return Object.values(dedicatedToolPaths).includes(pathname as typeof dedicatedToolPaths[DedicatedToolId])
}

function distinguishKey (pathname: string, params: URLSearchParams): string {
	const parts = [pathname]
	for (const key of ['task', 'view', 'mode', 'tab'] as const) {
		const value = params.get(key)
		if (value) parts.push(`${key}=${value}`)
	}
	return parts.join('|')
}

const LEGACY_REDIRECTS: Record<string, string> = {
	'/accounts|task=ownership': dedicatedPath('change-ownership'),
	'/accounts|task=modify': dedicatedPath('modify-account'),
	'/accounts|task=package': dedicatedPath('change-package'),
	'/accounts|task=password|mode=force': dedicatedPath('force-password'),
	'/accounts|task=password': dedicatedPath('password-modification'),
	'/accounts|task=suspension': dedicatedPath('suspend-account'),
	'/accounts|task=terminate': dedicatedPath('terminate-account'),
	'/accounts|view=terminated|task=remove': dedicatedPath('remove-terminated-account'),
	'/accounts|task=remove': dedicatedPath('remove-terminated-account'),
	'/accounts|task=login': dedicatedPath('login-control'),
	'/accounts|task=summary': dedicatedPath('account-summary'),
	'/accounts|task=tokens': dedicatedPath('api-tokens-whm'),
	'/packages|task=add': dedicatedPath('add-package'),
	'/packages|task=delete': dedicatedPath('delete-package'),
	'/domains|task=add': dedicatedPath('add-dns-zone'),
	'/domains|task=delete': dedicatedPath('delete-dns-zone'),
	'/domains|view=alias|task=park': dedicatedPath('park-domain'),
	'/dns|task=edit': dedicatedPath('edit-dns-zone'),
	'/transfers|task=restore': dedicatedPath('backup-restoration'),
	'/transfers|task=copy': dedicatedPath('copy-account'),
	'/resellers|task=usage': dedicatedPath('reseller-usage'),
	'/ssl|task=request': dedicatedPath('generate-csr'),
	'/ssl|task=install': dedicatedPath('install-ssl'),
	'/ssl|task=autossl': dedicatedPath('manage-autossl'),
	'/ssl|task=inventory': dedicatedPath('ssl-storage'),
	'/ssl|task=status': dedicatedPath('ssl-tls-status'),
	'/ssl|task=service': dedicatedPath('service-ssl'),
	'/sql|task=password': dedicatedPath('db-user-password'),
	'/sql|task=processes': dedicatedPath('show-mysql-processes'),
	'/security|task=advisor': dedicatedPath('security-advisor'),
	'/security|task=reboot': dedicatedPath('graceful-reboot'),
	'/security|task=force-reboot': dedicatedPath('forceful-reboot'),
	'/status|task=info': dedicatedPath('server-information'),
	'/status|task=services': dedicatedPath('service-manager'),
	'/status|task=http': dedicatedPath('apache-status'),
	'/jobs|view=queue': dedicatedPath('task-queue'),
	'/processes|view=daily': dedicatedPath('daily-process-log'),
	'/updates|task=preferences': dedicatedPath('update-preferences'),
	'/updates|task=changelog': dedicatedPath('change-log'),
	'/usage|view=disk': dedicatedPath('disk-usage'),
	'/webmail|task=client': dedicatedPath('configure-email-client'),
}

export function legacyDedicatedRedirect (pathname: string, search = ''): string | null {
	const params = new URLSearchParams(search.startsWith('?') ? search : search ? `?${search}` : '')
	const target = LEGACY_REDIRECTS[distinguishKey(pathname, params)]
	if (!target) return null
	const next = new URL(target, 'https://director.local')
	const account = params.get('account')
	if (account) next.searchParams.set('account', account)
	const query = next.searchParams.toString()
	return query ? `${next.pathname}?${query}` : next.pathname
}

export function accountFunctionHref (toolPath: string, accountId?: string): string {
	if (!accountId) return toolPath
	return `${toolPath}?account=${encodeURIComponent(accountId)}`
}

export function managerFocus (pathname: string, root: string): string {
	if (pathname === root) return ''
	if (pathname.startsWith(`${root}/`)) return pathname.slice(root.length + 1)
	return ''
}
