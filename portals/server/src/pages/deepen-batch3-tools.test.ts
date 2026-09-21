import { describe, expect, test } from 'vitest'
import { hubForToolId, hrefForFeature } from '../nav-hubs'
import { featureById } from '../whm-catalog'

describe('deepen batch 3 catalog membership', () => {
	test('Firewall and CSF aliases belong to Security and open a real tool href', () => {
		const firewall = featureById('firewall')
		const csf = featureById('csf')
		expect(firewall, 'firewall').toBeDefined()
		expect(csf, 'csf').toBeDefined()
		expect(hubForToolId('firewall')?.id).toBe('security')
		expect(hubForToolId('csf')?.id).toBe('security')
		expect(hrefForFeature(firewall!)).toBe('/section/security?tool=firewall')
		expect(hrefForFeature(csf!)).toBe('/section/security?tool=csf')
		expect(firewall!.label).toBe('Firewall')
		expect(csf!.label).toBe('Firewall (CSF link)')
		expect(firewall!.description).toMatch(/nftables/i)
		expect(firewall!.description).not.toMatch(/\bCSF\b/)
		expect(csf!.description).toMatch(/not (the Kelmor stack|installed)|nftables/i)
		expect(csf!.description).not.toMatch(/ConfigServer Firewall is installed/i)
	})

	test('Redirects is a dedicated Software tool at /redirects', () => {
		const redirects = featureById('redirects')
		expect(redirects, 'redirects').toBeDefined()
		expect(hubForToolId('redirects')?.id).toBe('websites')
		expect(redirects!.dedicated).toBe(true)
		expect(redirects!.path).toBe('/redirects')
		expect(hrefForFeature(redirects!)).toBe('/redirects')
	})

	test('Hotlink belongs to Websites', () => {
		const hotlink = featureById('hotlink')
		expect(hotlink, 'hotlink').toBeDefined()
		expect(hubForToolId('hotlink')?.id).toBe('websites')
		expect(hrefForFeature(hotlink!)).toBe('/section/websites?tool=hotlink')
	})

	test('Image Manager belongs to Files', () => {
		const images = featureById('image-manager')
		expect(images, 'image-manager').toBeDefined()
		expect(hubForToolId('image-manager')?.id).toBe('files')
		expect(hrefForFeature(images!)).toBe('/section/files?tool=image-manager')
	})

	test('Git Version Control is a dedicated Files tool at /git', () => {
		const git = featureById('git')
		expect(git, 'git').toBeDefined()
		expect(hubForToolId('git')?.id).toBe('files')
		expect(git!.dedicated).toBe(true)
		expect(git!.path).toBe('/git')
		expect(hrefForFeature(git!)).toBe('/git')
	})
})
