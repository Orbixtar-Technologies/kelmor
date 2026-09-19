import { describe, expect, test } from 'vitest'
import { isDirectorToolActive, navScopeForCategory } from './nav-active'
import type { ToolDefinition } from '../types'

function tool (id: string, path: string, category = 'Account Information'): ToolDefinition {
	return {
		id,
		label: id,
		description: id,
		category,
		path,
		icon: 'users',
		capabilities: ['accounts.read'],
	}
}

const catalog = [
	tool('home', '/', 'Kelmor Director'),
	tool('accounts', '/accounts'),
	tool('account-summary', '/accounts/summary'),
	tool('suspended', '/accounts?view=suspended'),
	tool('modify-account', '/accounts/modify', 'Account Functions'),
	tool('terminate-account', '/accounts/terminate', 'Account Functions'),
	tool('create-account', '/accounts/create', 'Account Functions'),
	tool('list-domains', '/domains'),
	tool('list-subdomains', '/domains?view=subdomain'),
	tool('list-parked', '/domains?view=alias'),
	tool('ssl', '/ssl', 'SSL/TLS'),
	tool('ssl-storage', '/ssl/inventory', 'SSL/TLS'),
	tool('generate-csr', '/ssl/request', 'SSL/TLS'),
	tool('manage-autossl', '/ssl/autossl', 'SSL/TLS'),
	tool('ssl-tls-status', '/ssl/status', 'SSL/TLS'),
	tool('service-ssl', '/ssl/service', 'SSL/TLS'),
	tool('jobs', '/jobs', 'System Tools'),
	tool('mail-delivery-reports', '/mail/delivery-reports', 'Email'),
	tool('track-delivery', '/mail/track-delivery', 'Email'),
	tool('review-transfers', '/jobs?q=transfer', 'Transfers'),
	tool('updates', '/updates', 'System Tools'),
	tool('update-preferences', '/updates/preferences', 'Server Configuration'),
	tool('change-log', '/updates/changelog', 'cPanel'),
]

function activeIds (pathname: string, search = '') {
	return catalog.filter((entry) => isDirectorToolActive(entry, pathname, search)).map((entry) => entry.id)
}

describe('isDirectorToolActive', () => {
	test('highlights only Home on the dashboard', () => {
		expect(activeIds('/')).toEqual(['home'])
	})

	test('highlights only List Accounts on the unfiltered inventory', () => {
		expect(activeIds('/accounts')).toEqual(['accounts'])
	})

	test('highlights only the matching query destination on shared /accounts paths', () => {
		expect(activeIds('/accounts', '?view=suspended')).toEqual(['suspended'])
		expect(activeIds('/accounts/modify')).toEqual(['modify-account'])
		expect(activeIds('/accounts/terminate')).toEqual(['terminate-account'])
		expect(activeIds('/accounts', '?task=modify')).toEqual(['accounts'])
	})

	test('highlights Account Summary for a specific account hub, not List Accounts', () => {
		expect(activeIds('/accounts/acc-1')).toEqual(['account-summary'])
		expect(activeIds('/accounts/acc-1/services', '?service=databases')).toEqual(['account-summary'])
	})

	test('highlights Create Account without activating List Accounts', () => {
		expect(activeIds('/accounts/create')).toEqual(['create-account'])
	})

	test('highlights Jobs even when an account filter is present', () => {
		expect(activeIds('/jobs', '?account=acc-1')).toEqual(['jobs'])
	})

	test('highlights only the selected mail delivery tool', () => {
		expect(activeIds('/mail/delivery-reports')).toEqual(['mail-delivery-reports'])
		expect(activeIds('/mail/track-delivery')).toEqual(['track-delivery'])
		expect(activeIds('/jobs', '?q=mail')).toEqual(['jobs'])
	})

	test('highlights Review Transfers only for the transfer Jobs filter', () => {
		expect(activeIds('/jobs', '?q=transfer')).toEqual(['review-transfers'])
	})

	test('highlights only the matching domain inventory view', () => {
		expect(activeIds('/domains')).toEqual(['list-domains'])
		expect(activeIds('/domains', '?view=subdomain')).toEqual(['list-subdomains'])
		expect(activeIds('/domains', '?view=alias')).toEqual(['list-parked'])
	})

	test('highlights only the matching dedicated update tool', () => {
		expect(activeIds('/updates')).toEqual(['updates'])
		expect(activeIds('/updates/preferences')).toEqual(['update-preferences'])
		expect(activeIds('/updates/changelog')).toEqual(['change-log'])
	})

	test('highlights only the matching dedicated SSL tool', () => {
		expect(activeIds('/ssl')).toEqual(['ssl'])
		expect(activeIds('/ssl', '?account=acc-1')).toEqual(['ssl'])
		expect(activeIds('/ssl/inventory', '?account=acc-1')).toEqual(['ssl-storage'])
		expect(activeIds('/ssl/request', '?account=acc-1')).toEqual(['generate-csr'])
		expect(activeIds('/ssl/autossl')).toEqual(['manage-autossl'])
		expect(activeIds('/ssl/status')).toEqual(['ssl-tls-status'])
		expect(activeIds('/ssl/service')).toEqual(['service-ssl'])
	})
})

describe('navScopeForCategory', () => {
	test('labels account versus host tool families', () => {
		expect(navScopeForCategory('Account Information')).toBe('account')
		expect(navScopeForCategory('Account Functions')).toBe('account')
		expect(navScopeForCategory('Server Status')).toBe('host')
		expect(navScopeForCategory('Security Center')).toBe('host')
		expect(navScopeForCategory('Software')).toBe('account')
		expect(navScopeForCategory('Email')).toBe('account')
		expect(navScopeForCategory('Backup')).toBe('account')
		expect(navScopeForCategory('Restart Services')).toBe('host')
		expect(navScopeForCategory('Server Configuration')).toBe('host')
	})
})

describe('generic tool paths', () => {
	test('highlights only the matching /tools/:id page', () => {
		const tweak = tool('tweak-settings', '/tools/tweak-settings', 'Server Configuration')
		const hostname = tool('change-hostname', '/tools/change-hostname', 'Networking Setup')
		expect(isDirectorToolActive(tweak, '/tools/tweak-settings')).toBe(true)
		expect(isDirectorToolActive(hostname, '/tools/tweak-settings')).toBe(false)
	})

	test('highlights only the selected tool on a combined hub page', () => {
		const tweak = tool('tweak-settings', '/tools/tweak-settings', 'Server Configuration')
		const hostname = tool('change-hostname', '/tools/change-hostname', 'Networking Setup')
		expect(isDirectorToolActive(tweak, '/section/server', '?tool=tweak-settings')).toBe(true)
		expect(isDirectorToolActive(hostname, '/section/server', '?tool=tweak-settings')).toBe(false)
	})
})
