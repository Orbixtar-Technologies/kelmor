import { describe, expect, test } from 'vitest'
import { dedicatedToolPaths } from '../dedicated-tool-routes'
import { featureById } from '../whm-catalog'
import { directorBreadcrumbs, directorCrumbLabels } from './breadcrumbs'

const labels = { jobs: 'Jobs', accounts: 'Accounts' }

function lastSegmentCrumbLabels (): Record<string, string> {
	return {
		packages: 'Packages',
		...Object.fromEntries(Object.entries(dedicatedToolPaths).map(([id, path]) => {
			const segment = path.split('/').pop() || id
			return [segment, featureById(id)?.label || segment]
		})),
	}
}

describe('directorBreadcrumbs', () => {
	test('makes account ancestors navigable', () => {
		expect(directorBreadcrumbs('/accounts/acc-1/services', labels, 'freshhost')).toEqual([
			{ label: 'Home', to: '/' },
			{ label: 'Accounts', to: '/accounts' },
			{ label: 'freshhost', to: '/accounts/acc-1' },
			{ label: 'Services' },
		])
	})

	test('keeps the current jobs crumb as plain text', () => {
		expect(directorBreadcrumbs('/jobs', labels)).toEqual([
			{ label: 'Home', to: '/' },
			{ label: 'Jobs' },
		])
	})

	test('appends the selected tool account after the tool crumb', () => {
		expect(directorBreadcrumbs('/dns', { dns: 'DNS Management' }, undefined, 'kelmor-demo')).toEqual([
			{ label: 'Home', to: '/' },
			{ label: 'DNS Management' },
			{ label: 'kelmor-demo' },
		])
	})

	test('labels generic WHM tool paths from the catalog', () => {
		expect(directorBreadcrumbs('/tools/tweak-settings', { tools: 'Tools', 'tweak-settings': 'Tweak Settings' })).toEqual([
			{ label: 'Home', to: '/' },
			{ label: 'Tools', to: '/tools' },
			{ label: 'Tweak Settings' },
		])
	})

	test('labels combined hub pages from the hub map', () => {
		expect(directorBreadcrumbs('/section/server', { section: 'Section', server: 'server' })).toEqual([
			{ label: 'Home', to: '/' },
			{ label: 'Server Configuration' },
		])
	})

	test('package add/delete breadcrumbs do not reuse DNS Zone labels', () => {
		const add = directorBreadcrumbs('/packages/add', directorCrumbLabels)
		const remove = directorBreadcrumbs('/packages/delete', directorCrumbLabels)
		const addTrail = add.map((crumb) => crumb.label).join(' / ')
		const deleteTrail = remove.map((crumb) => crumb.label).join(' / ')

		expect(addTrail).not.toMatch(/DNS Zone/)
		expect(deleteTrail).not.toMatch(/DNS Zone/)
		expect(add.at(-1)?.label).toBe('Add a Package')
		expect(remove.at(-1)?.label).toBe('Delete a Package')
		expect(directorBreadcrumbs('/domains/add', directorCrumbLabels).at(-1)?.label).toBe('Add a DNS Zone')
		expect(directorBreadcrumbs('/domains/delete', directorCrumbLabels).at(-1)?.label).toBe('Delete a DNS Zone')
	})

	test('keeps package crumbs distinct when last-segment labels collide', () => {
		const colliding = lastSegmentCrumbLabels()
		expect(colliding.add).toBe('Add a DNS Zone')
		expect(directorBreadcrumbs('/packages/add', colliding).at(-1)?.label).toBe('Add a Package')
		expect(directorBreadcrumbs('/packages/delete', colliding).at(-1)?.label).toBe('Delete a Package')
	})

	test('dedicated tool crumbs keep colliding last segments distinct', () => {
		expect(directorBreadcrumbs('/accounts/password', directorCrumbLabels).at(-1)?.label).toBe('Password Modification')
		expect(directorBreadcrumbs('/sql/password', directorCrumbLabels).at(-1)?.label).toBe('Change Database User Password')
		expect(directorBreadcrumbs('/status', directorCrumbLabels).at(-1)?.label).toBe('Service Status')
		expect(directorBreadcrumbs('/ssl/status', directorCrumbLabels).at(-1)?.label).toBe('SSL/TLS Status')
		expect(directorBreadcrumbs('/usage', directorCrumbLabels).at(-1)?.label).toBe('Account Usage')
		expect(directorBreadcrumbs('/resellers/usage', directorCrumbLabels).at(-1)?.label).toBe('View Reseller Usage and Manage Account Status')
		expect(directorBreadcrumbs('/processes', directorCrumbLabels).at(-1)?.label).toBe('Process Manager')
		expect(directorBreadcrumbs('/sql/processes', directorCrumbLabels).at(-1)?.label).toBe('Show MySQL Processes')
	})

	test('update preference and changelog crumbs stay exclusive of Software Updates', () => {
		expect(directorBreadcrumbs('/updates', directorCrumbLabels)).toEqual([
			{ label: 'Home', to: '/' },
			{ label: 'Software Updates' },
		])
		expect(directorBreadcrumbs('/updates/preferences', directorCrumbLabels)).toEqual([
			{ label: 'Home', to: '/' },
			{ label: 'Software Updates', to: '/updates' },
			{ label: 'Update Preferences' },
		])
		expect(directorBreadcrumbs('/updates/changelog', directorCrumbLabels)).toEqual([
			{ label: 'Home', to: '/' },
			{ label: 'Software Updates', to: '/updates' },
			{ label: 'Change Log' },
		])
	})
})
