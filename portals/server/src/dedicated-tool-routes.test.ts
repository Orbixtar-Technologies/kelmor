import { describe, expect, test } from 'vitest'
import {
	ACCOUNT_TOOL_SEGMENTS,
	dedicatedPath,
	dedicatedToolPaths,
	isAccountDetailPath,
	legacyDedicatedRedirect,
} from './dedicated-tool-routes'
import { hrefForFeature } from './nav-hubs'
import { isDirectorToolActive } from './layout/nav-active'
import { featureById, toToolDefinition, whmFeatures } from './whm-catalog'

describe('dedicated tool routes', () => {
	test('Change Ownership has a dedicated path, not the Accounts list hub', () => {
		expect(featureById('change-ownership')?.path).toBe('/accounts/ownership')
		expect(hrefForFeature(featureById('change-ownership')!)).toBe('/accounts/ownership')
		expect(featureById('change-ownership')?.path).not.toContain('task=')
	})

	test.each(Object.entries(dedicatedToolPaths))('%s opens %s without a list-hub query flag', (id, path) => {
		const feature = featureById(id)
		expect(feature, id).toBeDefined()
		expect(feature?.path).toBe(path)
		expect(hrefForFeature(feature!)).toBe(path)
		expect(path.includes('?')).toBe(false)
	})

	test('sidebar Change Ownership is exclusively active on its page', () => {
		const ownership = toToolDefinition(featureById('change-ownership')!)
		const list = toToolDefinition(featureById('list-accounts')!)
		const modify = toToolDefinition(featureById('modify-account')!)
		const summary = toToolDefinition(featureById('account-summary')!)
		expect(isDirectorToolActive(ownership, '/accounts/ownership')).toBe(true)
		expect(isDirectorToolActive(list, '/accounts/ownership')).toBe(false)
		expect(isDirectorToolActive(modify, '/accounts/ownership')).toBe(false)
		expect(isDirectorToolActive(summary, '/accounts/ownership')).toBe(false)
		expect(isDirectorToolActive(ownership, '/accounts')).toBe(false)
		expect(isDirectorToolActive(ownership, '/accounts', '?task=ownership')).toBe(false)
	})

	test('account-function pages do not steal Account Summary highlight', () => {
		const summary = toToolDefinition(featureById('account-summary')!)
		const modify = toToolDefinition(featureById('modify-account')!)
		expect(isDirectorToolActive(summary, '/accounts/summary')).toBe(true)
		expect(isDirectorToolActive(summary, '/accounts/acc-1')).toBe(true)
		expect(isDirectorToolActive(summary, '/accounts/modify')).toBe(false)
		expect(isDirectorToolActive(modify, '/accounts/modify')).toBe(true)
		expect(isDirectorToolActive(modify, '/accounts/acc-1')).toBe(false)
	})

	test('legacy list-hub query URLs redirect onto dedicated pages', () => {
		expect(legacyDedicatedRedirect('/accounts', '?task=ownership')).toBe('/accounts/ownership')
		expect(legacyDedicatedRedirect('/accounts', '?task=ownership&account=acc-1')).toBe('/accounts/ownership?account=acc-1')
		expect(legacyDedicatedRedirect('/accounts', '?task=password&mode=force')).toBe('/accounts/force-password')
		expect(legacyDedicatedRedirect('/packages', '?task=add')).toBe('/packages/add')
		expect(legacyDedicatedRedirect('/ssl', '?account=acc-1&task=request')).toBe('/ssl/request?account=acc-1')
		expect(legacyDedicatedRedirect('/accounts')).toBeNull()
		expect(legacyDedicatedRedirect('/accounts', '?view=suspended')).toBeNull()
	})

	test('account detail paths stay distinct from dedicated tool segments', () => {
		expect(isAccountDetailPath('/accounts/acc-1')).toBe(true)
		expect(isAccountDetailPath('/accounts/create')).toBe(false)
		expect(isAccountDetailPath('/accounts/ownership')).toBe(false)
		expect(isAccountDetailPath('/accounts')).toBe(false)
		expect(ACCOUNT_TOOL_SEGMENTS.has('ownership')).toBe(true)
	})

	test('Mail Delivery Reports and Track Delivery stay on their dedicated routes', () => {
		expect(featureById('mail-delivery-reports')?.path).toBe('/mail/delivery-reports')
		expect(featureById('track-delivery')?.path).toBe('/mail/track-delivery')
		expect(hrefForFeature(featureById('mail-delivery-reports')!)).toBe('/mail/delivery-reports')
		expect(hrefForFeature(featureById('track-delivery')!)).toBe('/mail/track-delivery')
	})

	test('DNS cleanup and synchronize stay exclusive of DNS Zone Manager', () => {
		const zoneManager = toToolDefinition(featureById('dns')!)
		const cleanup = toToolDefinition(featureById('dns-cleanup')!)
		const sync = toToolDefinition(featureById('synchronize-dns')!)
		expect(featureById('dns-cleanup')?.path).toBe('/dns/cleanup')
		expect(featureById('synchronize-dns')?.path).toBe('/dns/synchronize')
		expect(hrefForFeature(featureById('dns-cleanup')!)).toBe('/dns/cleanup')
		expect(hrefForFeature(featureById('synchronize-dns')!)).toBe('/dns/synchronize')
		expect(isDirectorToolActive(cleanup, '/dns/cleanup')).toBe(true)
		expect(isDirectorToolActive(sync, '/dns/synchronize')).toBe(true)
		expect(isDirectorToolActive(zoneManager, '/dns/cleanup')).toBe(false)
		expect(isDirectorToolActive(zoneManager, '/dns/synchronize')).toBe(false)
		expect(isDirectorToolActive(cleanup, '/dns')).toBe(false)
		expect(isDirectorToolActive(sync, '/dns')).toBe(false)
		expect(isDirectorToolActive(cleanup, '/dns/synchronize')).toBe(false)
		expect(isDirectorToolActive(sync, '/dns/cleanup')).toBe(false)
		expect(isDirectorToolActive(cleanup, '/mail/delivery-reports')).toBe(false)
		expect(isDirectorToolActive(sync, '/mail/track-delivery')).toBe(false)
	})

	test('converted dedicated tools keep unique catalog hrefs', () => {
		const converted = Object.keys(dedicatedToolPaths).map((id) => hrefForFeature(featureById(id)!))
		expect(new Set(converted).size).toBe(converted.length)
		const dedicated = whmFeatures.filter((feature) => feature.dedicated).map((feature) => hrefForFeature(feature))
		expect(new Set(dedicated).size).toBe(dedicated.length)
	})

	test('dedicatedPath helper matches the catalog', () => {
		expect(dedicatedPath('change-ownership')).toBe(featureById('change-ownership')?.path)
	})

	test('Update Preferences and Change Log stay exclusive of Software Updates', () => {
		const updates = toToolDefinition(featureById('updates')!)
		const preferences = toToolDefinition(featureById('update-preferences')!)
		const changelog = toToolDefinition(featureById('change-log')!)
		expect(isDirectorToolActive(updates, '/updates')).toBe(true)
		expect(isDirectorToolActive(updates, '/updates/preferences')).toBe(false)
		expect(isDirectorToolActive(updates, '/updates/changelog')).toBe(false)
		expect(isDirectorToolActive(preferences, '/updates/preferences')).toBe(true)
		expect(isDirectorToolActive(preferences, '/updates')).toBe(false)
		expect(isDirectorToolActive(preferences, '/updates/changelog')).toBe(false)
		expect(isDirectorToolActive(changelog, '/updates/changelog')).toBe(true)
		expect(isDirectorToolActive(changelog, '/updates')).toBe(false)
		expect(isDirectorToolActive(changelog, '/updates/preferences')).toBe(false)
	})

	test('intentionally hub-deferred tools stay list, tab, or already-dedicated views', () => {
		expect(featureById('list-accounts')?.path).toBe('/accounts')
		expect(featureById('suspended')?.path).toBe('/accounts?view=suspended')
		expect(featureById('over-quota')?.path).toBe('/accounts?view=over-quota')
		expect(featureById('list-domains')?.path).toBe('/domains')
		expect(featureById('list-subdomains')?.path).toBe('/domains?view=subdomain')
		expect(featureById('list-parked')?.path).toBe('/domains?view=alias')
		expect(featureById('email')?.path).toMatch(/^\/email/)
		expect(featureById('mailman')?.path).toBe('/email?tab=lists')
		expect(featureById('reset-mailman')?.path).toBe('/email?tab=lists&task=reset')
		expect(featureById('review-transfers')?.path).toBe('/jobs?q=transfer')
		expect(featureById('mail-delivery-reports')?.path).toBe('/mail/delivery-reports')
		expect(featureById('track-delivery')?.path).toBe('/mail/track-delivery')
	})
})
