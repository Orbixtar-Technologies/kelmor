import { describe, expect, test } from 'vitest'
import { groupAccountServices, resourceManagePath, resourcePrimaryLabel, serviceActionLabel } from './account-service-groups'

const services = [
	{ id: 'websites', label: 'Websites' },
	{ id: 'domains', label: 'Domains' },
	{ id: 'databases', label: 'Databases' },
	{ id: 'mailboxes', label: 'Mailboxes' },
	{ id: 'aliases', label: 'Aliases' },
	{ id: 'certificates', label: 'Certificates' },
	{ id: 'files', label: 'Files' },
	{ id: 'backups', label: 'Backups & restore' },
	{ id: 'cron', label: 'Cron' },
	{ id: 'ssh', label: 'SSH / SFTP' },
	{ id: 'ftp', label: 'FTP' },
	{ id: 'tokens', label: 'API tokens' },
	{ id: 'applications', label: 'Applications' },
	{ id: 'mail-domains', label: 'Mail domains' },
]

describe('groupAccountServices', () => {
	test('groups services into Web, Email, Data, Access, and Automation', () => {
		const groups = groupAccountServices(services)
		expect(groups.map((group) => group.label)).toEqual(['Web', 'Email', 'Data', 'Access', 'Automation'])
		expect(groups.find((group) => group.id === 'web')?.items.map((item) => item.id)).toEqual([
			'websites', 'domains', 'certificates', 'applications',
		])
		expect(groups.find((group) => group.id === 'email')?.items.map((item) => item.id)).toEqual([
			'mail-domains', 'mailboxes', 'aliases',
		])
	})

	test('omits empty groups when a role cannot see those services', () => {
		const groups = groupAccountServices(services.filter((service) => service.id === 'databases' || service.id === 'cron'))
		expect(groups.map((group) => group.id)).toEqual(['data', 'automation'])
	})
})

describe('resource and action labels', () => {
	test('prefers a human name for existing resources', () => {
		expect(resourcePrimaryLabel('databases', { id: 'db-1', name: 'shop_db' })).toBe('shop_db')
		expect(resourcePrimaryLabel('websites', { id: 'web-1', hostname: 'shop.example.com' })).toBe('shop.example.com')
		expect(resourcePrimaryLabel('mailboxes', { id: 'mb-1', local_part: 'info' })).toBe('info')
	})

	test('uses create-oriented labels instead of Apply website', () => {
		expect(serviceActionLabel('websites')).toBe('Create website')
		expect(serviceActionLabel('databases')).toBe('Create database')
		expect(serviceActionLabel('backups')).toBe('Queue encrypted backup')
	})

	test('points existing resources at a dedicated management route', () => {
		expect(resourceManagePath('databases', 'acc-1')).toBe('/sql?account=acc-1')
		expect(resourceManagePath('certificates', 'acc-1')).toBe('/ssl?account=acc-1')
		expect(resourceManagePath('mailboxes', 'acc-1')).toBe('/email?account=acc-1&tab=mailboxes')
		expect(resourceManagePath('websites', 'acc-1')).toBe('/websites?account=acc-1')
		expect(resourceManagePath('domains', 'acc-1')).toBe('/domains?account=acc-1')
		expect(resourceManagePath('cron', 'acc-1')).toBe('/cron?account=acc-1')
		expect(resourceManagePath('ftp', 'acc-1')).toBe('/ftp?account=acc-1')
	})
})
