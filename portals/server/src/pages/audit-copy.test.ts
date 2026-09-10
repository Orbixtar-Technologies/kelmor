import { describe, expect, test } from 'vitest'
import { auditActorLabel, formatAuditAction, groupAuditEvents, maskSourceIp } from './audit-copy'
import type { AuditEvent } from '../types'

function event (overrides: Partial<AuditEvent> = {}): AuditEvent {
	return {
		id: 'evt-1',
		occurred_at: '2026-09-10T00:00:00Z',
		actor_type: 'user',
		action: 'server.reboot',
		request_id: 'req-1',
		success: true,
		...overrides,
	}
}

describe('audit copy', () => {
	test('uses a human operation label', () => {
		expect(formatAuditAction('server.firewall.apply')).toBe('Apply firewall')
		expect(formatAuditAction('server.update.check.intent')).toBe('Check for updates')
	})

	test('groups intent and outcome on the same request', () => {
		const grouped = groupAuditEvents([
			event({ id: '1', action: 'server.update.check.intent', occurred_at: '2026-09-10T00:00:00Z' }),
			event({ id: '2', action: 'server.update.check', occurred_at: '2026-09-10T00:00:01Z', success: false, metadata: { error: 'feed unreachable' } }),
		])
		expect(grouped).toHaveLength(1)
		expect(grouped[0].label).toBe('Check for updates')
		expect(grouped[0].reason).toBe('feed unreachable')
		expect(grouped[0].grouped).toBe(true)
	})

	test('masks source IP and avoids long actor UUIDs in the table', () => {
		expect(maskSourceIp('203.0.113.45')).toBe('203.0.*.*')
		expect(auditActorLabel(event({ actor_id: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee' }))).toBe('user')
	})
})
