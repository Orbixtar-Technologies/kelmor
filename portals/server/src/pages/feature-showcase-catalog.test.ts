import { describe, expect, test } from 'vitest'
import { assembleFeatureShowcase } from './feature-showcase-catalog'

describe('assembleFeatureShowcase', () => {
	test('builds package capabilities from feature sets and links real tools', () => {
		const entries = assembleFeatureShowcase({
			featureSets: [{
				id: 'fs-full', name: 'full-hosting',
				features: { websites: true, email: true, ssh: false },
			}],
			packages: [
				{ id: 'pkg-1', name: 'Starter', feature_set_id: 'fs-full' },
			],
			hostApps: [],
		})
		expect(entries.map((entry) => entry.id)).toEqual(['package:email', 'package:ssh', 'package:websites'])
		const websites = entries.find((entry) => entry.id === 'package:websites')
		expect(websites?.name).toBe('MultiPHP Manager')
		expect(websites?.enabled).toBe(true)
		expect(websites?.href).toBe('/websites')
		expect(websites?.detail).toMatch(/1 of 1 feature set/)
		const ssh = entries.find((entry) => entry.id === 'package:ssh')
		expect(ssh?.enabled).toBe(false)
		expect(ssh?.href).toBe('/section/accounts?tool=manage-shell')
		expect(ssh?.description).toMatch(/POSIX shell/)
	})

	test('includes host apps reported by the Agent', () => {
		const entries = assembleFeatureShowcase({
			featureSets: [],
			packages: [],
			hostApps: [{
				id: 'phpmyadmin',
				label: 'phpMyAdmin',
				status: 'installed',
				description: 'SQL browser installed from the host phpmyadmin package.',
			}],
		})
		expect(entries).toHaveLength(1)
		expect(entries[0]?.name).toBe('phpMyAdmin')
		expect(entries[0]?.href).toBe('/section/sql?tool=phpmyadmin')
		expect(entries[0]?.status).toBe('installed')
	})

	test('keeps unknown feature keys honest and points at Feature Manager', () => {
		const entries = assembleFeatureShowcase({
			featureSets: [{ id: 'fs-1', name: 'custom', features: { custom_flag: true } }],
			packages: [],
			hostApps: [],
		})
		expect(entries[0]?.name).toBe('custom flag')
		expect(entries[0]?.href).toBe('/features')
		expect(entries[0]?.description).toBe('Package capability from Feature Manager.')
	})

	test('returns no invented entries when every catalog is empty', () => {
		expect(assembleFeatureShowcase({ featureSets: [], packages: [], hostApps: [] })).toEqual([])
		expect(assembleFeatureShowcase({
			featureSets: [{ id: 'empty', name: 'empty', features: {} }],
			packages: [],
			hostApps: [],
		})).toEqual([])
	})
})
