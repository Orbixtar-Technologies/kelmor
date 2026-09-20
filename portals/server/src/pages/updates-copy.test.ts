import { describe, expect, test } from 'vitest'
import { formatLastChecked, updateCheckFeedback } from './updates-copy'

describe('update check copy', () => {
	test('labels a successful last-checked timestamp as local time', () => {
		expect(formatLastChecked()).toBe('—')
		expect(formatLastChecked('2026-09-20T13:00:00Z')).toMatch(/local time/)
		expect(formatLastChecked('2026-09-20T13:00:00Z')).not.toBe('—')
	})

	test('describes an idle check as up to date', () => {
		expect(updateCheckFeedback({
			state: 'idle',
			installed_release: '0.2.413',
		})).toBe("Checked — you're up to date")
	})

	test('describes a newer available release', () => {
		expect(updateCheckFeedback({
			state: 'available',
			installed_release: '0.2.413',
			available_release: '0.2.500',
		})).toBe('Checked — update available')
	})
})
