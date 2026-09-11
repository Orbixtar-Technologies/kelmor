import { describe, expect, test } from 'vitest'
import { accountTaskTarget, discoverTools, toolCatalog } from './tool-catalog'
import { featureById, whmFeatures } from './whm-catalog'

describe('tool discovery', () => {
	test('exposes the entire WHM-mapped catalog regardless of capabilities', () => {
		const tools = discoverTools(toolCatalog, { 'accounts.read': true })

		expect(tools.length).toBe(toolCatalog.length)
		expect(tools.length).toBe(whmFeatures.length)
		expect(tools.some((tool) => tool.label === 'List Accounts')).toBe(true)
		expect(tools.some((tool) => tool.id === 'packages')).toBe(true)
		expect(tools.some((tool) => tool.label === 'Service Status')).toBe(true)
		expect(tools.some((tool) => tool.id === 'tweak-settings')).toBe(true)
		expect(tools.some((tool) => tool.id === 'terminal')).toBe(true)
	})

	test('covers the primary operator tool families', () => {
		expect(toolCatalog.map((tool) => tool.label)).toEqual(expect.arrayContaining([
			'Account Summary',
			'Modify an Account',
			'Upgrade/Downgrade an Account',
			'Manage Account Suspension',
			'Terminate Accounts',
			'Force Password Change',
			'Database Manager',
			'Email Management',
			'SSL / TLS',
			'File Manager',
			'Webmail',
		]))
	})

	test('includes WHM-mapped first-class tools', () => {
		expect(toolCatalog.map((tool) => tool.label)).toEqual(expect.arrayContaining([
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
		['cron-jobs', ['accounts.read', 'cron.read']],
		['deliverability', ['accounts.read', 'mail.read', 'dns.read']],
	] as const)('still declares every capability for the %s tool', (toolId, requiredCapabilities) => {
		const tool = toolCatalog.find((entry) => entry.id === toolId)
		expect(tool?.capabilities).toEqual(requiredCapabilities)
		expect(discoverTools(toolCatalog, {}).some((entry) => entry.id === toolId)).toBe(true)
	})

	test('routes account service tools to dedicated hub pages', () => {
		expect(accountTaskTarget('databases', 'account-1')).toBe('/sql?account=account-1')
		expect(accountTaskTarget('email', 'account-1')).toBe('/email?account=account-1&tab=mailboxes')
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

describe('WHM catalog', () => {
	test('has unique ids and enough interfaces to cover the WHM panel', () => {
		const ids = whmFeatures.map((feature) => feature.id)
		expect(new Set(ids).size).toBe(ids.length)
		expect(ids.length).toBeGreaterThanOrEqual(140)
	})

	test('gives every generic tool a /tools/:id path and a workflow', () => {
		for (const feature of whmFeatures) {
			expect(feature.steps.length).toBeGreaterThan(0)
			expect(feature.path).toMatch(/^\//)
			if (!feature.dedicated && feature.path.startsWith('/tools/')) {
				expect(feature.path).toBe(`/tools/${feature.id}`)
				expect(featureById(feature.id)?.label).toBe(feature.label)
			}
		}
	})

	test('routes SSL family tools to SSL manager tasks instead of Account Services', () => {
		expect(featureById('ssl')?.path).toBe('/ssl')
		expect(featureById('generate-csr')?.path).toBe('/ssl?task=request')
		expect(featureById('install-ssl')?.path).toBe('/ssl?task=request')
		expect(featureById('manage-autossl')?.path).toBe('/ssl?task=autossl')
		expect(featureById('ssl-storage')?.path).toBe('/ssl?task=inventory')
		expect(featureById('ssl-tls-status')?.path).toBe('/ssl?task=status')
		expect(featureById('service-ssl')?.path).toBe('/ssl?task=service')
		for (const feature of whmFeatures.filter((entry) => entry.category === 'SSL/TLS' || entry.id === 'service-ssl')) {
			expect(feature.path).not.toContain('/services')
			expect(feature.path).not.toBe('/tools/manage-autossl')
		}
	})
})
