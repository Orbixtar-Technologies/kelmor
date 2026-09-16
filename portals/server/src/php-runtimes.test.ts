import { describe, expect, test } from 'vitest'
import { isSupportedPHPVersion, SUPPORTED_PHP_VERSIONS } from './php-runtimes'

describe('php runtimes', () => {
	test('accepts the release runtime set and rejects legacy versions', () => {
		expect(SUPPORTED_PHP_VERSIONS).toEqual(['8.3', '8.4', '8.5'])
		expect(isSupportedPHPVersion('8.3')).toBe(true)
		expect(isSupportedPHPVersion('8.1')).toBe(false)
		expect(isSupportedPHPVersion('7.4')).toBe(false)
	})
})
