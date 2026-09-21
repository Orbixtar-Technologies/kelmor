import { describe, expect, test } from 'vitest'
import {
	canonicalToolHref,
	hubById,
	hubEntryPath,
	hubForCategory,
	hubForLocation,
	hubForToolId,
	hrefForFeature,
	isDirectorHubActive,
	misplacedSectionRedirect,
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
		expect(hrefForFeature(featureById('ip-migration')!)).toBe('/section/server?tool=ip-migration')
		expect(hrefForFeature(featureById('file-dir-restore')!)).toBe('/section/backups?tool=file-dir-restore')
	})

	test('redirects a tool query on the wrong section to its owner hub', () => {
		expect(canonicalToolHref('ip-migration')).toBe('/section/server?tool=ip-migration')
		expect(canonicalToolHref('file-dir-restore')).toBe('/section/backups?tool=file-dir-restore')
		expect(canonicalToolHref('list-accounts')).toBe('/accounts')
		expect(canonicalToolHref('missing-tool')).toBeNull()
		expect(misplacedSectionRedirect('accounts', 'ip-migration')).toBe('/section/server?tool=ip-migration')
		expect(misplacedSectionRedirect('accounts', 'file-dir-restore')).toBe('/section/backups?tool=file-dir-restore')
		expect(misplacedSectionRedirect('dns', 'ip-migration')).toBe('/section/server?tool=ip-migration')
		expect(misplacedSectionRedirect('server', 'ip-migration')).toBeNull()
		expect(misplacedSectionRedirect('backups', 'file-dir-restore')).toBeNull()
		expect(misplacedSectionRedirect('accounts', 'change-site-ip')).toBeNull()
		expect(misplacedSectionRedirect('accounts', 'list-accounts')).toBeNull()
		expect(misplacedSectionRedirect('accounts', 'not-a-real-tool')).toBeNull()
	})

	test('every catalog tool on a foreign section resolves to its canonical href', () => {
		for (const feature of whmFeatures) {
			const owner = hubForToolId(feature.id)
			expect(owner, feature.id).toBeDefined()
			expect(canonicalToolHref(feature.id)).toBe(hrefForFeature(feature))
			expect(misplacedSectionRedirect(owner!.id, feature.id)).toBeNull()
			for (const hub of navHubs) {
				if (hub.id === 'home' || hub.id === owner!.id) continue
				expect(misplacedSectionRedirect(hub.id, feature.id), `${feature.id} on ${hub.id}`).toBe(hrefForFeature(feature))
			}
		}
	})

	test('resolves the current hub from dedicated, section, and tool paths', () => {
		expect(hubForLocation('/')?.id).toBe('home')
		expect(hubForLocation('/accounts')?.id).toBe('accounts')
		expect(hubForLocation('/accounts/acc-1')?.id).toBe('accounts')
		expect(hubForLocation('/section/server', '?tool=tweak-settings')?.id).toBe('server')
		expect(hubForLocation('/tools/tweak-settings')?.id).toBe('server')
		expect(hubForLocation('/ssl', '?task=request')?.id).toBe('ssl')
		expect(hubForLocation('/jobs')?.id).toBe('system')
		expect(hubForLocation('/jobs', '?q=mail')?.id).toBe('system')
		expect(hubForLocation('/mail/delivery-reports')?.id).toBe('email')
		expect(hubForLocation('/mail/track-delivery')?.id).toBe('email')
		expect(hubForLocation('/ip-usage')?.id).toBe('server')
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

	test('keeps sibling hub tabs on catalog section landings only', () => {
		expect(shouldShowHubTabs('/section/server')).toBe(true)
		expect(shouldShowHubTabs('/section/accounts')).toBe(true)
		expect(shouldShowHubTabs('/section/packages')).toBe(true)
		expect(shouldShowHubTabs('/section/accounts', '?tool=change-site-ip')).toBe(false)
		expect(shouldShowHubTabs('/section/packages', '?tool=email-resellers')).toBe(false)
		expect(shouldShowHubTabs('/section/server', '?tool=tweak-settings')).toBe(false)
		expect(shouldShowHubTabs('/section/accounts', 'tool=limit-bandwidth')).toBe(false)
		expect(shouldShowHubTabs('/accounts')).toBe(false)
		expect(shouldShowHubTabs('/accounts/create')).toBe(false)
		expect(shouldShowHubTabs('/accounts/acc-1')).toBe(false)
		expect(shouldShowHubTabs('/packages')).toBe(false)
		expect(shouldShowHubTabs('/jobs')).toBe(false)
		expect(shouldShowHubTabs('/')).toBe(false)
	})
})
