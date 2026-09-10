import { describe, expect, test } from 'vitest'
import { directorBreadcrumbs } from './breadcrumbs'

const labels = { jobs: 'Jobs', accounts: 'Accounts' }

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
})
