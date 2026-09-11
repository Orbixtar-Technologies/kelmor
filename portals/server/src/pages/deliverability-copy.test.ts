import { describe, expect, test } from 'vitest'
import { analyzeDeliverabilityRecords, deliverabilitySummary } from './deliverability-copy'

const zone = 'shop.example.com'

describe('analyzeDeliverabilityRecords', () => {
	test('reports missing authentication records', () => {
		const checks = analyzeDeliverabilityRecords(zone, [
			{ name: zone, type: 'A', content: '10.0.0.8' },
		])
		expect(checks.map((check) => check.status)).toEqual(['missing', 'missing', 'warn'])
		expect(deliverabilitySummary(checks)).toContain('SPF missing')
	})

	test('accepts SPF, selector DKIM, and DMARC TXT records', () => {
		const checks = analyzeDeliverabilityRecords(zone, [
			{ name: zone, type: 'TXT', content: 'v=spf1 a mx -all' },
			{ name: `default._domainkey.${zone}`, type: 'TXT', content: 'v=DKIM1; k=rsa; p=abc' },
			{ name: `_dmarc.${zone}`, type: 'TXT', content: 'v=DMARC1; p=none' },
		])
		expect(checks.every((check) => check.status === 'pass')).toBe(true)
		expect(deliverabilitySummary(checks)).toBe('SPF, DKIM, and DMARC are present.')
	})
})
