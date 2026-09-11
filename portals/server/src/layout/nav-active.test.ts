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
	tool('account-summary', '/accounts?task=summary'),
	tool('suspended', '/accounts?view=suspended'),
	tool('modify-account', '/accounts?task=modify', 'Account Functions'),
	tool('terminate-account', '/accounts?task=terminate', 'Account Functions'),
	tool('create-account', '/accounts/create', 'Account Functions'),
	tool('list-domains', '/domains'),
	tool('list-subdomains', '/domains?view=subdomain'),
	tool('list-parked', '/domains?view=alias'),
	tool('ssl', '/ssl', 'SSL/TLS'),
	tool('ssl-storage', '/ssl?task=inventory', 'SSL/TLS'),
	tool('generate-csr', '/ssl?task=request', 'SSL/TLS'),
	tool('manage-autossl', '/ssl?task=autossl', 'SSL/TLS'),
	tool('ssl-tls-status', '/ssl?task=status', 'SSL/TLS'),
	tool('service-ssl', '/ssl?task=service', 'SSL/TLS'),
	tool('jobs', '/jobs', 'System Tools'),
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
		expect(activeIds('/accounts', '?task=modify')).toEqual(['modify-account'])
		expect(activeIds('/accounts', '?task=terminate')).toEqual(['terminate-account'])
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

	test('highlights only the matching domain inventory view', () => {
		expect(activeIds('/domains')).toEqual(['list-domains'])
		expect(activeIds('/domains', '?view=subdomain')).toEqual(['list-subdomains'])
		expect(activeIds('/domains', '?view=alias')).toEqual(['list-parked'])
	})

	test('highlights only the matching SSL task on the shared /ssl manager', () => {
		expect(activeIds('/ssl')).toEqual(['ssl', 'ssl-storage'])
		expect(activeIds('/ssl', '?account=acc-1')).toEqual(['ssl', 'ssl-storage'])
		expect(activeIds('/ssl', '?account=acc-1&task=inventory')).toEqual(['ssl', 'ssl-storage'])
		expect(activeIds('/ssl', '?account=acc-1&task=request')).toEqual(['generate-csr'])
		expect(activeIds('/ssl', '?task=autossl')).toEqual(['manage-autossl'])
		expect(activeIds('/ssl', '?task=status')).toEqual(['ssl-tls-status'])
		expect(activeIds('/ssl', '?task=service')).toEqual(['service-ssl'])
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
})
