import { describe, expect, test } from 'vitest'
import { isDisruptiveServiceAction, serviceActionImpact, updateInstallDisabledReason } from './service-control-copy'

describe('service and update copy', () => {
	test('treats stop and restart as disruptive', () => {
		expect(isDisruptiveServiceAction('stop')).toBe(true)
		expect(isDisruptiveServiceAction('reload')).toBe(false)
		expect(serviceActionImpact('stop')).toMatch(/unavailable/)
	})

	test('explains a disabled install when versions match', () => {
		expect(updateInstallDisabledReason('1.2.0', '1.2.0')).toMatch(/matches/)
		expect(updateInstallDisabledReason('1.2.0', '1.3.0')).toBe('')
	})
})
