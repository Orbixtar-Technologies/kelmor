import { describe, expect, test } from 'vitest'
import { controlImpersonationUrl, controlPortalOrigin } from './control-url'

describe('controlPortalOrigin', () => {
	test('maps Director Vite preview to Control Vite preview', () => {
		expect(controlPortalOrigin({ protocol: 'http:', hostname: '127.0.0.1', port: '18443' })).toBe('http://127.0.0.1:18444')
	})

	test('maps installed Director TLS to installed Control TLS', () => {
		expect(controlPortalOrigin({ protocol: 'https:', hostname: 'host.example.net', port: '8443' })).toBe('https://host.example.net:8444')
	})
})

describe('controlImpersonationUrl', () => {
	test('puts the session token in the fragment', () => {
		expect(controlImpersonationUrl('https://host.example.net:8444', 'tok/en')).toBe('https://host.example.net:8444/#session=tok%2Fen')
	})
})
