import { describe, expect, it } from 'vitest'
import { addonDomainsFrom, convertAddonDomainsHref, domainFqdn } from './convert-addon'

describe('addonDomainsFrom', () => {
	it('keeps only real addon domains from the account inventory', () => {
		const items = [
			{ id: 'd1', ascii_fqdn: 'alpha.test', type: 'primary' },
			{ id: 'd2', ascii_fqdn: 'shop.alpha.test', type: 'addon' },
			{ id: 'd3', ascii_fqdn: 'park.alpha.test', type: 'alias' },
			{ id: 'd4', ascii_fqdn: 'blog.alpha.test', type: 'subdomain' },
		]
		expect(addonDomainsFrom(items).map(domainFqdn)).toEqual(['shop.alpha.test'])
	})

	it('does not invent addon domains when the account only has a primary', () => {
		expect(addonDomainsFrom([{ id: 'd1', ascii_fqdn: 'alpha.test', type: 'primary' }])).toEqual([])
	})
})

describe('convertAddonDomainsHref', () => {
	it('opens List Domains scoped to the account addon view', () => {
		expect(convertAddonDomainsHref('acc-1')).toBe('/domains?account=acc-1&view=addon')
	})
})
