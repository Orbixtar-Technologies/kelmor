import { describe, expect, test } from 'vitest'
import {
	failedJobDisplay,
	measuredVital,
	operatorGroupForTool,
	operatorNavGroups,
} from './operator-nav'
import { toolCatalog } from './tool-catalog'
import type { ToolDefinition } from './types'
import { whmFeatures } from './whm-catalog'

const sample: ToolDefinition[] = [
	{ id: 'home', label: 'Home', description: 'Overview', category: 'Kelmor Director', path: '/', icon: 'home', capabilities: [] },
	{ id: 'create-account', label: 'Create a New Account', description: 'Provision', category: 'Account Functions', path: '/accounts/create', icon: 'plus', capabilities: [] },
	{ id: 'list-accounts', label: 'List Accounts', description: 'Inventory', category: 'Account Information', path: '/accounts', icon: 'users', capabilities: [] },
	{ id: 'packages', label: 'Edit a Package', description: 'Limits', category: 'Packages', path: '/packages', icon: 'box', capabilities: [] },
	{ id: 'dns', label: 'DNS Zone Manager', description: 'Zones', category: 'DNS Functions', path: '/dns', icon: 'globe', capabilities: [] },
	{ id: 'services', label: 'Service Status', description: 'Health', category: 'Server Status', path: '/status', icon: 'pulse', capabilities: [] },
	{ id: 'processes', label: 'Process Manager', description: 'Processes', category: 'System Health', path: '/processes', icon: 'pulse', capabilities: [] },
	{ id: 'jobs', label: 'Jobs', description: 'Queue', category: 'System Tools', path: '/jobs', icon: 'jobs', capabilities: [] },
	{ id: 'audit', label: 'Audit Trail', description: 'History', category: 'Security Center', path: '/audit', icon: 'audit', capabilities: [] },
	{ id: 'security', label: 'Security & Host Configuration', description: 'Firewall', category: 'Security Center', path: '/security', icon: 'shield', capabilities: [] },
	{ id: 'feature-showcase', label: 'Feature Showcase', description: 'Notes', category: 'cPanel', path: '/tools/feature-showcase', icon: 'box', capabilities: [] },
]

describe('operator navigation groups', () => {
	test('uses WHM-style categories grounded in Kelmor domains', () => {
		const groups = operatorNavGroups(sample)
		expect(groups.map((group) => group.label)).toEqual([
			'Account Functions',
			'Account Information',
			'Packages',
			'DNS',
			'Service / Server Status',
			'Jobs & Audit',
			'Security',
			'Kelmor Control',
		])
		expect(groups.find((group) => group.id === 'account-functions')?.tools.map((tool) => tool.id)).toEqual(['create-account'])
		expect(groups.find((group) => group.id === 'jobs-audit')?.tools.map((tool) => tool.id)).toEqual(['jobs', 'audit'])
	})

	test('places every catalog tool except Home in exactly one group', () => {
		const groups = operatorNavGroups(toolCatalog)
		const seen = groups.flatMap((group) => group.tools.map((tool) => tool.id))
		expect(seen).not.toContain('home')
		expect(new Set(seen).size).toBe(seen.length)
		expect(seen.length).toBe(toolCatalog.filter((tool) => tool.id !== 'home').length)
	})

	test('maps every catalog category without inventing a cPanel label', () => {
		for (const feature of whmFeatures) {
			const group = operatorGroupForTool({
				id: feature.id,
				label: feature.label,
				description: feature.description,
				category: feature.category,
				path: feature.path,
				icon: feature.icon,
				capabilities: feature.capabilities,
			})
			expect(group.label).not.toMatch(/cPanel|WHM|WebHost Manager/i)
			expect(group.id).toBeTruthy()
		}
	})
})

describe('home vitals', () => {
	test('does not invent metrics when a probe is missing', () => {
		expect(measuredVital(false, '0.40')).toBe('Not reported')
		expect(measuredVital(true, '0.40')).toBe('0.40')
		expect(failedJobDisplay(undefined)).toEqual({
			value: 'Not reported',
			detail: 'Open Jobs for history',
		})
		expect(failedJobDisplay(2)).toEqual({
			value: '2',
			detail: '2 failed jobs',
		})
	})
})
