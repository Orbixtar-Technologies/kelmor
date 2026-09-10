import { describe, expect, test } from 'vitest'
import { certExpiryLabel, certRenewalState } from './cert-copy'

const now = Date.parse('2026-09-10T00:00:00Z')

describe('cert copy', () => {
	test('shows days remaining and urgency', () => {
		expect(certExpiryLabel('2026-09-20T00:00:00Z', now)).toBe('10 days remaining · renew soon')
		expect(certExpiryLabel('2026-09-01T00:00:00Z', now)).toBe('Expired 9 days ago')
	})

	test('labels failed and due renewals', () => {
		expect(certRenewalState('failed')).toBe('Last request failed')
		expect(certRenewalState('active', '2026-09-20T00:00:00Z', now)).toBe('Renewal due')
	})
})
