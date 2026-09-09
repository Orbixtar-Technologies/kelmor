import { describe, expect, test } from 'vitest'
import { rankFindResults, shouldHandleFindShortcut } from './global-find'
import type { Account, ToolDefinition } from '../types'

const tools: ToolDefinition[] = [
	{
		id: 'dns',
		label: 'DNS Management',
		description: 'Edit zones and records',
		category: 'DNS Functions',
		path: '/dns',
		icon: 'globe',
		capabilities: ['dns.read'],
	},
	{
		id: 'accounts',
		label: 'List Accounts',
		description: 'Search customers and hosting identities',
		category: 'Account Information',
		path: '/accounts',
		icon: 'users',
		capabilities: ['accounts.read'],
	},
]

const accounts: Account[] = [
	{
		id: 'a1',
		username: 'kelmor-demo',
		primary_domain: 'demo.kelmor.test',
		owner_user_id: 'u1',
		package_id: 'p1',
		status: 'active',
		home_path: '/home/kelmor-demo',
		linux_uid: 1001,
		linux_gid: 1001,
		shell_class: 'sftp-only',
		login_disabled: false,
		desired_revision: 1,
		observed_revision: 1,
	},
]

describe('global find ranking', () => {
	test('matches feature labels and descriptions', () => {
		expect(rankFindResults('zones', tools, []).map((result) => result.label)).toEqual(['DNS Management'])
		expect(rankFindResults('list accounts', tools, [])[0]?.label).toBe('List Accounts')
	})

	test('matches account usernames and domains and ranks exact prefixes first', () => {
		const byUsername = rankFindResults('kelmor', tools, accounts)
		const byDomain = rankFindResults('demo.kelmor', tools, accounts)

		expect(byUsername[0]?.label).toBe('kelmor-demo')
		expect(byDomain[0]?.description).toContain('demo.kelmor.test')
	})

	test('handles slash outside editable controls only', () => {
		const input = document.createElement('input')
		const div = document.createElement('div')

		expect(shouldHandleFindShortcut('/', false, div)).toBe(true)
		expect(shouldHandleFindShortcut('/', false, input)).toBe(false)
		expect(shouldHandleFindShortcut('/', true, div)).toBe(false)
	})
})
