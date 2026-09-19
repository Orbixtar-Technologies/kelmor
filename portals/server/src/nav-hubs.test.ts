import { describe, expect, test } from 'vitest'
import {
	hubById,
	hubEntryPath,
	hubForCategory,
	hubForLocation,
	hubForToolId,
	hrefForFeature,
	isDirectorHubActive,
	navHubs,
	shouldShowHubTabs,
	sidebarToolGroups,
	visibleHubs,
} from './nav-hubs'
import { toolCatalog } from './tool-catalog'
import { featureById, whmFeatures } from './whm-catalog'

describe('nav hubs', () => {
	test('maps every catalog category to exactly one hub', () => {
		const categories = [...new Set(whmFeatures.map((feature) => feature.category))]
		for (const category of categories) {
			expect(hubForCategory(category), `missing hub for ${category}`).toBeDefined()
		}
		expect(navHubs.map((hub) => hub.id)).toEqual([...new Set(navHubs.map((hub) => hub.id))])
	})

	test('places every feature on one combined hub page', () => {
		for (const feature of whmFeatures) {
			expect(hubForToolId(feature.id)?.categories).toContain(feature.category)
		}
	})

	test('keeps Server Configuration tools on one combined page', () => {
		const server = hubById('server')
		expect(server?.label).toBe('Server Configuration')
		expect(hubEntryPath(server!)).toBe('/section/server')
		expect(hrefForFeature(featureById('tweak-settings')!)).toBe('/section/server?tool=tweak-settings')
		expect(hrefForFeature(featureById('change-hostname')!)).toBe('/section/server?tool=change-hostname')
		expect(hrefForFeature(featureById('basic-setup')!)).toBe('/section/server?tool=basic-setup')
	})

	test('lists every catalog tool in the same groups as Home All tools', () => {
		const grouped = sidebarToolGroups(toolCatalog)
		const listed = grouped.flatMap((group) => group.tools.map((tool) => tool.id))
		const expected = toolCatalog.filter((tool) => tool.id !== 'home').map((tool) => tool.id)
		expect(listed.sort()).toEqual([...expected].sort())
		expect(grouped.find((group) => group.hub.id === 'server')?.tools.map((tool) => tool.id)).toEqual(
			expect.arrayContaining(['tweak-settings', 'change-hostname', 'basic-setup', 'contact-manager']),
		)
	})

	test('keeps dedicated managers as the hub entry when they already have a page', () => {
		expect(hubEntryPath(hubById('accounts')!)).toBe('/accounts')
		expect(hubEntryPath(hubById('email')!)).toBe('/email')
		expect(hrefForFeature(featureById('list-accounts')!)).toBe('/accounts')
		expect(hrefForFeature(featureById('limit-bandwidth')!)).toBe('/section/accounts?tool=limit-bandwidth')
	})

	test('resolves the current hub from dedicated, section, and tool paths', () => {
		expect(hubForLocation('/')?.id).toBe('home')
		expect(hubForLocation('/accounts')?.id).toBe('accounts')
		expect(hubForLocation('/accounts/acc-1')?.id).toBe('accounts')
		expect(hubForLocation('/section/server', '?tool=tweak-settings')?.id).toBe('server')
		expect(hubForLocation('/tools/tweak-settings')?.id).toBe('server')
		expect(hubForLocation('/ssl', '?task=request')?.id).toBe('ssl')
		expect(hubForLocation('/jobs')?.id).toBe('system')
		expect(hubForLocation('/jobs', '?q=mail')?.id).toBe('email')
	})

	test('highlights only one sidebar hub at a time', () => {
		expect(isDirectorHubActive(hubById('accounts')!, '/accounts/acc-1')).toBe(true)
		expect(isDirectorHubActive(hubById('packages')!, '/accounts/acc-1')).toBe(false)
		expect(isDirectorHubActive(hubById('server')!, '/section/server', '?tool=tweak-settings')).toBe(true)
		expect(isDirectorHubActive(hubById('security')!, '/section/server', '?tool=tweak-settings')).toBe(false)
	})

	test('lists every hub that still has catalog tools', () => {
		expect(visibleHubs(toolCatalog).map((hub) => hub.id)).toEqual(navHubs.map((hub) => hub.id))
	})

	test('keeps sibling hub tabs on catalog section pages only', () => {
		expect(shouldShowHubTabs('/section/server')).toBe(true)
		expect(shouldShowHubTabs('/section/accounts')).toBe(true)
		expect(shouldShowHubTabs('/accounts')).toBe(false)
		expect(shouldShowHubTabs('/accounts/create')).toBe(false)
		expect(shouldShowHubTabs('/accounts/acc-1')).toBe(false)
		expect(shouldShowHubTabs('/jobs')).toBe(false)
		expect(shouldShowHubTabs('/')).toBe(false)
	})
})
