import { describe, expect, test, vi } from 'vitest'
import { assignOpenedWindow, controlImpersonationUrl, controlPortalOrigin, preferredControlHostname } from './control-url'

describe('controlPortalOrigin', () => {
	test('maps Director Vite preview to Control Vite preview', () => {
		expect(controlPortalOrigin({ protocol: 'http:', hostname: '127.0.0.1', port: '18443' })).toBe('http://127.0.0.1:18444')
	})

	test('maps installed Director TLS to installed Control TLS', () => {
		expect(controlPortalOrigin({ protocol: 'https:', hostname: 'host.example.net', port: '2087' })).toBe('https://host.example.net:2083')
	})

	test('does not keep a legacy Director port such as 8443', () => {
		expect(controlPortalOrigin({ protocol: 'https:', hostname: '150.239.113.59', port: '8443' })).toBe('https://150.239.113.59:2083')
	})

	test('prefers an advertised public hostname over a browser IP', () => {
		expect(controlPortalOrigin(
			{ protocol: 'https:', hostname: '150.239.113.59', port: '8443' },
			{ hostname: 'kelmor.host' },
		)).toBe('https://kelmor.host:2083')
	})

	test('uses the API-advertised Control origin', () => {
		expect(controlPortalOrigin(
			{ protocol: 'https:', hostname: '150.239.113.59', port: '8443' },
			{ controlUrl: 'https://kelmor.host:2083/' },
		)).toBe('https://kelmor.host:2083')
	})

	test('upgrades an HTTP advertised Control origin on a public host', () => {
		expect(controlPortalOrigin(
			{ protocol: 'https:', hostname: 'kelmor.host', port: '2087' },
			{ controlUrl: 'http://kelmor.host:2083/' },
		)).toBe('https://kelmor.host:2083')
	})
})

describe('preferredControlHostname', () => {
	test('keeps a public browser hostname when no config is present', () => {
		expect(preferredControlHostname('kelmor.host', '')).toBe('kelmor.host')
	})
})

describe('controlImpersonationUrl', () => {
	test('puts the session token in the fragment', () => {
		expect(controlImpersonationUrl('https://host.example.net:2083', 'tok/en')).toBe('https://host.example.net:2083/#session=tok%2Fen')
	})
})

describe('assignOpenedWindow', () => {
	test('writes the URL into a window opened during the click', () => {
		const popup = { closed: false, opener: window, location: { replace: vi.fn() } }
		assignOpenedWindow(popup as unknown as Window, 'https://kelmor.host:2083/#session=tok')
		expect(popup.opener).toBeNull()
		expect(popup.location.replace).toHaveBeenCalledWith('https://kelmor.host:2083/#session=tok')
	})
})
