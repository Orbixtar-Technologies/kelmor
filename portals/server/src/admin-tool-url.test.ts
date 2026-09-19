import { describe, expect, test } from 'vitest'
import { adminToolUrl, isLiteralIPHost, isLoopbackHost, upgradePublicHttps } from './admin-tool-url'

describe('adminToolUrl', () => {
	test('upgrades HTTP vhosts that nginx TLS-termination would expose', () => {
		expect(adminToolUrl('webmail', 'http://webmail.shop.test/', 'shop.test'))
			.toBe('https://webmail.shop.test/')
		expect(adminToolUrl('phpmyadmin', 'http://phpmyadmin.shop.test/', 'shop.test'))
			.toBe('https://phpmyadmin.shop.test/')
	})

	test('synthesizes HTTPS tool hosts when admin-tools is empty', () => {
		expect(adminToolUrl('webmail', undefined, 'orbixtar.com'))
			.toBe('https://webmail.orbixtar.com/')
		expect(adminToolUrl('phpmyadmin', '', 'orbixtar.com'))
			.toBe('https://phpmyadmin.orbixtar.com/')
	})

	test('keeps a configured HTTPS URL', () => {
		expect(adminToolUrl('webmail', 'https://webmail.shop.test/', 'other.test'))
			.toBe('https://webmail.shop.test/')
	})
})

describe('upgradePublicHttps', () => {
	test('leaves loopback HTTP alone for local previews', () => {
		expect(upgradePublicHttps('http://127.0.0.1:18444/')).toBe('http://127.0.0.1:18444/')
	})
})

describe('host classifiers', () => {
	test('detects loopback and literal IPs', () => {
		expect(isLoopbackHost('127.0.0.1')).toBe(true)
		expect(isLiteralIPHost('150.239.113.59')).toBe(true)
		expect(isLiteralIPHost('kelmor.host')).toBe(false)
	})
})
