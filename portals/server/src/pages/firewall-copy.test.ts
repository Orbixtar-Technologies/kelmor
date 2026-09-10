import { describe, expect, test } from 'vitest'
import { firewallImpactLines, firewallPolicyPreview, firewallRollbackCopy, rebootImpactLines } from './firewall-copy'

describe('firewall copy', () => {
	test('explains inbound drop, preserved ports, and rollback', () => {
		expect(firewallImpactLines().join(' ')).toMatch(/drop/i)
		expect(firewallPolicyPreview()).toContain('80, 443')
		expect(firewallRollbackCopy().toLocaleLowerCase()).toContain('re-appl')
	})

	test('lists reboot impact before confirmation', () => {
		expect(rebootImpactLines().some((line) => /disconnect/i.test(line))).toBe(true)
	})
})
