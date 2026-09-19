import { describe, expect, test } from 'vitest'
import {
	LOCAL_SETTINGS_BANNER,
	LOCAL_SETTINGS_LABEL,
	QUOTA_PACKAGE_COPY,
	isLocalSettingsFeature,
	isLocalSettingsToolId,
	localSettingsBadgeTitle,
} from './catalog-honesty'
import { featureById, whmFeatures } from './whm-catalog'

describe('catalog honesty', () => {
	test('labels every settings-stub tool and leaves dedicated managers unlabeled', () => {
		const stubs = whmFeatures.filter((feature) => feature.settingKey)
		expect(stubs.length).toBeGreaterThan(40)
		for (const feature of stubs) {
			expect(isLocalSettingsFeature(feature)).toBe(true)
			expect(isLocalSettingsToolId(feature.id)).toBe(true)
			expect(localSettingsBadgeTitle(feature.label)).toContain(LOCAL_SETTINGS_LABEL)
		}
		for (const id of ['email', 'sql', 'file-manager', 'dns', 'ssl', 'ftp', 'cron-jobs']) {
			expect(isLocalSettingsToolId(id)).toBe(false)
			expect(featureById(id)?.dedicated).toBe(true)
		}
	})

	test('does not treat quota or bandwidth tiles as preference stubs', () => {
		expect(featureById('quota-modification')?.settingKey).toBeUndefined()
		expect(featureById('limit-bandwidth')?.settingKey).toBeUndefined()
		expect(featureById('reset-bandwidth')?.settingKey).toBeUndefined()
		expect(featureById('quota-modification')?.accountAction).toBe('patch')
		expect(featureById('limit-bandwidth')?.accountAction).toBe('patch')
		expect(featureById('quota-modification')?.description).toContain('package')
		expect(featureById('quota-modification')?.description).not.toMatch(/override disk quota for one account/i)
		expect(QUOTA_PACKAGE_COPY).toMatch(/no separate per-account override/i)
	})

	test('banner copy does not claim host enforcement', () => {
		expect(LOCAL_SETTINGS_BANNER).toMatch(/Not applied to host/i)
		expect(LOCAL_SETTINGS_BANNER).not.toMatch(/Agent enforces/i)
	})
})
