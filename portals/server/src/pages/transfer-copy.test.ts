import { describe, expect, test } from 'vitest'
import { summarizeExport, transferRollbackCopy } from './transfer-copy'

describe('transfer copy', () => {
	test('counts objects and reports export identity', () => {
		const summary = summarizeExport(JSON.stringify({
			username: 'shop',
			primary_domain: 'shop.example.com',
			format_version: '1',
			exported_at: '2026-09-10T00:00:00Z',
			domains: [{}, {}],
			mailboxes: [{}],
			databases: [],
		}))
		expect(summary?.username).toBe('shop')
		expect(summary?.counts.domains).toBe(2)
		expect(summary?.counts.mailboxes).toBe(1)
		expect(summary?.formatVersion).toBe('1')
	})

	test('explains rollback and cleanup', () => {
		expect(transferRollbackCopy().toLocaleLowerCase()).toContain('does not modify the source')
	})
})
