import { describe, expect, test } from 'vitest'
import { accountTaskTarget, discoverTools, toolCatalog } from './tool-catalog'

describe('tool discovery', () => {
	test('keeps only tools allowed by the current capabilities', () => {
		const tools = discoverTools(toolCatalog, {
			'accounts.read': true,
			'packages.read': false,
			'server.read': true,
		})

		expect(tools.some((tool) => tool.label === 'List Accounts')).toBe(true)
		expect(tools.some((tool) => tool.label === 'Packages')).toBe(false)
		expect(tools.some((tool) => tool.label === 'Service Status')).toBe(true)
	})

	test('includes tools that accept any one of several capabilities', () => {
		const tools = discoverTools(toolCatalog, { 'accounts.read': true })

		expect(tools.some((tool) => tool.label === 'Jobs')).toBe(true)
	})

	test('covers the primary operator tool families', () => {
		const tools = discoverTools(toolCatalog, {
			'accounts.read': true,
			'accounts.modify': true,
			'accounts.suspend': true,
			'accounts.terminate': true,
			'databases.read': true,
			'files.read': true,
			'mail.read': true,
			'websites.read': true,
		})

		expect(tools.map((tool) => tool.label)).toEqual(expect.arrayContaining([
			'Account Summary',
			'Modify an Account',
			'Change Account Package',
			'Suspend or Unsuspend',
			'Terminate an Account',
			'Force Password Change',
			'Database Manager',
			'Email Management',
			'SSL / TLS',
			'File Manager',
			'Webmail',
		]))
	})

	test('includes WHM-mapped first-class tools when capabilities allow them', () => {
		const tools = discoverTools(toolCatalog, {
			'accounts.read': true,
			'accounts.impersonate': true,
			'cron.read': true,
			'domains.read': true,
			'dns.read': true,
			'files.read': true,
			'mail.read': true,
			'packages.read': true,
			'websites.read': true,
		})

		expect(tools.map((tool) => tool.label)).toEqual(expect.arrayContaining([
			'List Domains',
			'List Subdomains',
			'List Parked Domains',
			'Feature Manager',
			'MultiPHP Manager',
			'FTP Accounts',
			'Cron Jobs',
			'Email Deliverability',
			'Login to Kelmor Control',
		]))
	})

	test('requires account inventory access for account-scoped service selectors', () => {
		const tools = discoverTools(toolCatalog, {
			'databases.read': true,
			'dns.read': true,
			'mail.read': true,
			'websites.read': true,
		})

		expect(tools.map((tool) => tool.id)).not.toEqual(expect.arrayContaining(['dns', 'sql', 'email', 'ssl']))
	})

	test.each([
		['over-quota', ['accounts.read', 'billing.usage.read', 'packages.read']],
		['create-account', ['accounts.create', 'packages.read']],
		['modify-account', ['accounts.read', 'accounts.modify']],
		['change-package', ['accounts.read', 'accounts.modify']],
		['suspend-account', ['accounts.read', 'accounts.suspend']],
		['terminate-account', ['accounts.read', 'accounts.terminate']],
		['force-password', ['accounts.read', 'accounts.modify']],
		['usage', ['billing.usage.read', 'accounts.read', 'packages.read']],
		['transfers', ['accounts.read']],
		['login-control', ['accounts.read', 'accounts.impersonate']],
		['list-domains', ['accounts.read', 'domains.read']],
		['websites', ['accounts.read', 'websites.read']],
		['cron', ['accounts.read', 'cron.read']],
		['deliverability', ['accounts.read', 'mail.read', 'dns.read']],
	] as const)('requires every capability for the %s tool', (toolId, requiredCapabilities) => {
		for (const omittedCapability of requiredCapabilities) {
			const capabilities = Object.fromEntries(requiredCapabilities.map((capability) => [capability, capability !== omittedCapability]))
			expect(discoverTools(toolCatalog, capabilities).some((tool) => tool.id === toolId)).toBe(false)
		}

		const capabilities = Object.fromEntries(requiredCapabilities.map((capability) => [capability, true]))
		expect(discoverTools(toolCatalog, capabilities).some((tool) => tool.id === toolId)).toBe(true)
	})

	test('routes account service tools to dedicated hub pages', () => {
		expect(accountTaskTarget('databases', 'account-1')).toBe('/sql?account=account-1')
		expect(accountTaskTarget('email', 'account-1')).toBe('/email?account=account-1')
		expect(accountTaskTarget('certificates', 'account-1')).toBe('/ssl?account=account-1')
		expect(accountTaskTarget('files', 'account-1')).toBe('/files?account=account-1')
		expect(accountTaskTarget('cron', 'account-1')).toBe('/cron?account=account-1')
		expect(accountTaskTarget('domains', 'account-1')).toBe('/domains?account=account-1')
		expect(accountTaskTarget('login', 'account-1')).toBe('/accounts/account-1?task=login')
	})

	test.each(['password', 'terminate', 'package', 'modify', 'suspension', 'summary'])('preserves the %s lifecycle task after account selection', (task) => {
		expect(accountTaskTarget(task, 'account-1')).toBe(`/accounts/account-1?task=${task}`)
	})
})
