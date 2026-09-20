import { describe, expect, test } from 'vitest'
import {
	HOST_SETTINGS_BANNER,
	LOCAL_SETTINGS_BANNER,
	LOCAL_SETTINGS_LABEL,
	QUOTA_PACKAGE_COPY,
	isHostSettingsFeature,
	isLocalSettingsFeature,
	isLocalSettingsToolId,
	localSettingsBadgeTitle,
} from './catalog-honesty'
import { featureById, whmFeatures } from './whm-catalog'

describe('catalog honesty', () => {
	test('labels only deferred policy records as local settings', () => {
		const deferred = whmFeatures.filter((feature) => isLocalSettingsFeature(feature))
		expect(deferred.map((feature) => feature.id).sort()).toEqual([])
		for (const feature of deferred) {
			expect(isLocalSettingsToolId(feature.id)).toBe(true)
			expect(localSettingsBadgeTitle(feature.label)).toContain(LOCAL_SETTINGS_LABEL)
		}
		for (const id of ['email', 'sql', 'file-manager', 'dns', 'ssl', 'ftp', 'cron-jobs']) {
			expect(isLocalSettingsToolId(id)).toBe(false)
			expect(featureById(id)?.dedicated).toBe(true)
		}
	})

	test('treats host-applied setting tiles as live, not local stubs', () => {
		expect(isHostSettingsFeature(featureById('tweak-settings')!)).toBe(true)
		expect(isLocalSettingsFeature(featureById('tweak-settings')!)).toBe(false)
		expect(isHostSettingsFeature(featureById('change-hostname')!)).toBe(true)
		expect(isHostSettingsFeature(featureById('spamd-startup')!)).toBe(true)
		expect(isHostSettingsFeature(featureById('server-profile')!)).toBe(true)
		expect(isLocalSettingsFeature(featureById('server-profile')!)).toBe(false)
		expect(isLocalSettingsToolId('module-installers')).toBe(false)
		expect(isLocalSettingsToolId('perl-modules')).toBe(false)
		expect(isLocalSettingsToolId('php-pear')).toBe(false)
		expect(isLocalSettingsToolId('php-pecl')).toBe(false)
		expect(isLocalSettingsToolId('ruby-gems')).toBe(false)
		expect(isLocalSettingsToolId('configuration-cluster')).toBe(false)
		expect(isLocalSettingsToolId('remote-access-key')).toBe(false)
		expect(isLocalSettingsToolId('grant-support-access')).toBe(false)
		expect(isHostSettingsFeature(featureById('external-auth')!)).toBe(true)
		expect(isHostSettingsFeature(featureById('two-factor')!)).toBe(true)
		expect(isHostSettingsFeature(featureById('link-nodes')!)).toBe(true)
		expect(isLocalSettingsToolId('external-auth')).toBe(false)
		expect(isLocalSettingsToolId('two-factor')).toBe(false)
		expect(isLocalSettingsToolId('link-nodes')).toBe(false)
		expect(isHostSettingsFeature(featureById('postgres-config')!)).toBe(true)
		expect(isLocalSettingsToolId('postgres-config')).toBe(false)
		expect(isLocalSettingsToolId('mysql-upgrade')).toBe(false)
		expect(featureById('postgres-config')?.fields?.some((field) => field.name === 'auth')).toBe(true)
		expect(featureById('mysql-upgrade')?.settingKey).toBeUndefined()
		expect(HOST_SETTINGS_BANNER).toMatch(/host apply job/i)
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

	test('banner copy does not claim host enforcement for deferred records', () => {
		expect(LOCAL_SETTINGS_BANNER).toMatch(/Not applied to host/i)
		expect(LOCAL_SETTINGS_BANNER).not.toMatch(/Agent enforces/i)
	})
})
