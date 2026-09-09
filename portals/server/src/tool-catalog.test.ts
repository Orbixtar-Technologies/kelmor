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

	test('routes account service tools to the selected account section', () => {
		expect(accountTaskTarget('databases', 'account-1')).toBe('/accounts/account-1/services?service=databases')
		expect(accountTaskTarget('email', 'account-1')).toBe('/accounts/account-1/services?service=mailboxes')
		expect(accountTaskTarget('password', 'account-1')).toBe('/accounts/account-1')
	})
})
