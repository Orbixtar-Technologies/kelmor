import { describe, expect, test } from 'vitest'
import {
	installedPHPVersions,
	isInstalledPHPVersion,
	isSupportedPHPVersion,
	missingHostPHPVersion,
	SUPPORTED_PHP_VERSIONS,
	type PHPRuntime,
} from './php-runtimes'

const hostInventory: PHPRuntime[] = [
	{ version: '8.3', status: 'installed' },
	{ version: '8.4', status: 'available' },
	{ version: '8.5', status: 'available' },
]

describe('php runtimes', () => {
	test('accepts the release runtime set and rejects legacy versions', () => {
		expect(SUPPORTED_PHP_VERSIONS).toEqual(['8.3', '8.4', '8.5'])
		expect(isSupportedPHPVersion('8.3')).toBe(true)
		expect(isSupportedPHPVersion('8.1')).toBe(false)
		expect(isSupportedPHPVersion('7.4')).toBe(false)
	})

	test('filters MultiPHP selectors to host-installed php-fpm only', () => {
		expect(installedPHPVersions(hostInventory)).toEqual(['8.3'])
		expect(isInstalledPHPVersion('8.3', ['8.3'])).toBe(true)
		expect(isInstalledPHPVersion('8.4', ['8.3'])).toBe(false)
		expect(isInstalledPHPVersion('8.1', ['8.3'])).toBe(false)
	})

	test('flags a recorded site version that the host no longer has', () => {
		expect(missingHostPHPVersion('8.4', ['8.3'])).toBe('8.4')
		expect(missingHostPHPVersion('8.3', ['8.3'])).toBe('')
		expect(missingHostPHPVersion('', ['8.3'])).toBe('')
	})
})
