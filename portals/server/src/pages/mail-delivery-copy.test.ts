import { describe, expect, test } from 'vitest'
import {
	defaultReportDate,
	deliveryReportsApiPath,
	deliveryReportsHref,
	formatDeliveryStatus,
	parseReportDate,
	trackDeliveryApiPath,
	trackDeliveryHref,
} from './mail-delivery-copy'

describe('mail delivery dates and hrefs', () => {
	test('parses a classic WHM month/day/year form', () => {
		expect(parseReportDate({ year: '2026', month: '9', day: '19' })).toEqual({
			date: { year: 2026, month: 9, day: 19 },
		})
	})

	test('rejects a day that is not on the calendar', () => {
		expect(parseReportDate({ year: '2026', month: '2', day: '31' })).toEqual({
			error: 'That calendar date does not exist.',
		})
	})

	test('defaults the report date to today in UTC', () => {
		expect(defaultReportDate(new Date('2026-09-19T12:00:00Z'))).toEqual({
			year: 2026,
			month: 9,
			day: 19,
		})
	})

	test('builds dedicated report and track URLs instead of Jobs', () => {
		expect(deliveryReportsHref({ year: 2026, month: 9, day: 19 }, 'user@example.com'))
			.toBe('/mail/delivery-reports?year=2026&month=9&day=19&q=user%40example.com')
		expect(trackDeliveryHref('user@example.com')).toBe('/mail/track-delivery?q=user%40example.com')
		expect(deliveryReportsApiPath({ year: 2026, month: 9, day: 19 }))
			.toBe('/api/v1/mail/delivery-reports?year=2026&month=9&day=19')
		expect(trackDeliveryApiPath('shop@shop.test'))
			.toBe('/api/v1/mail/delivery-track?q=shop%40shop.test')
		expect(deliveryReportsHref({ year: 2026, month: 9, day: 19 })).not.toContain('/jobs')
		expect(trackDeliveryHref()).not.toContain('/jobs')
	})

	test('labels Postfix statuses for operators', () => {
		expect(formatDeliveryStatus('delivered')).toBe('Delivered')
		expect(formatDeliveryStatus('deferred')).toBe('Deferred')
	})
})
