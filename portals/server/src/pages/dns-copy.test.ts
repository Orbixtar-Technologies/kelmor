import { describe, expect, test } from 'vitest'
import { describeDnsChange, dnssecStateLabel, validateDnsRecord, zoneSyncLabel } from './dns-copy'

describe('dns copy', () => {
	test('labels zone sync separately from DNSSEC', () => {
		expect(zoneSyncLabel({ desired_revision: 3, observed_revision: 2 })).toBe('Zone sync pending')
		expect(zoneSyncLabel({ desired_revision: 2, observed_revision: 2 })).toBe('Zone in sync')
		expect(dnssecStateLabel(false)).toBe('DNSSEC disabled')
		expect(dnssecStateLabel(true)).toBe('DNSSEC enabled')
	})

	test('validates record content by type', () => {
		expect(validateDnsRecord('A', 'not-an-ip')).toMatch(/IPv4/)
		expect(validateDnsRecord('A', '203.0.113.10')).toBe('')
		expect(validateDnsRecord('MX', 'mail.example.com')).toMatch(/priority/)
	})

	test('previews add and replace actions', () => {
		expect(describeDnsChange('replace', { name: 'www', type: 'A', content: '203.0.113.10' })).toContain('Replace')
	})
})
