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
			'SQL Services',
			'Email Services',
			'SSL Certificates',
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
	] as const)('requires every capability for the %s tool', (toolId, requiredCapabilities) => {
		for (const omittedCapability of requiredCapabilities) {
			const capabilities = Object.fromEntries(requiredCapabilities.map((capability) => [capability, capability !== omittedCapability]))
			expect(discoverTools(toolCatalog, capabilities).some((tool) => tool.id === toolId)).toBe(false)
		}

		const capabilities = Object.fromEntries(requiredCapabilities.map((capability) => [capability, true]))
		expect(discoverTools(toolCatalog, capabilities).some((tool) => tool.id === toolId)).toBe(true)
	})

	test('routes account service tools to the selected account section', () => {
		expect(accountTaskTarget('databases', 'account-1')).toBe('/accounts/account-1/services?service=databases')
		expect(accountTaskTarget('email', 'account-1')).toBe('/accounts/account-1/services?service=mailboxes')
		expect(accountTaskTarget('certificates', 'account-1')).toBe('/accounts/account-1/services?service=certificates')
	})

	test.each(['password', 'terminate', 'package', 'modify', 'suspension', 'summary'])('preserves the %s lifecycle task after account selection', (task) => {
		expect(accountTaskTarget(task, 'account-1')).toBe(`/accounts/account-1?task=${task}`)
	})
})
