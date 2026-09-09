import { describe, expect, test } from 'vitest'
import { hasCapabilities } from './rbac'

describe('hasCapabilities', () => {
	test('requires every listed capability', () => {
		expect(hasCapabilities({ 'accounts.create': true }, ['accounts.create', 'packages.read'])).toBe(false)
		expect(hasCapabilities({ 'accounts.create': true, 'packages.read': true }, ['accounts.create', 'packages.read'])).toBe(true)
	})
})
